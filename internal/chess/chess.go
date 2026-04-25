// Package chess provides chess game state for the tournament arbiter.
//
// It wraps github.com/notnil/chess to give the arbiter a focused, UCI-oriented
// API: load a position, push UCI moves, query outcome and termination reason.
// Threefold repetition and the fifty-move rule are auto-claimed when eligible.
package chess

import (
	"errors"
	"fmt"

	notchess "github.com/notnil/chess"
)

// Outcome is the high-level game result.
type Outcome string

const (
	Ongoing   Outcome = "*"
	WhiteWins Outcome = "1-0"
	BlackWins Outcome = "0-1"
	Drawn     Outcome = "1/2-1/2"
)

// Reason explains why a game ended (or is ongoing).
type Reason string

const (
	ReasonInProgress      Reason = ""
	ReasonCheckmate       Reason = "checkmate"
	ReasonStalemate       Reason = "stalemate"
	ReasonInsufficient    Reason = "insufficient material"
	ReasonFiftyMove       Reason = "fifty-move rule"
	ReasonThreefold       Reason = "threefold repetition"
	ReasonResignation     Reason = "resignation"
	ReasonIllegalMove     Reason = "illegal move"
	ReasonTimeForfeit     Reason = "time forfeit"
	ReasonAdjudication    Reason = "adjudication"
)

// Side is "w" or "b".
type Side string

const (
	White Side = "w"
	Black Side = "b"
)

// Game holds chess state for a single in-progress or finished game.
type Game struct {
	inner  *notchess.Game
	reason Reason
}

// NewGame returns a new game at the standard starting position.
func NewGame() *Game {
	return &Game{inner: notchess.NewGame()}
}

// FromFEN returns a new game starting at the given FEN.
func FromFEN(fen string) (*Game, error) {
	opt, err := notchess.FEN(fen)
	if err != nil {
		return nil, fmt.Errorf("chess: parse FEN: %w", err)
	}
	return &Game{inner: notchess.NewGame(opt)}, nil
}

// PushUCI applies a move given in UCI long algebraic notation
// (e.g., "e2e4", "e1g1", "e7e8q"). It returns an error if the
// engine is finished or the move is illegal in the current position.
//
// After the move applies, threefold repetition and the fifty-move rule
// are auto-claimed if eligible.
func (g *Game) PushUCI(uci string) error {
	if g.Outcome() != Ongoing {
		return errors.New("chess: game is over")
	}
	move, err := notchess.UCINotation{}.Decode(g.inner.Position(), uci)
	if err != nil {
		return fmt.Errorf("chess: decode %q: %w", uci, err)
	}
	if err := g.inner.Move(move); err != nil {
		return fmt.Errorf("chess: illegal move %q: %w", uci, err)
	}
	g.autoClaimDraws()
	return nil
}

// autoClaimDraws claims a draw if the game is currently eligible for one
// under the fifty-move rule or threefold repetition.
func (g *Game) autoClaimDraws() {
	if g.inner.Outcome() != notchess.NoOutcome {
		return
	}
	for _, m := range g.inner.EligibleDraws() {
		if m == notchess.FiftyMoveRule || m == notchess.ThreefoldRepetition {
			_ = g.inner.Draw(m)
			return
		}
	}
}

// FEN returns the current position as a FEN string.
func (g *Game) FEN() string {
	return g.inner.FEN()
}

// Outcome returns the current game result.
func (g *Game) Outcome() Outcome {
	switch g.inner.Outcome() {
	case notchess.NoOutcome:
		return Ongoing
	case notchess.WhiteWon:
		return WhiteWins
	case notchess.BlackWon:
		return BlackWins
	case notchess.Draw:
		return Drawn
	}
	return Ongoing
}

// Reason returns why the game ended (empty if ongoing). Reasons set by
// arbiter actions (resignation, time forfeit, adjudication, illegal move)
// override notnil/chess's auto-detected reason.
func (g *Game) Reason() Reason {
	if g.reason != "" {
		return g.reason
	}
	switch g.inner.Method() {
	case notchess.NoMethod:
		return ReasonInProgress
	case notchess.Checkmate:
		return ReasonCheckmate
	case notchess.Stalemate:
		return ReasonStalemate
	case notchess.InsufficientMaterial:
		return ReasonInsufficient
	case notchess.FiftyMoveRule:
		return ReasonFiftyMove
	case notchess.ThreefoldRepetition:
		return ReasonThreefold
	case notchess.Resignation:
		return ReasonResignation
	}
	return ReasonInProgress
}

// SideToMove returns "w" or "b".
func (g *Game) SideToMove() Side {
	if g.inner.Position().Turn() == notchess.White {
		return White
	}
	return Black
}

// HalfmoveClock returns plies since the last capture or pawn move.
func (g *Game) HalfmoveClock() int {
	return g.inner.Position().HalfMoveClock()
}

// MovesUCI returns the move history in UCI long algebraic notation.
func (g *Game) MovesUCI() []string {
	moves := g.inner.Moves()
	positions := g.inner.Positions()
	out := make([]string, 0, len(moves))
	enc := notchess.UCINotation{}
	for i, m := range moves {
		out = append(out, enc.Encode(positions[i], m))
	}
	return out
}

// MovesSAN returns the move history in standard algebraic notation.
func (g *Game) MovesSAN() []string {
	moves := g.inner.Moves()
	positions := g.inner.Positions()
	out := make([]string, 0, len(moves))
	enc := notchess.AlgebraicNotation{}
	for i, m := range moves {
		out = append(out, enc.Encode(positions[i], m))
	}
	return out
}

// Resign records a resignation by the given side. The opposing side wins.
func (g *Game) Resign(loser Side) {
	if g.Outcome() != Ongoing {
		return
	}
	if loser == White {
		g.inner.Resign(notchess.White)
	} else {
		g.inner.Resign(notchess.Black)
	}
	g.reason = ReasonResignation
}

// Adjudicate ends the game with the given outcome and a custom reason
// (intended for time forfeit, illegal move, or score-based adjudication).
func (g *Game) Adjudicate(outcome Outcome, reason Reason) error {
	if g.Outcome() != Ongoing {
		return errors.New("chess: game is over")
	}
	switch outcome {
	case WhiteWins:
		g.inner.Resign(notchess.Black)
	case BlackWins:
		g.inner.Resign(notchess.White)
	case Drawn:
		if err := g.inner.Draw(notchess.DrawOffer); err != nil {
			return fmt.Errorf("chess: adjudicate draw: %w", err)
		}
	default:
		return fmt.Errorf("chess: invalid adjudication outcome %q", outcome)
	}
	g.reason = reason
	return nil
}

// PGN returns the game encoded as PGN.
func (g *Game) PGN() string {
	return g.inner.String()
}
