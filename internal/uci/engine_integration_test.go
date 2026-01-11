//go:build integration

package uci

import (
	"context"
	"os"
	"testing"
	"time"
)

func getStockfishPath(t *testing.T) string {
	path := os.Getenv("STOCKFISH_PATH")
	if path == "" {
		// Try common locations
		candidates := []string{
			"/usr/bin/stockfish",
			"/usr/local/bin/stockfish",
			"/usr/games/stockfish",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
		t.Skip("STOCKFISH_PATH not set and stockfish not found in common locations")
	}
	return path
}

func TestEngineLifecycle(t *testing.T) {
	sfPath := getStockfishPath(t)

	engine := NewEngine("test-sf", sfPath)

	// Start engine
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer engine.Stop()

	// Should be ready after Start returns
	if engine.State() != EngineStateReady {
		t.Errorf("State() = %v, want Ready", engine.State())
	}

	// Should have received engine name
	if engine.Name == "" {
		t.Error("Name should be set after UCI init")
	}
	t.Logf("Engine name: %s", engine.Name)

	// Should have options
	opts := engine.Options()
	if len(opts) == 0 {
		t.Error("Should have received UCI options")
	}
	t.Logf("Received %d UCI options", len(opts))

	// Check for expected Stockfish options
	if _, ok := opts["Hash"]; !ok {
		t.Error("Should have Hash option")
	}
	if _, ok := opts["Threads"]; !ok {
		t.Error("Should have Threads option")
	}

	// IsReady should work
	err = engine.IsReady(5 * time.Second)
	if err != nil {
		t.Errorf("IsReady() error: %v", err)
	}

	// Stop should work
	err = engine.Stop()
	if err != nil {
		t.Errorf("Stop() error: %v", err)
	}

	if engine.State() != EngineStateStopped {
		t.Errorf("State() = %v, want Stopped", engine.State())
	}
}

func TestEngineAnalysis(t *testing.T) {
	sfPath := getStockfishPath(t)

	engine := NewEngine("test-sf", sfPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer engine.Stop()

	// Set starting position
	err = engine.SetPosition("", nil)
	if err != nil {
		t.Fatalf("SetPosition() error: %v", err)
	}

	// Start analysis to depth 10
	err = engine.Go(GoParams{Depth: 10})
	if err != nil {
		t.Fatalf("Go() error: %v", err)
	}

	// Should be thinking now
	if engine.State() != EngineStateThinking {
		t.Errorf("State() = %v, want Thinking", engine.State())
	}

	// Collect analysis info
	infoCh := engine.InfoChannel()
	var lastInfo AnalysisInfo
	timeout := time.After(15 * time.Second)

	for {
		select {
		case info, ok := <-infoCh:
			if !ok {
				t.Fatal("Info channel closed unexpectedly")
			}
			lastInfo = info
			t.Logf("depth=%d score=%s nodes=%d pv=%v",
				info.Depth, info.Score.String(), info.Nodes, info.PV)

			// Check if we reached target depth
			if info.Depth >= 10 && len(info.PV) > 0 {
				goto done
			}
		case <-timeout:
			t.Fatal("Timed out waiting for depth 10")
		}
	}

done:
	// Should have received meaningful analysis
	if lastInfo.Depth < 10 {
		t.Errorf("Depth = %d, want >= 10", lastInfo.Depth)
	}
	if len(lastInfo.PV) == 0 {
		t.Error("PV should not be empty")
	}
	if lastInfo.Nodes == 0 {
		t.Error("Nodes should not be zero")
	}

	// Wait for bestmove (engine should transition back to ready)
	time.Sleep(500 * time.Millisecond)

	// Engine should be back to ready after bestmove
	if engine.State() != EngineStateReady {
		t.Logf("State after analysis: %v (may still be processing)", engine.State())
	}
}

func TestEngineStopSearch(t *testing.T) {
	sfPath := getStockfishPath(t)

	engine := NewEngine("test-sf", sfPath)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer engine.Stop()

	// Set position
	err = engine.SetPosition("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1", nil)
	if err != nil {
		t.Fatalf("SetPosition() error: %v", err)
	}

	// Start infinite analysis
	err = engine.Go(GoParams{Infinite: true})
	if err != nil {
		t.Fatalf("Go() error: %v", err)
	}

	// Let it run briefly
	time.Sleep(500 * time.Millisecond)

	if engine.State() != EngineStateThinking {
		t.Errorf("State() = %v, want Thinking", engine.State())
	}

	// Stop the search
	err = engine.StopSearch()
	if err != nil {
		t.Errorf("StopSearch() error: %v", err)
	}

	// Wait for bestmove
	time.Sleep(500 * time.Millisecond)

	// Should be ready again
	if engine.State() != EngineStateReady {
		t.Errorf("State() = %v, want Ready after stop", engine.State())
	}
}

func TestEngineSetOption(t *testing.T) {
	sfPath := getStockfishPath(t)

	engine := NewEngine("test-sf", sfPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer engine.Stop()

	// Set Hash option
	err = engine.SetOption("Hash", "32")
	if err != nil {
		t.Errorf("SetOption(Hash) error: %v", err)
	}

	// Set Threads option
	err = engine.SetOption("Threads", "2")
	if err != nil {
		t.Errorf("SetOption(Threads) error: %v", err)
	}

	// Verify isready still works after setting options
	err = engine.IsReady(5 * time.Second)
	if err != nil {
		t.Errorf("IsReady() after SetOption error: %v", err)
	}
}

func TestEnginePositionWithMoves(t *testing.T) {
	sfPath := getStockfishPath(t)

	engine := NewEngine("test-sf", sfPath)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer engine.Stop()

	// Set position with moves from startpos
	moves := []string{"e2e4", "e7e5", "g1f3", "b8c6"}
	err = engine.SetPosition("", moves)
	if err != nil {
		t.Fatalf("SetPosition() error: %v", err)
	}

	// Analyze briefly
	err = engine.Go(GoParams{Depth: 8})
	if err != nil {
		t.Fatalf("Go() error: %v", err)
	}

	// Collect some analysis
	infoCh := engine.InfoChannel()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case info, ok := <-infoCh:
			if !ok {
				return
			}
			if info.Depth >= 8 && len(info.PV) > 0 {
				t.Logf("Analysis at depth %d: %v", info.Depth, info.PV)
				return
			}
		case <-timeout:
			t.Fatal("Timed out")
		}
	}
}

func TestManagerMultipleEngines(t *testing.T) {
	sfPath := getStockfishPath(t)

	mgr := NewEngineManager()
	defer mgr.Shutdown()

	// Register two engine instances
	err := mgr.RegisterEngine("sf1", sfPath)
	if err != nil {
		t.Fatalf("RegisterEngine(sf1) error: %v", err)
	}

	err = mgr.RegisterEngine("sf2", sfPath)
	if err != nil {
		t.Fatalf("RegisterEngine(sf2) error: %v", err)
	}

	// List should show both
	engines := mgr.ListEngines()
	if len(engines) != 2 {
		t.Errorf("ListEngines() = %d engines, want 2", len(engines))
	}

	// Start both
	err = mgr.StartEngine("sf1")
	if err != nil {
		t.Fatalf("StartEngine(sf1) error: %v", err)
	}

	err = mgr.StartEngine("sf2")
	if err != nil {
		t.Fatalf("StartEngine(sf2) error: %v", err)
	}

	// Both should be ready
	engines = mgr.ListEngines()
	for _, e := range engines {
		if e.State != "ready" {
			t.Errorf("Engine %s state = %s, want ready", e.ID, e.State)
		}
	}

	// Start analysis on both
	err = mgr.StartAnalysis("", nil, []string{"sf1", "sf2"}, GoParams{Depth: 5})
	if err != nil {
		t.Fatalf("StartAnalysis() error: %v", err)
	}

	// Let them analyze briefly
	time.Sleep(2 * time.Second)

	// Stop analysis
	err = mgr.StopAnalysis([]string{"sf1", "sf2"})
	if err != nil {
		t.Errorf("StopAnalysis() error: %v", err)
	}

	// Shutdown should stop all
	mgr.Shutdown()
}
