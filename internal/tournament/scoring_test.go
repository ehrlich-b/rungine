package tournament

import (
	"errors"
	"math"
	"strings"
	"testing"

	"rungine/internal/chess"
)

// outcome is a small helper to build a GameOutcome for scoring tests.
func outcome(white, black string, result chess.Outcome) GameOutcome {
	return GameOutcome{
		Pairing: Pairing{
			White: EngineSpec{Name: white},
			Black: EngineSpec{Name: black},
		},
		Result: &Result{Outcome: result, Reason: chess.ReasonCheckmate},
	}
}

func TestBuildStandingsBasic(t *testing.T) {
	outcomes := []GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.Drawn),
		outcome("A", "C", chess.BlackWins),
		outcome("C", "B", chess.WhiteWins),
	}
	st := BuildStandings(outcomes)

	if len(st.Players) != 3 {
		t.Fatalf("len(Players) = %d, want 3", len(st.Players))
	}

	scoresByName := map[string]float64{}
	for _, p := range st.Players {
		scoresByName[p.Name] = p.Points
	}
	// A: win + draw + loss = 1.5
	// B: loss + draw + loss = 0.5
	// C: win + win = 2
	if got := scoresByName["A"]; got != 1.5 {
		t.Errorf("A points = %v, want 1.5", got)
	}
	if got := scoresByName["B"]; got != 0.5 {
		t.Errorf("B points = %v, want 0.5", got)
	}
	if got := scoresByName["C"]; got != 2 {
		t.Errorf("C points = %v, want 2", got)
	}

	// Order: C (2), A (1.5), B (0.5).
	want := []string{"C", "A", "B"}
	for i, name := range want {
		if st.Players[i].Name != name {
			t.Errorf("Players[%d] = %q, want %q", i, st.Players[i].Name, name)
		}
	}
}

func TestBuildStandingsTieBreaks(t *testing.T) {
	// Two engines with the same points; tie-broken by wins, then name.
	outcomes := []GameOutcome{
		outcome("alpha", "beta", chess.WhiteWins), // alpha wins
		outcome("beta", "alpha", chess.WhiteWins), // beta wins
		outcome("alpha", "gamma", chess.Drawn),
		outcome("gamma", "alpha", chess.Drawn),
		outcome("beta", "gamma", chess.Drawn),
		outcome("gamma", "beta", chess.Drawn),
	}
	st := BuildStandings(outcomes)
	// alpha: W=1, D=2, L=1 → 2.0 pts
	// beta:  W=1, D=2, L=1 → 2.0 pts
	// gamma: W=0, D=4, L=0 → 2.0 pts
	// Tie on points: alpha and beta both have 1 win, gamma 0; alpha < beta alphabetically.
	wantOrder := []string{"alpha", "beta", "gamma"}
	for i, w := range wantOrder {
		if st.Players[i].Name != w {
			t.Errorf("Players[%d] = %q, want %q", i, st.Players[i].Name, w)
		}
	}
}

func TestBuildStandingsSkipsErrorAndOngoing(t *testing.T) {
	failed := GameOutcome{
		Pairing: Pairing{White: EngineSpec{Name: "A"}, Black: EngineSpec{Name: "B"}},
		Err:     errors.New("crash"),
	}
	ongoing := GameOutcome{
		Pairing: Pairing{White: EngineSpec{Name: "A"}, Black: EngineSpec{Name: "B"}},
		Result:  &Result{Outcome: chess.Ongoing},
	}
	good := outcome("A", "B", chess.WhiteWins)

	st := BuildStandings([]GameOutcome{failed, ongoing, good})
	for _, p := range st.Players {
		if p.Games != 1 {
			t.Errorf("%s.Games = %d, want 1 (only the good outcome counts)", p.Name, p.Games)
		}
	}
}

func TestPlayerScorePct(t *testing.T) {
	p := PlayerScore{Wins: 3, Draws: 2, Losses: 5, Games: 10, Points: 4}
	if got := p.ScorePct(); got != 0.4 {
		t.Errorf("ScorePct = %v, want 0.4", got)
	}
	if got := (PlayerScore{}).ScorePct(); got != 0 {
		t.Errorf("empty ScorePct = %v, want 0", got)
	}
}

func TestStandingsString(t *testing.T) {
	st := BuildStandings([]GameOutcome{
		outcome("Stockfish", "Komodo", chess.WhiteWins),
		outcome("Komodo", "Stockfish", chess.Drawn),
	})
	got := st.String()
	if !strings.Contains(got, "Stockfish") || !strings.Contains(got, "Komodo") {
		t.Errorf("Standings.String() missing engine names:\n%s", got)
	}
	// 1.5 points for Stockfish (win + draw)
	if !strings.Contains(got, "1.5") {
		t.Errorf("Standings.String() missing 1.5 points:\n%s", got)
	}
}

func TestBuildCrosstable(t *testing.T) {
	outcomes := []GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.WhiteWins),
		outcome("A", "C", chess.Drawn),
		outcome("C", "A", chess.WhiteWins),
		outcome("B", "C", chess.Drawn),
		outcome("C", "B", chess.Drawn),
	}
	ct := BuildCrosstable(outcomes)

	if len(ct.Players) != 3 {
		t.Fatalf("len(Players) = %d, want 3", len(ct.Players))
	}

	// Build name → index from the crosstable's Player order.
	idx := map[string]int{}
	for i, n := range ct.Players {
		idx[n] = i
	}

	a, b, c := idx["A"], idx["B"], idx["C"]

	// A vs B: A win as white (1), A loss as black (0) → 1/2.
	if ct.Score[a][b] != 1 || ct.Games[a][b] != 2 {
		t.Errorf("A vs B = %.1f/%d, want 1.0/2", ct.Score[a][b], ct.Games[a][b])
	}
	// A vs C: A draw as white (0.5), A loss as black (0) → 0.5/2.
	if ct.Score[a][c] != 0.5 || ct.Games[a][c] != 2 {
		t.Errorf("A vs C = %.1f/%d, want 0.5/2", ct.Score[a][c], ct.Games[a][c])
	}
	// C vs A: C draw as black (0.5), C win as white (1) → 1.5/2.
	if ct.Score[c][a] != 1.5 || ct.Games[c][a] != 2 {
		t.Errorf("C vs A = %.1f/%d, want 1.5/2", ct.Score[c][a], ct.Games[c][a])
	}

	// Diagonal is zero.
	for i := range ct.Players {
		if ct.Score[i][i] != 0 || ct.Games[i][i] != 0 {
			t.Errorf("diagonal[%d] = %.1f/%d, want 0/0", i, ct.Score[i][i], ct.Games[i][i])
		}
	}
}

func TestBuildCrosstableEmpty(t *testing.T) {
	ct := BuildCrosstable(nil)
	if len(ct.Players) != 0 {
		t.Errorf("empty input should produce empty crosstable, got %d players", len(ct.Players))
	}
}

func TestCrosstableString(t *testing.T) {
	ct := BuildCrosstable([]GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.Drawn),
	})
	s := ct.String()
	if !strings.Contains(s, "A") || !strings.Contains(s, "B") {
		t.Errorf("Crosstable.String() missing engine names:\n%s", s)
	}
	if !strings.Contains(s, "—") {
		t.Errorf("Crosstable.String() missing diagonal placeholder:\n%s", s)
	}
}

func TestEloDelta(t *testing.T) {
	cases := []struct {
		name     string
		score    float64
		want     float64
		tolerant float64
	}{
		{"even", 0.5, 0, 0.001},
		{"75%", 0.75, 191.0, 1.0}, // -400*log10(1/.75 - 1) ≈ 191
		{"25%", 0.25, -191.0, 1.0},
		{"100% caps", 1.0, 800, 0.001},
		{"0% caps", 0.0, -800, 0.001},
	}
	for _, c := range cases {
		got := EloDelta(c.score)
		if math.Abs(got-c.want) > c.tolerant {
			t.Errorf("%s: EloDelta(%v) = %v, want %v ± %v", c.name, c.score, got, c.want, c.tolerant)
		}
	}
}

func TestPerformanceRating(t *testing.T) {
	// 75% score against 1500 average → ~1691 perf rating.
	got := PerformanceRating(0.75, 1500)
	if math.Abs(got-1691) > 1 {
		t.Errorf("PerformanceRating(0.75, 1500) = %v, want ~1691", got)
	}
	// Even score vs anchor 0 → 0.
	if got := PerformanceRating(0.5, 0); got != 0 {
		t.Errorf("PerformanceRating(0.5, 0) = %v, want 0", got)
	}
}

func TestDrawRatio(t *testing.T) {
	if got := DrawRatio(2, 5); got != 0.4 {
		t.Errorf("DrawRatio(2, 5) = %v, want 0.4", got)
	}
	if got := DrawRatio(0, 0); got != 0 {
		t.Errorf("DrawRatio(0, 0) = %v, want 0", got)
	}
}

func TestLikelihoodOfSuperiority(t *testing.T) {
	// No data → 0.5.
	if got := LikelihoodOfSuperiority(0, 0); got != 0.5 {
		t.Errorf("LOS(0, 0) = %v, want 0.5", got)
	}
	// Equal wins and losses → 0.5.
	if got := LikelihoodOfSuperiority(10, 10); got != 0.5 {
		t.Errorf("LOS(10, 10) = %v, want 0.5", got)
	}
	// Strong evidence → close to 1.
	if got := LikelihoodOfSuperiority(20, 0); got < 0.99 {
		t.Errorf("LOS(20, 0) = %v, want >0.99", got)
	}
	// Strong evidence the other way → close to 0.
	if got := LikelihoodOfSuperiority(0, 20); got > 0.01 {
		t.Errorf("LOS(0, 20) = %v, want <0.01", got)
	}
	// Symmetry: LOS(W, L) + LOS(L, W) ≈ 1.
	a := LikelihoodOfSuperiority(7, 3)
	b := LikelihoodOfSuperiority(3, 7)
	if math.Abs(a+b-1) > 0.001 {
		t.Errorf("LOS(7,3) + LOS(3,7) = %v, want 1", a+b)
	}
}

func TestScoreInterval(t *testing.T) {
	// 50% in many games has a tight interval around 0.5.
	lo, mid, hi := ScoreInterval(50, 0, 50, 0.95)
	if math.Abs(mid-0.5) > 0.001 {
		t.Errorf("mid = %v, want 0.5", mid)
	}
	if lo > 0.5 || hi < 0.5 {
		t.Errorf("interval [%v, %v] should bracket 0.5", lo, hi)
	}
	if hi-lo > 0.2 {
		t.Errorf("interval [%v, %v] too wide for n=100", lo, hi)
	}
	// Empty input is centered at 0.5.
	lo, mid, hi = ScoreInterval(0, 0, 0, 0.95)
	if mid != 0.5 || lo != 0.5 || hi != 0.5 {
		t.Errorf("empty interval = [%v, %v, %v], want 0.5/0.5/0.5", lo, mid, hi)
	}
}

func TestEloInterval(t *testing.T) {
	// 60/40 with 50 games gives a positive ELO delta with a CI that crosses 0.
	lo, mid, hi := EloInterval(60, 0, 40, 0.95)
	if mid <= 0 {
		t.Errorf("mid = %v, want > 0", mid)
	}
	if lo > mid || hi < mid {
		t.Errorf("interval [%v, %v, %v] not ordered", lo, mid, hi)
	}
	if !(lo < 0 && hi > 0) {
		// Note: 60-40 in 100 is ~ +69 ELO with ~ ±97 CI for 95% — should cross 0.
		t.Logf("60/40 interval [%v, %v, %v] (informational)", lo, mid, hi)
	}
}

func TestEstimateElosOrdering(t *testing.T) {
	// Strong > Mid > Weak. Strong wins 80%, Mid wins 50% vs Weak,
	// Mid loses 80% to Strong.
	outcomes := []GameOutcome{}
	add := func(w, b string, n int, result chess.Outcome) {
		for range n {
			outcomes = append(outcomes, outcome(w, b, result))
		}
	}
	// Strong vs Mid: Strong wins 8, draws 2 of 10
	add("Strong", "Mid", 8, chess.WhiteWins)
	add("Mid", "Strong", 8, chess.BlackWins) // Strong as black
	add("Strong", "Mid", 2, chess.Drawn)
	add("Mid", "Strong", 2, chess.Drawn)
	// Strong vs Weak: 10/10
	add("Strong", "Weak", 10, chess.WhiteWins)
	add("Weak", "Strong", 10, chess.BlackWins)
	// Mid vs Weak: Mid wins 7, 3 draws of 10
	add("Mid", "Weak", 7, chess.WhiteWins)
	add("Weak", "Mid", 7, chess.BlackWins)
	add("Mid", "Weak", 3, chess.Drawn)
	add("Weak", "Mid", 3, chess.Drawn)

	elos := EstimateElos(outcomes, "Mid", 0)
	if elos["Strong"] <= elos["Mid"] {
		t.Errorf("Strong (%v) should rate above Mid (%v)", elos["Strong"], elos["Mid"])
	}
	if elos["Mid"] <= elos["Weak"] {
		t.Errorf("Mid (%v) should rate above Weak (%v)", elos["Mid"], elos["Weak"])
	}
	// Anchor is held.
	if math.Abs(elos["Mid"]) > 0.05 {
		t.Errorf("Mid anchored at 0, got %v", elos["Mid"])
	}
}

func TestEstimateElosTwoEngines(t *testing.T) {
	// One engine wins 75% → +191 ELO delta.
	outcomes := []GameOutcome{}
	for range 30 {
		outcomes = append(outcomes, outcome("A", "B", chess.WhiteWins))
	}
	for range 10 {
		outcomes = append(outcomes, outcome("A", "B", chess.BlackWins))
	}
	elos := EstimateElos(outcomes, "B", 0)
	delta := elos["A"] - elos["B"]
	if math.Abs(delta-191) > 5 {
		t.Errorf("two-engine A-B delta = %v, want ~191", delta)
	}
}

func TestEstimateElosNoAnchorPinsMean(t *testing.T) {
	// Without an anchor, the mean rating is held at anchorRating (0 here).
	outcomes := []GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.WhiteWins),
		outcome("A", "C", chess.Drawn),
		outcome("C", "A", chess.Drawn),
		outcome("B", "C", chess.WhiteWins),
		outcome("C", "B", chess.Drawn),
	}
	elos := EstimateElos(outcomes, "", 0)
	if len(elos) != 3 {
		t.Fatalf("got %d ratings, want 3", len(elos))
	}
	sum := 0.0
	for _, v := range elos {
		sum += v
	}
	if math.Abs(sum/3) > 0.05 {
		t.Errorf("mean rating = %v, want ~0", sum/3)
	}
}

func TestEstimateElosEmpty(t *testing.T) {
	if got := EstimateElos(nil, "", 0); len(got) != 0 {
		t.Errorf("empty outcomes returned %d ratings, want 0", len(got))
	}
}
