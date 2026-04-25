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
