// Package tournament arbitrates UCI engine vs engine games.
//
// An Arbiter drives a single game between two engines, enforcing the rules:
// it plays moves into a chess.Game, tracks per-side clocks, detects time
// forfeits, illegal moves, and engine crashes, and produces a final Result
// with the game record.
//
// The Engine interface is the minimal surface an arbiter needs from a UCI
// engine; *uci.Engine implements it, and tests use a scripted mock.
package tournament

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rungine/internal/chess"
	"rungine/internal/uci"
)

// Engine is the minimal UCI surface the arbiter relies on. The real
// *uci.Engine satisfies this interface; tests provide a scripted mock.
type Engine interface {
	SetPosition(fen string, moves []string) error
	Go(params uci.GoParams) error
	StopSearch() error
	BestMoveChannel() <-chan uci.BestMove
}

// TimeControl describes how time is allocated during a game. Exactly one
// mode is active per game, picked in order: FixedMovetime, FixedNodes,
// FixedDepth, then real clock (Initial + Increment, with optional
// MovesPerPeriod).
type TimeControl struct {
	Initial        time.Duration // per-side starting clock
	Increment      time.Duration // added per move
	MovesPerPeriod int           // moves per period (0 = sudden death)

	FixedDepth    int           // depth=N — diagnostic mode
	FixedNodes    int64         // nodes=N — diagnostic mode
	FixedMovetime time.Duration // movetime=ms — diagnostic mode
}

// fixed reports whether the time control uses a fixed search budget rather
// than a real clock. In fixed mode, no per-side clock decrements.
func (tc TimeControl) fixed() bool {
	return tc.FixedMovetime > 0 || tc.FixedNodes > 0 || tc.FixedDepth > 0
}

// Config configures a single game.
type Config struct {
	White Engine
	Black Engine

	WhiteName string
	BlackName string

	// StartFEN is the starting position. Empty string means startpos.
	StartFEN string
	// StartMoves are applied from StartFEN before the engines play.
	StartMoves []string

	TimeControl TimeControl

	// MoveGrace is added to a side's remaining clock when computing how
	// long to wait for a bestmove. A small grace covers IPC overhead.
	// Defaults to 250ms if zero.
	MoveGrace time.Duration

	// MaxPlies caps the game length (0 = no cap). When reached, the game
	// is adjudicated as a draw. Useful for tests and fixed-depth runs.
	MaxPlies int
}

// Result records the outcome of a single arbitrated game.
type Result struct {
	Outcome chess.Outcome
	Reason  chess.Reason

	// Loser is the display name of the losing engine (empty for draws).
	Loser string

	Game *chess.Game

	PlyCount int

	WhiteClock time.Duration // remaining at end of game
	BlackClock time.Duration

	StartedAt time.Time
	EndedAt   time.Time

	// Cause carries detail when the game ended via forfeit (empty for
	// natural terminations).
	Cause error
}

// Arbiter runs a single game between two engines.
type Arbiter struct {
	cfg  Config
	game *chess.Game

	whiteClock time.Duration
	blackClock time.Duration
}

// New constructs an Arbiter from a Config. Any starting moves are applied
// up front so the engines see the same position the arbiter does.
func New(cfg Config) (*Arbiter, error) {
	if cfg.White == nil || cfg.Black == nil {
		return nil, errors.New("arbiter: White and Black engines required")
	}
	if cfg.MoveGrace == 0 {
		cfg.MoveGrace = 250 * time.Millisecond
	}

	var game *chess.Game
	if cfg.StartFEN == "" {
		game = chess.NewGame()
	} else {
		g, err := chess.FromFEN(cfg.StartFEN)
		if err != nil {
			return nil, fmt.Errorf("arbiter: start FEN: %w", err)
		}
		game = g
	}
	for _, mv := range cfg.StartMoves {
		if err := game.PushUCI(mv); err != nil {
			return nil, fmt.Errorf("arbiter: apply start move %q: %w", mv, err)
		}
	}

	return &Arbiter{
		cfg:        cfg,
		game:       game,
		whiteClock: cfg.TimeControl.Initial,
		blackClock: cfg.TimeControl.Initial,
	}, nil
}

// Game returns the underlying chess.Game (live; updated as moves are made).
func (a *Arbiter) Game() *chess.Game {
	return a.game
}

// Run plays the game to completion, returning the Result. Run respects
// context cancellation between moves and while waiting for bestmoves.
func (a *Arbiter) Run(ctx context.Context) (*Result, error) {
	result := &Result{
		Game:      a.game,
		StartedAt: time.Now(),
	}
	defer func() {
		result.EndedAt = time.Now()
		result.PlyCount = len(a.game.MovesUCI())
		result.WhiteClock = a.whiteClock
		result.BlackClock = a.blackClock
	}()

	if err := a.broadcastPosition(); err != nil {
		return nil, err
	}

	for a.game.Outcome() == chess.Ongoing {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if a.cfg.MaxPlies > 0 && len(a.game.MovesUCI()) >= a.cfg.MaxPlies {
			a.adjudicateDraw(result, chess.ReasonAdjudication, errors.New("max plies reached"))
			return result, nil
		}

		side := a.game.SideToMove()
		engine, name := a.engineFor(side)

		params := a.buildGoParams()
		deadline := a.moveDeadline(side)

		moveStart := time.Now()
		if err := engine.Go(params); err != nil {
			a.adjudicateForfeit(result, side, name, chess.ReasonAdjudication, fmt.Errorf("go: %w", err))
			return result, nil
		}

		bm, err := a.awaitBestMove(ctx, engine, deadline)
		elapsed := time.Since(moveStart)

		if err != nil {
			// Tell the engine to abort its search before we move on.
			_ = engine.StopSearch()
			switch {
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				return result, err
			case errors.Is(err, errMoveTimeout):
				a.adjudicateForfeit(result, side, name, chess.ReasonTimeForfeit, err)
				return result, nil
			case errors.Is(err, errEngineCrashed):
				a.adjudicateForfeit(result, side, name, chess.ReasonAdjudication, err)
				return result, nil
			default:
				a.adjudicateForfeit(result, side, name, chess.ReasonAdjudication, err)
				return result, nil
			}
		}

		// Update the clock before validating the move so that a side that
		// flagged exactly at move time forfeits correctly.
		if !a.cfg.TimeControl.fixed() {
			a.tickClock(side, elapsed)
			if a.clockExpired(side) {
				a.adjudicateForfeit(result, side, name, chess.ReasonTimeForfeit,
					fmt.Errorf("clock expired during move (elapsed=%v)", elapsed))
				return result, nil
			}
		}

		if bm.Move == "" || bm.Move == "0000" {
			a.adjudicateForfeit(result, side, name, chess.ReasonIllegalMove,
				fmt.Errorf("engine returned empty bestmove %q", bm.Move))
			return result, nil
		}

		if err := a.game.PushUCI(bm.Move); err != nil {
			a.adjudicateForfeit(result, side, name, chess.ReasonIllegalMove, err)
			return result, nil
		}

		if err := a.broadcastPosition(); err != nil {
			return nil, err
		}
	}

	// Natural termination — checkmate, stalemate, threefold, etc.
	result.Outcome = a.game.Outcome()
	result.Reason = a.game.Reason()
	return result, nil
}

func (a *Arbiter) engineFor(side chess.Side) (Engine, string) {
	if side == chess.White {
		return a.cfg.White, a.cfg.WhiteName
	}
	return a.cfg.Black, a.cfg.BlackName
}

func (a *Arbiter) buildGoParams() uci.GoParams {
	tc := a.cfg.TimeControl
	switch {
	case tc.FixedMovetime > 0:
		return uci.GoParams{MoveTime: tc.FixedMovetime}
	case tc.FixedNodes > 0:
		return uci.GoParams{Nodes: tc.FixedNodes}
	case tc.FixedDepth > 0:
		return uci.GoParams{Depth: tc.FixedDepth}
	}
	return uci.GoParams{
		WhiteTime: a.whiteClock,
		BlackTime: a.blackClock,
		WhiteInc:  tc.Increment,
		BlackInc:  tc.Increment,
		MovesToGo: tc.MovesPerPeriod,
	}
}

func (a *Arbiter) moveDeadline(side chess.Side) time.Duration {
	tc := a.cfg.TimeControl
	switch {
	case tc.FixedMovetime > 0:
		return tc.FixedMovetime + 5*time.Second
	case tc.FixedNodes > 0, tc.FixedDepth > 0:
		// Fixed search budgets self-terminate; cap at a generous wall
		// time so a misbehaving engine can't hang the arbiter.
		return 10 * time.Minute
	}
	clock := a.whiteClock
	if side == chess.Black {
		clock = a.blackClock
	}
	return clock + a.cfg.MoveGrace
}

func (a *Arbiter) tickClock(side chess.Side, elapsed time.Duration) {
	tc := a.cfg.TimeControl
	if side == chess.White {
		a.whiteClock -= elapsed
		if a.whiteClock > 0 {
			a.whiteClock += tc.Increment
		}
	} else {
		a.blackClock -= elapsed
		if a.blackClock > 0 {
			a.blackClock += tc.Increment
		}
	}
}

func (a *Arbiter) clockExpired(side chess.Side) bool {
	if a.cfg.TimeControl.Initial == 0 {
		return false
	}
	if side == chess.White {
		return a.whiteClock <= 0
	}
	return a.blackClock <= 0
}

func (a *Arbiter) broadcastPosition() error {
	moves := a.game.MovesUCI()
	all := make([]string, 0, len(a.cfg.StartMoves)+len(moves))
	all = append(all, a.cfg.StartMoves...)
	all = append(all, moves...)

	if err := a.cfg.White.SetPosition(a.cfg.StartFEN, all); err != nil {
		return fmt.Errorf("white setposition: %w", err)
	}
	if err := a.cfg.Black.SetPosition(a.cfg.StartFEN, all); err != nil {
		return fmt.Errorf("black setposition: %w", err)
	}
	return nil
}

var (
	errMoveTimeout   = errors.New("move timeout")
	errEngineCrashed = errors.New("engine crashed")
)

func (a *Arbiter) awaitBestMove(ctx context.Context, engine Engine, deadline time.Duration) (uci.BestMove, error) {
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	select {
	case bm, ok := <-engine.BestMoveChannel():
		if !ok {
			return uci.BestMove{}, errEngineCrashed
		}
		return bm, nil
	case <-timer.C:
		return uci.BestMove{}, errMoveTimeout
	case <-ctx.Done():
		return uci.BestMove{}, ctx.Err()
	}
}

func (a *Arbiter) adjudicateForfeit(result *Result, loser chess.Side, loserName string, reason chess.Reason, cause error) {
	winner := chess.WhiteWins
	if loser == chess.White {
		winner = chess.BlackWins
	}

	if err := a.game.Adjudicate(winner, reason); err != nil {
		// Already terminated; record what we wanted to.
	}
	result.Outcome = winner
	result.Reason = reason
	result.Loser = loserName
	result.Cause = cause
}

func (a *Arbiter) adjudicateDraw(result *Result, reason chess.Reason, cause error) {
	if err := a.game.Adjudicate(chess.Drawn, reason); err != nil {
		// Already terminated; record what we wanted to.
	}
	result.Outcome = chess.Drawn
	result.Reason = reason
	result.Cause = cause
}
