package tournament

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rungine/internal/chess"
)

// scriptedTable maps EngineSpec.Name to a function that builds the
// scripted engine for that spec. Tests register entries and pass
// scriptedTable.factory as the scheduler's EngineFactory.
type scriptedTable map[string]func() *scripted

func (st scriptedTable) factory(_ context.Context, spec EngineSpec) (*RunningEngine, error) {
	make, ok := st[spec.Name]
	if !ok {
		return nil, errors.New("no test engine for " + spec.Name)
	}
	eng := make()
	return &RunningEngine{Engine: eng, Stop: func() error { return nil }}, nil
}

func TestSchedulerRequiresFactory(t *testing.T) {
	if _, err := NewScheduler(SchedulerConfig{}); err == nil {
		t.Error("NewScheduler with nil Factory should fail")
	}
}

func TestSchedulerSinglePairing(t *testing.T) {
	// Fool's mate.
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"f2f3", "g2g4"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5", "d8h4"}) },
	}

	sch, err := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		TimeControl: TimeControl{FixedDepth: 1},
	})
	if err != nil {
		t.Fatalf("NewScheduler: %v", err)
	}

	pairings := []Pairing{{
		GameNumber: 1,
		White:      EngineSpec{Name: "W"},
		Black:      EngineSpec{Name: "B"},
	}}

	out := sch.Run(context.Background(), pairings)
	if len(out) != 1 {
		t.Fatalf("len(outcomes) = %d, want 1", len(out))
	}
	o := out[0]
	if o.Err != nil {
		t.Fatalf("Err = %v", o.Err)
	}
	if o.Result == nil || o.Result.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %v, want BlackWins", o.Result)
	}
	if o.PGN == "" {
		t.Error("PGN empty")
	}
}

func TestSchedulerPreservesOutcomeOrder(t *testing.T) {
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"e2e4", "g1f3"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5", "g8f6"}) },
	}
	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 4,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
	})

	pairings := make([]Pairing, 8)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			Round:      "r",
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	out := sch.Run(context.Background(), pairings)
	if len(out) != len(pairings) {
		t.Fatalf("len(outcomes) = %d, want %d", len(out), len(pairings))
	}
	for i, o := range out {
		if o.Pairing.GameNumber != i+1 {
			t.Errorf("outcomes[%d].GameNumber = %d, want %d", i, o.Pairing.GameNumber, i+1)
		}
		if o.Err != nil {
			t.Errorf("outcomes[%d].Err = %v", i, o.Err)
		}
	}
}

func TestSchedulerRunsConcurrently(t *testing.T) {
	// Each engine takes 100ms per move. With 4 games at concurrency=4,
	// total wall time should be roughly one game's worth (~400ms+),
	// not 4x that.
	const perMove = 100 * time.Millisecond
	const perGame = 4 * perMove // 4 plies per game

	st := scriptedTable{
		"W": func() *scripted {
			e := newScripted([]string{"e2e4", "g1f3"})
			e.delay = perMove
			return e
		},
		"B": func() *scripted {
			e := newScripted([]string{"e7e5", "g8f6"})
			e.delay = perMove
			return e
		},
	}

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 4,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
	})

	pairings := make([]Pairing, 4)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	start := time.Now()
	out := sch.Run(context.Background(), pairings)
	elapsed := time.Since(start)

	for i, o := range out {
		if o.Err != nil {
			t.Errorf("outcomes[%d].Err = %v", i, o.Err)
		}
	}
	// Sequential would be 4 * perGame = 1.6s; allow generous slack but
	// fail if this took anywhere near sequential time.
	if elapsed > 2*perGame {
		t.Errorf("elapsed = %v, want closer to %v (parallel) than %v (sequential)",
			elapsed, perGame, 4*perGame)
	}
}

func TestSchedulerConcurrencyOne(t *testing.T) {
	// With concurrency=1, only one game runs at a time. Use a counter
	// to assert at most one runOne is in flight.
	var inflight, peak atomic.Int32

	st := scriptedTable{
		"W": func() *scripted {
			e := newScripted([]string{"e2e4"})
			e.delay = 30 * time.Millisecond
			return e
		},
		"B": func() *scripted { return newScripted([]string{"e7e5"}) },
	}

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 1,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
		OnGameStart: func(_ Pairing) {
			n := inflight.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
		},
		OnGameComplete: func(_ GameOutcome) { inflight.Add(-1) },
	})

	pairings := make([]Pairing, 5)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	out := sch.Run(context.Background(), pairings)
	for i, o := range out {
		if o.Err != nil {
			t.Errorf("outcomes[%d].Err = %v", i, o.Err)
		}
	}
	if peak.Load() != 1 {
		t.Errorf("peak inflight = %d, want 1", peak.Load())
	}
}

func TestSchedulerFactoryErrorPropagated(t *testing.T) {
	// Black's factory always fails — white should still spawn and stop
	// cleanly, and the outcome should carry the error.
	stopped := false
	factory := func(ctx context.Context, spec EngineSpec) (*RunningEngine, error) {
		if spec.Name == "B" {
			return nil, errors.New("binary not found")
		}
		return &RunningEngine{
			Engine: newScripted(nil),
			Stop:   func() error { stopped = true; return nil },
		}, nil
	}

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     factory,
		TimeControl: TimeControl{FixedDepth: 1},
	})

	out := sch.Run(context.Background(), []Pairing{{
		GameNumber: 1,
		White:      EngineSpec{Name: "W"},
		Black:      EngineSpec{Name: "B"},
	}})

	if len(out) != 1 || out[0].Err == nil {
		t.Fatalf("expected one outcome with error, got %+v", out)
	}
	if !stopped {
		t.Error("white should have been Stopped after black setup failed")
	}
}

func TestSchedulerCallbacksFire(t *testing.T) {
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"e2e4"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5"}) },
	}

	var (
		mu        sync.Mutex
		started   []int
		completed []int
	)
	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 2,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
		OnGameStart: func(p Pairing) {
			mu.Lock()
			started = append(started, p.GameNumber)
			mu.Unlock()
		},
		OnGameComplete: func(o GameOutcome) {
			mu.Lock()
			completed = append(completed, o.Pairing.GameNumber)
			mu.Unlock()
		},
	})

	pairings := []Pairing{
		{GameNumber: 1, White: EngineSpec{Name: "W"}, Black: EngineSpec{Name: "B"}},
		{GameNumber: 2, White: EngineSpec{Name: "W"}, Black: EngineSpec{Name: "B"}},
		{GameNumber: 3, White: EngineSpec{Name: "W"}, Black: EngineSpec{Name: "B"}},
	}
	sch.Run(context.Background(), pairings)

	mu.Lock()
	defer mu.Unlock()
	if len(started) != 3 || len(completed) != 3 {
		t.Errorf("started=%v completed=%v, want both length 3", started, completed)
	}
}

func TestSchedulerShouldStopHaltsDispatch(t *testing.T) {
	// ShouldStop returns true after the first game; remaining pairings
	// should be marked ErrSchedulerStopped without ever spawning engines.
	var spawned atomic.Int32
	factory := func(_ context.Context, spec EngineSpec) (*RunningEngine, error) {
		spawned.Add(1)
		var moves []string
		switch spec.Name {
		case "W":
			moves = []string{"f2f3", "g2g4"}
		case "B":
			moves = []string{"e7e5", "d8h4"}
		}
		return &RunningEngine{Engine: newScripted(moves), Stop: func() error { return nil }}, nil
	}

	var completed atomic.Int32
	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     factory,
		Concurrency: 1,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
		ShouldStop: func(_ GameOutcome) bool {
			return completed.Add(1) >= 2
		},
	})

	pairings := make([]Pairing, 5)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	out := sch.Run(context.Background(), pairings)
	if len(out) != 5 {
		t.Fatalf("len(outcomes) = %d, want 5", len(out))
	}

	completedCount, stoppedCount := 0, 0
	for _, o := range out {
		switch {
		case o.Err == nil:
			completedCount++
		case errors.Is(o.Err, ErrSchedulerStopped):
			stoppedCount++
		default:
			t.Errorf("unexpected error %v on game %d", o.Err, o.Pairing.GameNumber)
		}
	}
	if completedCount != 2 {
		t.Errorf("completed = %d, want exactly 2 before stop", completedCount)
	}
	if stoppedCount != 3 {
		t.Errorf("stopped = %d, want 3 (games 3-5 skipped)", stoppedCount)
	}
	if got := spawned.Load(); got > 4 {
		t.Errorf("spawned = %d engine instances, want at most 4 (2 games)", got)
	}
}

func TestSchedulerSPRTStopper(t *testing.T) {
	// Cand wins every game (Fool's mate). SPRT should accept H1 within
	// a few dozen games and most pairings should be skipped.
	st := scriptedTable{
		"Cand": func() *scripted { return newScripted([]string{"f2f3", "g2g4"}) },
		"Base": func() *scripted { return newScripted([]string{"e7e5", "d8h4"}) },
	}

	var decided atomic.Bool
	stopper := NewSPRTStopper(
		SPRTConfig{Elo0: 0, Elo1: 20, Alpha: 0.05, Beta: 0.05},
		"Cand",
		func(SPRTResult) { decided.Store(true) },
	)

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 1, // serialize so the decision is deterministic
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
		ShouldStop:  stopper,
	})

	pairings := make([]Pairing, 200)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "Cand"},
			Black:      EngineSpec{Name: "Base"},
		}
	}

	out := sch.Run(context.Background(), pairings)
	if len(out) != 200 {
		t.Fatalf("len(outcomes) = %d, want 200", len(out))
	}
	if !decided.Load() {
		t.Fatal("SPRT stopper never reached a decision")
	}
	stoppedCount := 0
	for _, o := range out {
		if errors.Is(o.Err, ErrSchedulerStopped) {
			stoppedCount++
		}
	}
	if stoppedCount < 100 {
		t.Errorf("stopped = %d, want >100 (most games skipped early)", stoppedCount)
	}
}

func TestSchedulerContextCancelled(t *testing.T) {
	// Block in the factory until ctx fires so we can cancel during
	// scheduling. Pairings that never reach the factory should surface
	// ctx.Err in their outcome.
	st := scriptedTable{
		"W": func() *scripted {
			e := newScripted([]string{"e2e4"})
			e.delay = time.Second
			return e
		},
		"B": func() *scripted {
			e := newScripted([]string{"e7e5"})
			e.delay = time.Second
			return e
		},
	}

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 1,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
	})

	pairings := make([]Pairing, 4)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	out := sch.Run(ctx, pairings)
	if len(out) != len(pairings) {
		t.Fatalf("len(outcomes) = %d, want %d", len(out), len(pairings))
	}

	// At least one game should report context cancellation. The first
	// game might complete normally if it raced ahead, but later games
	// should not all be successes.
	cancelled := 0
	for _, o := range out {
		if errors.Is(o.Err, context.Canceled) {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Errorf("expected at least one outcome with context.Canceled; got none")
	}
}

func TestSchedulerPauseDelaysDispatch(t *testing.T) {
	// Pause before Run, kick off dispatch in a goroutine, verify nothing
	// completes until Resume is called.
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"e2e4"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5"}) },
	}

	var completed atomic.Int32
	sch, _ := NewScheduler(SchedulerConfig{
		Factory:        st.factory,
		Concurrency:    1,
		TimeControl:    TimeControl{FixedDepth: 1},
		MaxPlies:       2,
		OnGameComplete: func(_ GameOutcome) { completed.Add(1) },
	})

	pairings := make([]Pairing, 3)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	sch.Pause()
	if !sch.Paused() {
		t.Error("Paused() = false after Pause()")
	}

	done := make(chan []GameOutcome, 1)
	go func() {
		done <- sch.Run(context.Background(), pairings)
	}()

	// While paused, no game should complete.
	time.Sleep(50 * time.Millisecond)
	if n := completed.Load(); n != 0 {
		t.Errorf("completed = %d while paused, want 0", n)
	}
	select {
	case out := <-done:
		t.Fatalf("Run returned while paused: %v", out)
	default:
	}

	sch.Resume()
	if sch.Paused() {
		t.Error("Paused() = true after Resume()")
	}

	select {
	case out := <-done:
		if len(out) != 3 {
			t.Errorf("len(outcomes) = %d, want 3", len(out))
		}
		for i, o := range out {
			if o.Err != nil {
				t.Errorf("outcomes[%d].Err = %v", i, o.Err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not complete after Resume")
	}
}

func TestSchedulerPauseMidRun(t *testing.T) {
	// Run starts, complete one game, pause, verify subsequent games
	// don't start until Resume.
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"e2e4"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5"}) },
	}

	var (
		mu        sync.Mutex
		started   int
		completed int
		sch       *Scheduler
	)
	sch, _ = NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 1,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
		OnGameStart: func(_ Pairing) {
			mu.Lock()
			started++
			mu.Unlock()
		},
		OnGameComplete: func(_ GameOutcome) {
			mu.Lock()
			completed++
			c := completed
			mu.Unlock()
			if c == 1 {
				// Pause from inside the first game's completion callback.
				sch.Pause()
			}
		},
	})

	pairings := make([]Pairing, 4)
	for i := range pairings {
		pairings[i] = Pairing{
			GameNumber: i + 1,
			White:      EngineSpec{Name: "W"},
			Black:      EngineSpec{Name: "B"},
		}
	}

	done := make(chan []GameOutcome, 1)
	go func() {
		done <- sch.Run(context.Background(), pairings)
	}()

	// Wait long enough for the next dispatch to be blocked.
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if started != 1 || completed != 1 {
		t.Errorf("after pause: started=%d completed=%d, want 1/1", started, completed)
	}
	mu.Unlock()

	sch.Resume()

	select {
	case out := <-done:
		if len(out) != 4 {
			t.Errorf("len(outcomes) = %d, want 4", len(out))
		}
		for i, o := range out {
			if o.Err != nil {
				t.Errorf("outcomes[%d].Err = %v", i, o.Err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not complete after Resume")
	}
}

func TestSchedulerPauseCancelledByContext(t *testing.T) {
	// Cancelling ctx while paused should release the wait and surface
	// context.Canceled on remaining pairings.
	st := scriptedTable{
		"W": func() *scripted { return newScripted([]string{"e2e4"}) },
		"B": func() *scripted { return newScripted([]string{"e7e5"}) },
	}

	sch, _ := NewScheduler(SchedulerConfig{
		Factory:     st.factory,
		Concurrency: 1,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
	})

	pairings := []Pairing{{
		GameNumber: 1,
		White:      EngineSpec{Name: "W"},
		Black:      EngineSpec{Name: "B"},
	}}

	sch.Pause()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	out := sch.Run(ctx, pairings)
	if len(out) != 1 {
		t.Fatalf("len(outcomes) = %d, want 1", len(out))
	}
	if !errors.Is(out[0].Err, context.Canceled) {
		t.Errorf("outcomes[0].Err = %v, want context.Canceled", out[0].Err)
	}
}
