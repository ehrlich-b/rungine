package chess

import (
	"fmt"
	"strings"

	notchess "github.com/notnil/chess"
)

// UCIToSAN converts a sequence of UCI moves, played from the position given
// by fen, into Standard Algebraic Notation. It returns one SAN token per
// input move, in order. The starting FEN must describe a legal position and
// every move must be legal in turn, otherwise an error is returned.
//
// Full SAN rules are applied: piece disambiguation (origin file, rank, or
// both, only when required), captures ('x'), castling (O-O / O-O-O),
// promotion (=Q, with capture), en passant, and the check ('+') / mate ('#')
// suffix.
func UCIToSAN(fen string, moves []string) ([]string, error) {
	g, err := FromFEN(fen)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(moves))
	for _, uci := range moves {
		pos := g.inner.Position()
		m, err := notchess.UCINotation{}.Decode(pos, uci)
		if err != nil {
			return nil, fmt.Errorf("chess: decode %q: %w", uci, err)
		}
		if err := g.inner.Move(m); err != nil {
			return nil, fmt.Errorf("chess: illegal move %q: %w", uci, err)
		}
		applied := g.inner.Moves()[len(g.inner.Moves())-1]
		out = append(out, encodeSAN(pos, g.inner.Position(), applied))
	}
	return out, nil
}

// encodeSAN renders move m (played from pos, resulting in after) in SAN. It
// relies on the tags the engine attaches to a move when it is applied:
// castling, capture, en passant, and check.
func encodeSAN(pos, after *notchess.Position, m *notchess.Move) string {
	suffix := ""
	if m.HasTag(notchess.Check) {
		if after.Status() == notchess.Checkmate {
			suffix = "#"
		} else {
			suffix = "+"
		}
	}
	if m.HasTag(notchess.KingSideCastle) {
		return "O-O" + suffix
	}
	if m.HasTag(notchess.QueenSideCastle) {
		return "O-O-O" + suffix
	}
	piece := pos.Board().Piece(m.S1())
	if piece.Type() == notchess.Pawn {
		var sb strings.Builder
		if m.HasTag(notchess.Capture) || m.HasTag(notchess.EnPassant) {
			sb.WriteByte('a' + byte(m.S1().File()))
			sb.WriteByte('x')
		}
		sb.WriteString(m.S2().String())
		if m.Promo() != notchess.NoPieceType {
			sb.WriteByte('=')
			sb.WriteString(sanLetter(m.Promo()))
		}
		sb.WriteString(suffix)
		return sb.String()
	}
	var sb strings.Builder
	sb.WriteString(sanLetter(piece.Type()))
	sb.WriteString(disambiguate(pos, m))
	if m.HasTag(notchess.Capture) {
		sb.WriteByte('x')
	}
	sb.WriteString(m.S2().String())
	sb.WriteString(suffix)
	return sb.String()
}

// disambiguate returns the minimal origin prefix SAN requires to tell move
// m apart from other movable pieces of the same type that can also reach
// m's destination: the file when possible, the rank when a same-file piece
// is the competitor, and both when needed.
func disambiguate(pos *notchess.Position, m *notchess.Move) string {
	piece := pos.Board().Piece(m.S1())
	var req, fileReq, rankReq bool
	for _, mv := range pos.ValidMoves() {
		if mv.S1() != m.S1() && mv.S2() == m.S2() && pos.Board().Piece(mv.S1()) == piece {
			req = true
			if mv.S1().File() == m.S1().File() {
				rankReq = true
			}
			if mv.S1().Rank() == m.S1().Rank() {
				fileReq = true
			}
		}
	}
	var sb strings.Builder
	if fileReq || (!rankReq && req) {
		sb.WriteByte('a' + byte(m.S1().File()))
	}
	if rankReq {
		sb.WriteByte('1' + byte(m.S1().Rank()))
	}
	return sb.String()
}

// sanLetter returns the SAN letter for a piece type ("" for a pawn).
func sanLetter(pt notchess.PieceType) string {
	switch pt {
	case notchess.King:
		return "K"
	case notchess.Queen:
		return "Q"
	case notchess.Rook:
		return "R"
	case notchess.Bishop:
		return "B"
	case notchess.Knight:
		return "N"
	}
	return ""
}
