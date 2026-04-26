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
	"strconv"
	"strings"
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
	InfoChannel() <-chan uci.AnalysisInfo
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

	// ResignScore is the centipawn threshold (positive) for resign
	// adjudication: a side that sees itself losing by at least this margin
	// (or being mated) for ResignMoves consecutive of its own moves
	// resigns. Disabled when ResignMoves <= 0.
	ResignScore int
	ResignMoves int

	// DrawScore is the centipawn threshold (positive) for draw
	// adjudication: when both sides report a |score| <= DrawScore (no
	// mate scores) for DrawMoves consecutive plies, after at least
	// DrawMinPly plies have been played, the game is adjudicated drawn.
	// Disabled when DrawMoves <= 0.
	DrawScore  int
	DrawMoves  int
	DrawMinPly int

	// Event, Site, and Round populate the matching PGN tags. All optional;
	// missing tags are written as "?".
	Event string
	Site  string
	Round string

	// OnMove, when non-nil, is invoked after each move is appended to the
	// game record. The provided FEN reflects the position after the move.
	// The callback runs on the arbiter's goroutine and must not block.
	OnMove func(rec MoveRecord, fen string)

	// OnInfo, when non-nil, is invoked whenever the side-to-move's engine
	// emits an analysis info line during its search. It runs on the
	// arbiter's goroutine and must not block. Use this to surface live
	// per-engine PV, depth, score, nodes, NPS to the GUI.
	OnInfo func(side chess.Side, info uci.AnalysisInfo)
}

// MoveRecord captures one ply's move and the engine's last reported
// analysis info for that move.
type MoveRecord struct {
	Ply        int        // 1-based half-move number
	Side       chess.Side // who played the move
	UCI        string     // move in UCI long algebraic
	SAN        string     // move in standard algebraic
	Info       uci.AnalysisInfo
	HasInfo    bool          // false when no info line was received
	Elapsed    time.Duration // wall clock used to choose this move
	ClockAfter time.Duration // remaining clock after the move (0 in fixed mode)
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

	// Moves is the per-ply log: move played, engine's last reported
	// analysis info, and clock state.
	Moves []MoveRecord

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

	// Per-period move counters for moves+time controls (0 in sudden death).
	whiteMovesInPeriod int
	blackMovesInPeriod int

	startMoveNum int
	startSide    chess.Side

	resignStreakWhite int
	resignStreakBlack int
	drawStreak        int
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

	startMoveNum, startSide := parseFENMoveNumber(cfg.StartFEN)

	for _, mv := range cfg.StartMoves {
		if err := game.PushUCI(mv); err != nil {
			return nil, fmt.Errorf("arbiter: apply start move %q: %w", mv, err)
		}
	}

	return &Arbiter{
		cfg:          cfg,
		game:         game,
		whiteClock:   cfg.TimeControl.Initial,
		blackClock:   cfg.TimeControl.Initial,
		startMoveNum: startMoveNum,
		startSide:    startSide,
	}, nil
}

// parseFENMoveNumber extracts the full move number and side to move
// from a FEN string. Empty input means startpos (move 1, white).
func parseFENMoveNumber(fen string) (int, chess.Side) {
	if fen == "" {
		return 1, chess.White
	}
	fields := strings.Fields(fen)
	side := chess.White
	if len(fields) >= 2 && fields[1] == "b" {
		side = chess.Black
	}
	moveNum := 1
	if len(fields) >= 6 {
		if n, err := strconv.Atoi(fields[5]); err == nil && n >= 1 {
			moveNum = n
		}
	}
	return moveNum, side
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

		params := a.buildGoParams(side)
		deadline := a.moveDeadline(side)

		moveStart := time.Now()
		if err := engine.Go(params); err != nil {
			a.adjudicateForfeit(result, side, name, chess.ReasonAdjudication, fmt.Errorf("go: %w", err))
			return result, nil
		}

		bm, info, hasInfo, err := a.awaitBestMove(ctx, engine, deadline, side)
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

		rec := a.recordMove(side, bm.Move, info, hasInfo, elapsed)
		result.Moves = append(result.Moves, rec)
		if a.cfg.OnMove != nil {
			a.cfg.OnMove(rec, a.game.FEN())
		}

		if outcome, reason, loser, ok := a.checkAdjudication(rec); ok {
			switch outcome {
			case chess.Drawn:
				a.adjudicateDraw(result, reason, nil)
			default:
				_, loserName := a.engineFor(loser)
				a.adjudicateForfeit(result, loser, loserName, reason,
					fmt.Errorf("score adjudication: %s", outcome))
			}
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

func (a *Arbiter) buildGoParams(side chess.Side) uci.GoParams {
	tc := a.cfg.TimeControl
	switch {
	case tc.FixedMovetime > 0:
		return uci.GoParams{MoveTime: tc.FixedMovetime}
	case tc.FixedNodes > 0:
		return uci.GoParams{Nodes: tc.FixedNodes}
	case tc.FixedDepth > 0:
		return uci.GoParams{Depth: tc.FixedDepth}
	}
	movesToGo := 0
	if tc.MovesPerPeriod > 0 {
		played := a.whiteMovesInPeriod
		if side == chess.Black {
			played = a.blackMovesInPeriod
		}
		movesToGo = tc.MovesPerPeriod - played
	}
	return uci.GoParams{
		WhiteTime: a.whiteClock,
		BlackTime: a.blackClock,
		WhiteInc:  tc.Increment,
		BlackInc:  tc.Increment,
		MovesToGo: movesToGo,
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
	var clock *time.Duration
	var movesInPeriod *int
	if side == chess.White {
		clock = &a.whiteClock
		movesInPeriod = &a.whiteMovesInPeriod
	} else {
		clock = &a.blackClock
		movesInPeriod = &a.blackMovesInPeriod
	}

	*clock -= elapsed
	if *clock > 0 {
		*clock += tc.Increment
	}

	// Moves+time: when this side completes the Nth move of the period, add
	// another Initial allotment to their clock and start a fresh period.
	// Only credit if the side actually survived the move (clock > 0); a
	// flagged engine still forfeits via clockExpired.
	if tc.MovesPerPeriod > 0 {
		*movesInPeriod++
		if *movesInPeriod >= tc.MovesPerPeriod {
			if *clock > 0 {
				*clock += tc.Initial
			}
			*movesInPeriod = 0
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

// awaitBestMove blocks until the engine produces a bestmove, the deadline
// elapses, the context is cancelled, or the engine crashes. While waiting
// it drains analysis info; the latest one is returned alongside the move.
// hasInfo is true iff the engine sent at least one info line for this
// search. If Config.OnInfo is set it is called at most every 100ms per
// search so the GUI can show live PV/score/depth.
func (a *Arbiter) awaitBestMove(ctx context.Context, engine Engine, deadline time.Duration, side chess.Side) (uci.BestMove, uci.AnalysisInfo, bool, error) {
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	bestMoveCh := engine.BestMoveChannel()
	infoCh := engine.InfoChannel()

	var (
		latest      uci.AnalysisInfo
		hasInfo     bool
		lastInfoEmit time.Time
	)
	emit := func(info uci.AnalysisInfo) {
		if a.cfg.OnInfo == nil {
			return
		}
		now := time.Now()
		if now.Sub(lastInfoEmit) < 100*time.Millisecond {
			return
		}
		lastInfoEmit = now
		a.cfg.OnInfo(side, info)
	}

	for {
		select {
		case bm, ok := <-bestMoveCh:
			if !ok {
				return uci.BestMove{}, latest, hasInfo, errEngineCrashed
			}
			// Drain any pending info before returning so we capture the
			// engine's final eval even if it raced with bestmove.
			for {
				select {
				case info, ok := <-infoCh:
					if !ok {
						return bm, latest, hasInfo, nil
					}
					latest = info
					hasInfo = true
				default:
					// Final emit so the UI sees the engine's last word
					// for this move.
					if hasInfo && a.cfg.OnInfo != nil {
						a.cfg.OnInfo(side, latest)
					}
					return bm, latest, hasInfo, nil
				}
			}
		case info, ok := <-infoCh:
			if !ok {
				infoCh = nil
				continue
			}
			latest = info
			hasInfo = true
			emit(info)
		case <-timer.C:
			return uci.BestMove{}, latest, hasInfo, errMoveTimeout
		case <-ctx.Done():
			return uci.BestMove{}, latest, hasInfo, ctx.Err()
		}
	}
}

// recordMove builds a MoveRecord from the move just played. SAN is read
// off the game's history to avoid duplicating notnil/chess's encoding.
func (a *Arbiter) recordMove(side chess.Side, move string, info uci.AnalysisInfo, hasInfo bool, elapsed time.Duration) MoveRecord {
	sans := a.game.MovesSAN()
	san := ""
	if len(sans) > 0 {
		san = sans[len(sans)-1]
	}
	clock := a.whiteClock
	if side == chess.Black {
		clock = a.blackClock
	}
	if a.cfg.TimeControl.fixed() {
		clock = 0
	}
	return MoveRecord{
		Ply:        len(sans),
		Side:       side,
		UCI:        move,
		SAN:        san,
		Info:       info,
		HasInfo:    hasInfo,
		Elapsed:    elapsed,
		ClockAfter: clock,
	}
}

// checkAdjudication returns whether the latest move triggers a score-based
// adjudication. The first three return values are meaningful only when
// the bool is true. For draw outcomes, the returned side is ignored.
func (a *Arbiter) checkAdjudication(rec MoveRecord) (chess.Outcome, chess.Reason, chess.Side, bool) {
	losing := scoreShowsLoss(rec.Info.Score, a.cfg.ResignScore)
	drawn := scoreShowsDraw(rec.Info.Score, a.cfg.DrawScore)

	if !rec.HasInfo {
		// No info this ply: reset draw streak (we can't claim a peaceful
		// position we didn't observe). Resign streaks are per-side and
		// only update on the moving side's plies, so leave them.
		a.drawStreak = 0
	} else {
		if drawn {
			a.drawStreak++
		} else {
			a.drawStreak = 0
		}
	}

	if rec.Side == chess.White {
		if losing {
			a.resignStreakWhite++
		} else if rec.HasInfo {
			a.resignStreakWhite = 0
		}
	} else {
		if losing {
			a.resignStreakBlack++
		} else if rec.HasInfo {
			a.resignStreakBlack = 0
		}
	}

	if a.cfg.ResignMoves > 0 {
		if a.resignStreakWhite >= a.cfg.ResignMoves {
			return chess.BlackWins, chess.ReasonResignation, chess.White, true
		}
		if a.resignStreakBlack >= a.cfg.ResignMoves {
			return chess.WhiteWins, chess.ReasonResignation, chess.Black, true
		}
	}

	if a.cfg.DrawMoves > 0 && a.drawStreak >= a.cfg.DrawMoves &&
		len(a.game.MovesUCI()) >= a.cfg.DrawMinPly {
		return chess.Drawn, chess.ReasonAdjudication, "", true
	}

	return chess.Ongoing, chess.ReasonInProgress, "", false
}

// scoreShowsLoss reports whether the score, from the moving side's POV,
// indicates that side is losing by at least thresholdCp centipawns or is
// being mated. Returns false when the score is unset or the threshold is
// unconfigured.
func scoreShowsLoss(s uci.Score, thresholdCp int) bool {
	if thresholdCp <= 0 {
		return false
	}
	if s.Mate != nil {
		return *s.Mate < 0
	}
	if s.Centipawns != nil {
		return *s.Centipawns <= -thresholdCp
	}
	return false
}

// scoreShowsDraw reports whether the score is within ±thresholdCp and
// not a mate score. Returns false when the score is unset or the
// threshold is unconfigured.
func scoreShowsDraw(s uci.Score, thresholdCp int) bool {
	if thresholdCp < 0 {
		return false
	}
	if s.Mate != nil {
		return false
	}
	if s.Centipawns == nil {
		return false
	}
	cp := *s.Centipawns
	if cp < 0 {
		cp = -cp
	}
	return cp <= thresholdCp
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

// AnnotatedPGN renders the finished game as PGN with embedded
// [%eval ...] and [%clk ...] annotations per ply. Eval is normalized to
// white's POV.
func (a *Arbiter) AnnotatedPGN(result *Result) string {
	var sb strings.Builder

	tag := func(name, value string) {
		if value == "" {
			value = "?"
		}
		fmt.Fprintf(&sb, "[%s \"%s\"]\n", name, value)
	}

	tag("Event", a.cfg.Event)
	tag("Site", a.cfg.Site)
	if !result.StartedAt.IsZero() {
		tag("Date", result.StartedAt.Format("2006.01.02"))
	} else {
		tag("Date", "")
	}
	tag("Round", a.cfg.Round)
	tag("White", a.cfg.WhiteName)
	tag("Black", a.cfg.BlackName)

	resultStr := string(result.Outcome)
	if resultStr == "" || resultStr == string(chess.Ongoing) {
		resultStr = "*"
	}
	tag("Result", resultStr)

	if a.cfg.StartFEN != "" {
		tag("FEN", a.cfg.StartFEN)
		tag("SetUp", "1")
	}
	if tcStr := formatTimeControlPGN(a.cfg.TimeControl); tcStr != "" {
		tag("TimeControl", tcStr)
	}
	if result.Reason != "" && result.Reason != chess.ReasonInProgress {
		tag("Termination", string(result.Reason))
	}

	sb.WriteByte('\n')

	moveNum := a.startMoveNum
	for i, rec := range result.Moves {
		if rec.Side == chess.White {
			fmt.Fprintf(&sb, "%d. ", moveNum)
		} else if i == 0 {
			fmt.Fprintf(&sb, "%d... ", moveNum)
		}
		sb.WriteString(rec.SAN)
		if anno := formatAnnotation(rec); anno != "" {
			fmt.Fprintf(&sb, " {%s}", anno)
		}
		if rec.Side == chess.Black {
			moveNum++
		}
		if i+1 < len(result.Moves) {
			sb.WriteByte(' ')
		}
	}

	if len(result.Moves) > 0 {
		sb.WriteByte(' ')
	}
	sb.WriteString(resultStr)
	sb.WriteByte('\n')

	return sb.String()
}

func formatAnnotation(rec MoveRecord) string {
	var parts []string
	if eval := formatEval(rec.Info.Score, rec.Side); eval != "" {
		parts = append(parts, "[%eval "+eval+"]")
	}
	if rec.ClockAfter > 0 {
		parts = append(parts, "[%clk "+formatClock(rec.ClockAfter)+"]")
	}
	return strings.Join(parts, " ")
}

// formatEval renders a UCI score in the [%eval ...] convention: white's
// POV, "+0.42" / "-0.10" for centipawns, "#5" / "#-3" for mate scores.
func formatEval(s uci.Score, mover chess.Side) string {
	sign := 1
	if mover == chess.Black {
		sign = -1
	}
	if s.Mate != nil {
		m := *s.Mate * sign
		return fmt.Sprintf("#%d", m)
	}
	if s.Centipawns != nil {
		cp := float64(*s.Centipawns*sign) / 100.0
		if cp >= 0 {
			return fmt.Sprintf("+%.2f", cp)
		}
		return fmt.Sprintf("%.2f", cp)
	}
	return ""
}

func formatClock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalMs := d.Milliseconds()
	h := totalMs / 3_600_000
	totalMs -= h * 3_600_000
	m := totalMs / 60_000
	totalMs -= m * 60_000
	s := totalMs / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%d:%02d:%02d.%03d", h, m, s, ms)
}

// formatTimeControlPGN renders the active time control in PGN's
// "TimeControl" tag form. Empty string means no tag should be written.
func formatTimeControlPGN(tc TimeControl) string {
	switch {
	case tc.FixedMovetime > 0:
		return fmt.Sprintf("movetime/%dms", tc.FixedMovetime.Milliseconds())
	case tc.FixedNodes > 0:
		return fmt.Sprintf("nodes/%d", tc.FixedNodes)
	case tc.FixedDepth > 0:
		return fmt.Sprintf("depth/%d", tc.FixedDepth)
	}
	if tc.Initial == 0 {
		return ""
	}
	base := formatSeconds(tc.Initial)
	if tc.Increment > 0 {
		base += "+" + formatSeconds(tc.Increment)
	}
	if tc.MovesPerPeriod > 0 {
		return fmt.Sprintf("%d/%s", tc.MovesPerPeriod, base)
	}
	return base
}

func formatSeconds(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	if d%time.Second == 0 {
		return strconv.Itoa(int(d / time.Second))
	}
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64)
}
