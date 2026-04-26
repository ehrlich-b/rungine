package tournament

import (
	"math"
	"testing"

	"rungine/internal/chess"
)

func TestSPRTBoundsStandardConfig(t *testing.T) {
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	lower, upper := c.Bounds()
	// log(0.05/0.95) ≈ -2.944; log(0.95/0.05) ≈ +2.944.
	if math.Abs(lower-math.Log(0.05/0.95)) > 1e-9 {
		t.Errorf("lower = %v, want %v", lower, math.Log(0.05/0.95))
	}
	if math.Abs(upper-math.Log(0.95/0.05)) > 1e-9 {
		t.Errorf("upper = %v, want %v", upper, math.Log(0.95/0.05))
	}
	if lower > 0 || upper < 0 {
		t.Errorf("bounds [%v, %v] should bracket 0", lower, upper)
	}
}

func TestSPRTContinuesOnSparseData(t *testing.T) {
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	if got := c.Evaluate(0, 0, 0); got.Decision != SPRTContinue {
		t.Errorf("empty Decision = %v, want continue", got.Decision)
	}
	if got := c.Evaluate(2, 5, 1); got.Decision != SPRTContinue {
		t.Errorf("small sample Decision = %v, want continue", got.Decision)
	}
}

func TestSPRTAcceptsH1OnStrongEvidence(t *testing.T) {
	// Engine A is materially stronger than Elo1; expect H1 acceptance
	// long before reaching games-played cap.
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	res := c.Evaluate(200, 100, 50)
	if res.Decision != SPRTAcceptH1 {
		t.Errorf("decision = %v (LLR=%v), want acceptH1", res.Decision, res.LLR)
	}
	if res.LLR < res.UpperBound {
		t.Errorf("LLR=%v should be >= upper=%v", res.LLR, res.UpperBound)
	}
}

func TestSPRTAcceptsH0OnFlatEvidence(t *testing.T) {
	// 50% across many games is strong evidence the change isn't worth Elo1.
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	res := c.Evaluate(600, 400, 600)
	if res.Decision != SPRTAcceptH0 {
		t.Errorf("decision = %v (LLR=%v), want acceptH0", res.Decision, res.LLR)
	}
	if res.LLR > res.LowerBound {
		t.Errorf("LLR=%v should be <= lower=%v", res.LLR, res.LowerBound)
	}
}

func TestSPRTAcceptsH0OnRegression(t *testing.T) {
	// More losses than wins also rejects H1.
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	res := c.Evaluate(80, 200, 120)
	if res.Decision != SPRTAcceptH0 {
		t.Errorf("decision = %v (LLR=%v), want acceptH0", res.Decision, res.LLR)
	}
}

func TestSPRTLLRMonotonicWithWins(t *testing.T) {
	// Adding wins (without changing losses or draws) must raise LLR.
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	a := c.Evaluate(20, 20, 10)
	b := c.Evaluate(30, 20, 10)
	if b.LLR <= a.LLR {
		t.Errorf("more wins should raise LLR: a=%v, b=%v", a.LLR, b.LLR)
	}
}

func TestSPRTEvaluateOutcomesAggregates(t *testing.T) {
	outcomes := []GameOutcome{
		// Candidate "Cand" plays "Base"; alternates colors.
		outcome("Cand", "Base", chess.WhiteWins),
		outcome("Base", "Cand", chess.WhiteWins), // Cand loses
		outcome("Cand", "Base", chess.Drawn),
		outcome("Base", "Cand", chess.Drawn),
		// Foreign game shouldn't be counted.
		outcome("X", "Y", chess.WhiteWins),
	}
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}
	res := c.EvaluateOutcomes(outcomes, "Cand")
	// Cand: 1 win, 1 loss, 2 draws → LLR ~= 0 (no evidence either way).
	if math.Abs(res.LLR) > 0.5 {
		t.Errorf("LLR=%v should be near 0 for balanced outcomes", res.LLR)
	}
	if res.Decision != SPRTContinue {
		t.Errorf("decision = %v, want continue (insufficient evidence)", res.Decision)
	}
}

func TestSPRTStopperWithProgressFiresEveryGame(t *testing.T) {
	// Progress callback should fire after every relevant game (continue
	// and decided), with monotonically increasing LLR for a winning
	// candidate. Decision callback fires exactly once at termination.
	c := SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05}

	var (
		updates   []SPRTResult
		decisions []SPRTResult
	)
	stop := NewSPRTStopperWithProgress(c, "Cand",
		func(r SPRTResult) { updates = append(updates, r) },
		func(r SPRTResult) { decisions = append(decisions, r) },
	)

	// Cand wins every game; alternate colors so both branches exercise.
	for i := range 200 {
		var o GameOutcome
		if i%2 == 0 {
			o = outcome("Cand", "Base", chess.WhiteWins)
		} else {
			o = outcome("Base", "Cand", chess.BlackWins)
		}
		if stop(o) {
			break
		}
	}

	if len(updates) == 0 {
		t.Fatal("onProgress never fired")
	}
	if len(decisions) != 1 {
		t.Errorf("onDecision fired %d times, want 1", len(decisions))
	}
	for i := 1; i < len(updates); i++ {
		if updates[i].LLR < updates[i-1].LLR {
			t.Errorf("LLR not monotonically rising for winning candidate: updates[%d]=%v < updates[%d]=%v",
				i, updates[i].LLR, i-1, updates[i-1].LLR)
			break
		}
	}
	if updates[len(updates)-1].Decision != SPRTAcceptH1 {
		t.Errorf("final progress decision = %v, want acceptH1", updates[len(updates)-1].Decision)
	}
}

func TestSPRTDecisionString(t *testing.T) {
	cases := map[SPRTDecision]string{
		SPRTContinue:  "continue",
		SPRTAcceptH0:  "accept H0",
		SPRTAcceptH1:  "accept H1",
		SPRTDecision(99): "SPRTDecision(99)",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", d, got, want)
		}
	}
}
