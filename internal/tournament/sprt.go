package tournament

import (
	"fmt"
	"math"
	"sync"

	"rungine/internal/chess"
)

// SPRTConfig parameterises the Sequential Probability Ratio Test as
// used in engine development. Elo0 is the null hypothesis ("no
// improvement"); Elo1 is the alternative ("at least this much better").
// Alpha is the type-I error rate (false positive on Elo1); Beta is the
// type-II error rate (false negative on Elo0).
type SPRTConfig struct {
	Elo0  float64
	Elo1  float64
	Alpha float64
	Beta  float64
}

// SPRTDecision is the test outcome at a given sample size.
type SPRTDecision int

const (
	// SPRTContinue means more games are needed.
	SPRTContinue SPRTDecision = iota
	// SPRTAcceptH0 rejects the alternative ("the change is no better").
	SPRTAcceptH0
	// SPRTAcceptH1 accepts the alternative ("the change is at least Elo1 better").
	SPRTAcceptH1
)

func (d SPRTDecision) String() string {
	switch d {
	case SPRTContinue:
		return "continue"
	case SPRTAcceptH0:
		return "accept H0"
	case SPRTAcceptH1:
		return "accept H1"
	default:
		return fmt.Sprintf("SPRTDecision(%d)", int(d))
	}
}

// SPRTResult captures the test state after observing W/D/L.
type SPRTResult struct {
	LLR        float64
	LowerBound float64
	UpperBound float64
	Decision   SPRTDecision
}

// Bounds returns the LLR bounds derived from Alpha/Beta.
// LLR <= lower → accept H0; LLR >= upper → accept H1.
func (c SPRTConfig) Bounds() (lower, upper float64) {
	lower = math.Log(c.Beta / (1 - c.Alpha))
	upper = math.Log((1 - c.Beta) / c.Alpha)
	return lower, upper
}

// Evaluate runs the SPRT test against observed W/D/L counts. Uses the
// trinomial-with-fixed-draw-rate formulation: the draw probability is
// estimated from observed games and held constant across the two
// hypotheses, so only wins and losses contribute to LLR.
func (c SPRTConfig) Evaluate(wins, draws, losses int) SPRTResult {
	res := SPRTResult{}
	res.LowerBound, res.UpperBound = c.Bounds()

	n := wins + draws + losses
	if n == 0 {
		res.Decision = SPRTContinue
		return res
	}

	drawRate := float64(draws) / float64(n)
	score0 := 1 / (1 + math.Pow(10, -c.Elo0/400))
	score1 := 1 / (1 + math.Pow(10, -c.Elo1/400))

	pw0 := score0 - 0.5*drawRate
	pl0 := 1 - drawRate - pw0
	pw1 := score1 - 0.5*drawRate
	pl1 := 1 - drawRate - pw1

	// All four probabilities must be strictly positive for the LLR to
	// be defined; this can fail when the draw rate is so high that one
	// of the score-implied win/loss probabilities goes non-positive
	// (typical with very small samples).
	if pw0 <= 0 || pl0 <= 0 || pw1 <= 0 || pl1 <= 0 {
		res.Decision = SPRTContinue
		return res
	}

	res.LLR = float64(wins)*math.Log(pw1/pw0) + float64(losses)*math.Log(pl1/pl0)

	switch {
	case res.LLR >= res.UpperBound:
		res.Decision = SPRTAcceptH1
	case res.LLR <= res.LowerBound:
		res.Decision = SPRTAcceptH0
	default:
		res.Decision = SPRTContinue
	}
	return res
}

// NewSPRTStopper returns a Scheduler.ShouldStop function that runs SPRT
// against the running W/D/L tally for candidateName as games complete.
// Once the SPRT decision is anything other than Continue, it returns
// true and further pairings are skipped. The optional onDecision
// callback is invoked once with the terminating result (also from a
// worker goroutine).
func NewSPRTStopper(c SPRTConfig, candidateName string, onDecision func(SPRTResult)) func(GameOutcome) bool {
	var (
		mu       sync.Mutex
		w, d, l  int
		decided  bool
	)
	return func(o GameOutcome) bool {
		if o.Err != nil || o.Result == nil {
			return false
		}
		var candidateIsWhite bool
		switch candidateName {
		case o.Pairing.White.Name:
			candidateIsWhite = true
		case o.Pairing.Black.Name:
			candidateIsWhite = false
		default:
			return false
		}

		mu.Lock()
		defer mu.Unlock()
		if decided {
			return true
		}
		switch o.Result.Outcome {
		case chess.WhiteWins:
			if candidateIsWhite {
				w++
			} else {
				l++
			}
		case chess.BlackWins:
			if candidateIsWhite {
				l++
			} else {
				w++
			}
		case chess.Drawn:
			d++
		default:
			return false
		}
		res := c.Evaluate(w, d, l)
		if res.Decision != SPRTContinue {
			decided = true
			if onDecision != nil {
				onDecision(res)
			}
			return true
		}
		return false
	}
}

// EvaluateOutcomes is a convenience wrapper that aggregates a slice of
// GameOutcomes into W/D/L from the perspective of candidateName and
// runs the SPRT test. Outcomes that didn't involve candidateName, that
// errored, or that have a non-terminal result are skipped.
func (c SPRTConfig) EvaluateOutcomes(outcomes []GameOutcome, candidateName string) SPRTResult {
	var w, d, l int
	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		var candidateIsWhite bool
		switch candidateName {
		case o.Pairing.White.Name:
			candidateIsWhite = true
		case o.Pairing.Black.Name:
			candidateIsWhite = false
		default:
			continue
		}
		switch o.Result.Outcome {
		case chess.WhiteWins:
			if candidateIsWhite {
				w++
			} else {
				l++
			}
		case chess.BlackWins:
			if candidateIsWhite {
				l++
			} else {
				w++
			}
		case chess.Drawn:
			d++
		}
	}
	return c.Evaluate(w, d, l)
}
