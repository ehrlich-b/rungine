package tournament

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"rungine/internal/chess"
	"rungine/internal/uci"
)

// ErrSchedulerStopped is recorded on pairings that never started because
// a ShouldStop callback fired (e.g., SPRT early termination).
var ErrSchedulerStopped = errors.New("scheduler stopped early")

// EngineSpec describes how to spawn a UCI engine for a tournament game.
// One spec is reused across all games an engine plays in.
type EngineSpec struct {
	// Name is the display name written to PGN [White]/[Black] tags. It
	// also feeds back into UCI engine IDs for logging.
	Name string

	// BinaryPath is the path passed to the default factory. Custom
	// factories may ignore this field.
	BinaryPath string

	// Options is the set of UCI options applied immediately after the
	// engine reaches the ready state.
	Options map[string]string
}

// Pairing describes one game in the tournament.
type Pairing struct {
	// GameNumber is a monotonic identifier across the tournament. The
	// scheduler does not assign one; callers should set it when building
	// pairings so output can be correlated.
	GameNumber int

	// Round populates the PGN [Round] tag.
	Round string

	White EngineSpec
	Black EngineSpec

	// StartFEN and StartMoves let pairings start from a custom opening.
	// Empty StartFEN means the standard starting position.
	StartFEN   string
	StartMoves []string
}

// RunningEngine wraps an Engine with a stop function. Factories return
// these so the scheduler can release engines after a game without
// requiring Stop() on the Engine interface itself.
type RunningEngine struct {
	Engine Engine
	Stop   func() error
}

// EngineFactory creates and starts a UCI engine from a spec. On success
// the engine must already be in the Ready state and have any UCI options
// applied. Errors are surfaced as GameOutcome.Err.
type EngineFactory func(ctx context.Context, spec EngineSpec) (*RunningEngine, error)

// DefaultEngineFactory spawns a *uci.Engine, calls Start, and applies
// the spec's UCI options. The returned Stop calls eng.Stop, which
// quits the subprocess gracefully and force-kills if it doesn't exit.
func DefaultEngineFactory(ctx context.Context, spec EngineSpec) (*RunningEngine, error) {
	id := spec.Name
	if id == "" {
		id = spec.BinaryPath
	}
	eng := uci.NewEngine(id, spec.BinaryPath)
	if err := eng.Start(ctx); err != nil {
		return nil, fmt.Errorf("start %s: %w", spec.Name, err)
	}
	for k, v := range spec.Options {
		if err := eng.SetOption(k, v); err != nil {
			_ = eng.Stop()
			return nil, fmt.Errorf("setoption %s=%s: %w", k, v, err)
		}
	}
	return &RunningEngine{Engine: eng, Stop: eng.Stop}, nil
}

// SchedulerConfig configures how a tournament is executed.
type SchedulerConfig struct {
	// Concurrency caps the number of games running simultaneously.
	// Defaults to 1 if zero or negative.
	Concurrency int

	// TimeControl is applied to every game.
	TimeControl TimeControl

	// MaxPlies, MoveGrace, and the adjudication knobs are forwarded to
	// each game's arbiter.
	MaxPlies  int
	MoveGrace time.Duration

	ResignScore int
	ResignMoves int
	DrawScore   int
	DrawMoves   int
	DrawMinPly  int

	// Event and Site populate PGN tags. Round comes from the Pairing.
	Event string
	Site  string

	// Factory is required: it provides the per-game engine instances.
	// Use DefaultEngineFactory in production; tests inject mocks.
	Factory EngineFactory

	// Optional callbacks. Invoked from worker goroutines, so they must
	// be safe for concurrent use.
	OnGameStart    func(p Pairing)
	OnGameComplete func(o GameOutcome)

	// ShouldStop, when non-nil, is called after each completed game
	// (also from worker goroutines). Returning true halts dispatch of
	// further pairings; in-flight games complete normally. Pairings
	// that never start get ErrSchedulerStopped on their GameOutcome.Err.
	ShouldStop func(o GameOutcome) bool

	// OnGameMove, when non-nil, is called after each move played in any
	// in-flight game. It runs on the worker goroutine for that game and
	// must not block.
	OnGameMove func(p Pairing, rec MoveRecord, fen string)

	// OnGameInfo, when non-nil, is called when an engine emits live
	// analysis info during its search. side is "w" or "b" matching the
	// engine that produced the info. The arbiter throttles per-search
	// to ~10Hz, so this callback is safe to forward to the GUI.
	OnGameInfo func(p Pairing, side string, info uci.AnalysisInfo)
}

// GameOutcome records one completed game.
type GameOutcome struct {
	Pairing Pairing
	Result  *Result
	PGN     string
	Err     error
}

// Scheduler runs multiple games concurrently using its configured factory.
type Scheduler struct {
	cfg SchedulerConfig

	pauseMu sync.Mutex
	paused  bool
	// resumeCh is closed when the scheduler is not paused and replaced
	// with a fresh open channel each time Pause is called. Dispatch
	// blocks on whichever channel is current at the time it checks.
	resumeCh chan struct{}
}

// NewScheduler constructs a Scheduler. Factory is required.
func NewScheduler(cfg SchedulerConfig) (*Scheduler, error) {
	if cfg.Factory == nil {
		return nil, errors.New("scheduler: Factory required")
	}
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 1
	}
	return &Scheduler{cfg: cfg}, nil
}

// Pause halts dispatch of new pairings. In-flight games run to
// completion. Safe to call from any goroutine. Calling Pause while
// already paused is a no-op.
func (s *Scheduler) Pause() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if s.paused {
		return
	}
	s.paused = true
	s.resumeCh = make(chan struct{})
}

// Resume releases dispatch after a Pause. Calling Resume when not
// paused is a no-op.
func (s *Scheduler) Resume() {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	if !s.paused {
		return
	}
	s.paused = false
	close(s.resumeCh)
	s.resumeCh = nil
}

// Paused reports whether dispatch is currently halted.
func (s *Scheduler) Paused() bool {
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.paused
}

// waitWhilePaused blocks until either the scheduler is resumed or ctx
// fires. Returns ctx.Err on cancellation, nil otherwise.
func (s *Scheduler) waitWhilePaused(ctx context.Context) error {
	s.pauseMu.Lock()
	if !s.paused {
		s.pauseMu.Unlock()
		return nil
	}
	ch := s.resumeCh
	s.pauseMu.Unlock()
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// acquireSlot waits until pause is cleared, acquires a worker slot from
// sem, and re-checks pause. If pause flipped between the wait and the
// acquire, the slot is released and the loop retries. Returns ctx.Err
// if cancelled at any point.
func (s *Scheduler) acquireSlot(ctx context.Context, sem chan struct{}) error {
	for {
		if err := s.waitWhilePaused(ctx); err != nil {
			return err
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
		if !s.Paused() {
			return nil
		}
		// Pause flipped while we waited for sem; release and re-check.
		<-sem
	}
}

// Run executes all pairings, returning outcomes in the same order. Each
// game runs in its own goroutine; up to Concurrency games run
// simultaneously. Run blocks until all dispatched games complete or
// ctx is cancelled. Pairings that never start because of cancellation
// surface ctx.Err in their GameOutcome.
func (s *Scheduler) Run(ctx context.Context, pairings []Pairing) []GameOutcome {
	outcomes := make([]GameOutcome, len(pairings))
	sem := make(chan struct{}, s.cfg.Concurrency)
	var wg sync.WaitGroup
	var stopped atomic.Bool

	for i, p := range pairings {
		if stopped.Load() {
			outcomes[i] = GameOutcome{Pairing: p, Err: ErrSchedulerStopped}
			continue
		}
		if err := s.acquireSlot(ctx, sem); err != nil {
			outcomes[i] = GameOutcome{Pairing: p, Err: err}
			continue
		}
		// Re-check after sem acquire: ShouldStop may have flipped while
		// we were blocked waiting for an in-flight game to release.
		if stopped.Load() {
			<-sem
			outcomes[i] = GameOutcome{Pairing: p, Err: ErrSchedulerStopped}
			continue
		}
		wg.Add(1)
		go func(i int, p Pairing) {
			defer wg.Done()
			defer func() { <-sem }()
			out := s.runOne(ctx, p)
			outcomes[i] = out
			if s.cfg.ShouldStop != nil && s.cfg.ShouldStop(out) {
				stopped.Store(true)
			}
		}(i, p)
	}

	wg.Wait()
	return outcomes
}

func (s *Scheduler) runOne(ctx context.Context, p Pairing) GameOutcome {
	if s.cfg.OnGameStart != nil {
		s.cfg.OnGameStart(p)
	}

	out := GameOutcome{Pairing: p}
	defer func() {
		if s.cfg.OnGameComplete != nil {
			s.cfg.OnGameComplete(out)
		}
	}()

	white, err := s.cfg.Factory(ctx, p.White)
	if err != nil {
		out.Err = fmt.Errorf("white setup: %w", err)
		return out
	}
	defer white.Stop()

	black, err := s.cfg.Factory(ctx, p.Black)
	if err != nil {
		out.Err = fmt.Errorf("black setup: %w", err)
		return out
	}
	defer black.Stop()

	var onMove func(rec MoveRecord, fen string)
	if s.cfg.OnGameMove != nil {
		pairing := p
		hook := s.cfg.OnGameMove
		onMove = func(rec MoveRecord, fen string) {
			hook(pairing, rec, fen)
		}
	}
	var onInfo func(side chess.Side, info uci.AnalysisInfo)
	if s.cfg.OnGameInfo != nil {
		pairing := p
		hook := s.cfg.OnGameInfo
		onInfo = func(side chess.Side, info uci.AnalysisInfo) {
			hook(pairing, string(side), info)
		}
	}
	arb, err := New(Config{
		White:       white.Engine,
		Black:       black.Engine,
		WhiteName:   p.White.Name,
		BlackName:   p.Black.Name,
		StartFEN:    p.StartFEN,
		StartMoves:  p.StartMoves,
		TimeControl: s.cfg.TimeControl,
		MoveGrace:   s.cfg.MoveGrace,
		MaxPlies:    s.cfg.MaxPlies,
		ResignScore: s.cfg.ResignScore,
		ResignMoves: s.cfg.ResignMoves,
		DrawScore:   s.cfg.DrawScore,
		DrawMoves:   s.cfg.DrawMoves,
		DrawMinPly:  s.cfg.DrawMinPly,
		Event:       s.cfg.Event,
		Site:        s.cfg.Site,
		Round:       p.Round,
		OnMove:      onMove,
		OnInfo:      onInfo,
	})
	if err != nil {
		out.Err = fmt.Errorf("arbiter: %w", err)
		return out
	}

	res, err := arb.Run(ctx)
	out.Result = res
	if err != nil {
		out.Err = err
	}
	if res != nil {
		out.PGN = arb.AnnotatedPGN(res)
	}
	return out
}
