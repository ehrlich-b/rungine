//go:build integration

package tournament

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"rungine/internal/chess"
	"rungine/internal/uci"
)

func getStockfishPath(t *testing.T) string {
	t.Helper()
	if path := os.Getenv("STOCKFISH_PATH"); path != "" {
		return path
	}
	candidates := []string{
		"/usr/bin/stockfish",
		"/usr/local/bin/stockfish",
		"/opt/homebrew/bin/stockfish",
		"/usr/games/stockfish",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	t.Skip("STOCKFISH_PATH not set and stockfish not found in common locations")
	return ""
}

func startEngine(t *testing.T, id, path string) *uci.Engine {
	t.Helper()
	engine := uci.NewEngine(id, path)
	// Engine.Start stores this context for the engine's whole lifetime;
	// don't bound it by the helper scope or the subprocess gets killed
	// when this function returns.
	if err := engine.Start(context.Background()); err != nil {
		t.Fatalf("engine %s Start: %v", id, err)
	}
	t.Cleanup(func() { _ = engine.Stop() })
	return engine
}

// Stockfish vs Stockfish at fixed shallow depth, capped by MaxPlies so the
// test finishes quickly. Verifies the arbiter can drive a real UCI engine
// through a complete game.
func TestArbiterStockfishVsStockfish(t *testing.T) {
	path := getStockfishPath(t)

	white := startEngine(t, "sf-w", path)
	black := startEngine(t, "sf-b", path)

	arb, err := New(Config{
		White: white, Black: black,
		WhiteName: "Stockfish-W", BlackName: "Stockfish-B",
		TimeControl: TimeControl{FixedDepth: 4},
		MaxPlies:    20,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := arb.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.PlyCount == 0 {
		t.Fatal("game produced no moves")
	}
	t.Logf("game ended after %d plies: %s (%s)", res.PlyCount, res.Outcome, res.Reason)
	pgn := arb.AnnotatedPGN(res)
	t.Logf("Annotated PGN:\n%s", pgn)

	// At depth 4 over 20 plies, the game won't finish naturally — expect
	// the MaxPlies adjudication.
	if res.Outcome == chess.Ongoing {
		t.Errorf("Outcome = ongoing, want terminal")
	}

	// Every ply should have engine analysis info attached.
	if len(res.Moves) != res.PlyCount {
		t.Fatalf("len(res.Moves) = %d, want %d", len(res.Moves), res.PlyCount)
	}
	for i, m := range res.Moves {
		if !m.HasInfo {
			t.Errorf("Moves[%d] (%s) has no info", i, m.UCI)
		}
		if m.Info.Depth == 0 {
			t.Errorf("Moves[%d] (%s) depth = 0, want > 0", i, m.UCI)
		}
	}

	// PGN should embed [%eval ...] annotations from the captured info.
	if !strings.Contains(pgn, "[%eval") {
		t.Errorf("annotated PGN missing [%%eval] tags:\n%s", pgn)
	}
}
