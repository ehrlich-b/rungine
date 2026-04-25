//go:build integration

package tournament

import (
	"context"
	"strings"
	"testing"
	"time"

	"rungine/internal/chess"
)

// TestSchedulerStockfishMatch runs a 4-game match between two Stockfish
// instances at concurrency=2 using DefaultEngineFactory. It exercises
// the full path: spawn → ready → setoption → arbitrated game →
// graceful stop, run twice in parallel.
func TestSchedulerStockfishMatch(t *testing.T) {
	path := getStockfishPath(t)

	sch, err := NewScheduler(SchedulerConfig{
		Factory:     DefaultEngineFactory,
		Concurrency: 2,
		TimeControl: TimeControl{FixedDepth: 4},
		MaxPlies:    16,
		Event:       "Scheduler integration",
	})
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	whiteSpec := EngineSpec{Name: "Stockfish-W", BinaryPath: path}
	blackSpec := EngineSpec{Name: "Stockfish-B", BinaryPath: path}

	pairings := make([]Pairing, 4)
	for i := range pairings {
		// Alternate colors so the scheduler exercises both directions.
		w, b := whiteSpec, blackSpec
		if i%2 == 1 {
			w, b = blackSpec, whiteSpec
			w.Name, b.Name = "Stockfish-A", "Stockfish-B"
		}
		pairings[i] = Pairing{
			GameNumber: i + 1,
			Round:      "1",
			White:      w,
			Black:      b,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	outcomes := sch.Run(ctx, pairings)
	if len(outcomes) != len(pairings) {
		t.Fatalf("len(outcomes) = %d, want %d", len(outcomes), len(pairings))
	}

	for i, o := range outcomes {
		if o.Err != nil {
			t.Errorf("outcomes[%d].Err = %v", i, o.Err)
			continue
		}
		if o.Result == nil {
			t.Errorf("outcomes[%d].Result = nil", i)
			continue
		}
		if o.Result.Outcome == chess.Ongoing {
			t.Errorf("outcomes[%d].Outcome = ongoing, want terminal", i)
		}
		if o.PGN == "" || !strings.Contains(o.PGN, "[%eval") {
			t.Errorf("outcomes[%d].PGN missing [%%eval] tags", i)
		}
		t.Logf("game %d: %s (%s) plies=%d", o.Pairing.GameNumber, o.Result.Outcome, o.Result.Reason, o.Result.PlyCount)
	}
}
