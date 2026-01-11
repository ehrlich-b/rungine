// Package fen provides FEN (Forsyth-Edwards Notation) parsing and generation.
package fen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Standard starting position FEN.
const StartingFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// Piece represents a chess piece.
type Piece byte

const (
	NoPiece Piece = iota
	WhitePawn
	WhiteKnight
	WhiteBishop
	WhiteRook
	WhiteQueen
	WhiteKing
	BlackPawn
	BlackKnight
	BlackBishop
	BlackRook
	BlackQueen
	BlackKing
)

// Color represents a side to move.
type Color byte

const (
	White Color = iota
	Black
)

// CastlingRights represents available castling options.
type CastlingRights byte

const (
	NoCastling CastlingRights = 0
	WhiteKingSide CastlingRights = 1 << iota
	WhiteQueenSide
	BlackKingSide
	BlackQueenSide
	AllCastling = WhiteKingSide | WhiteQueenSide | BlackKingSide | BlackQueenSide
)

// Square represents a board square (0-63).
type Square int8

const NoSquare Square = -1

// Position represents a chess position.
type Position struct {
	Board         [64]Piece      // Board[0] = a1, Board[63] = h8
	SideToMove    Color
	Castling      CastlingRights
	EnPassant     Square         // En passant target square, or NoSquare
	HalfmoveClock int            // Halfmove clock for 50-move rule
	FullmoveNum   int            // Fullmove number
}

var (
	ErrInvalidFEN          = errors.New("invalid FEN string")
	ErrInvalidPiecePlacement = errors.New("invalid piece placement")
	ErrInvalidSideToMove   = errors.New("invalid side to move")
	ErrInvalidCastling     = errors.New("invalid castling rights")
	ErrInvalidEnPassant    = errors.New("invalid en passant square")
	ErrInvalidHalfmove     = errors.New("invalid halfmove clock")
	ErrInvalidFullmove     = errors.New("invalid fullmove number")
)

// pieceFromChar maps FEN piece characters to Piece values.
var pieceFromChar = map[byte]Piece{
	'P': WhitePawn, 'N': WhiteKnight, 'B': WhiteBishop,
	'R': WhiteRook, 'Q': WhiteQueen, 'K': WhiteKing,
	'p': BlackPawn, 'n': BlackKnight, 'b': BlackBishop,
	'r': BlackRook, 'q': BlackQueen, 'k': BlackKing,
}

// charFromPiece maps Piece values to FEN characters.
var charFromPiece = map[Piece]byte{
	WhitePawn: 'P', WhiteKnight: 'N', WhiteBishop: 'B',
	WhiteRook: 'R', WhiteQueen: 'Q', WhiteKing: 'K',
	BlackPawn: 'p', BlackKnight: 'n', BlackBishop: 'b',
	BlackRook: 'r', BlackQueen: 'q', BlackKing: 'k',
}

// Parse parses a FEN string into a Position.
func Parse(fen string) (*Position, error) {
	fields := strings.Fields(fen)
	if len(fields) < 4 {
		return nil, fmt.Errorf("%w: expected at least 4 fields, got %d", ErrInvalidFEN, len(fields))
	}

	pos := &Position{
		EnPassant:     NoSquare,
		HalfmoveClock: 0,
		FullmoveNum:   1,
	}

	// Parse piece placement
	if err := parsePiecePlacement(fields[0], pos); err != nil {
		return nil, err
	}

	// Parse side to move
	switch fields[1] {
	case "w":
		pos.SideToMove = White
	case "b":
		pos.SideToMove = Black
	default:
		return nil, fmt.Errorf("%w: got %q", ErrInvalidSideToMove, fields[1])
	}

	// Parse castling rights
	if err := parseCastling(fields[2], pos); err != nil {
		return nil, err
	}

	// Parse en passant square
	if err := parseEnPassant(fields[3], pos); err != nil {
		return nil, err
	}

	// Parse halfmove clock (optional)
	if len(fields) >= 5 {
		hm, err := strconv.Atoi(fields[4])
		if err != nil || hm < 0 {
			return nil, fmt.Errorf("%w: got %q", ErrInvalidHalfmove, fields[4])
		}
		pos.HalfmoveClock = hm
	}

	// Parse fullmove number (optional)
	if len(fields) >= 6 {
		fm, err := strconv.Atoi(fields[5])
		if err != nil || fm < 1 {
			return nil, fmt.Errorf("%w: got %q", ErrInvalidFullmove, fields[5])
		}
		pos.FullmoveNum = fm
	}

	return pos, nil
}

// parsePiecePlacement parses the piece placement field (first field of FEN).
func parsePiecePlacement(s string, pos *Position) error {
	ranks := strings.Split(s, "/")
	if len(ranks) != 8 {
		return fmt.Errorf("%w: expected 8 ranks, got %d", ErrInvalidPiecePlacement, len(ranks))
	}

	for rankIdx, rank := range ranks {
		file := 0
		boardRank := 7 - rankIdx // FEN is top-to-bottom, board[0] is a1

		for i := 0; i < len(rank); i++ {
			c := rank[i]
			if c >= '1' && c <= '8' {
				// Skip empty squares
				skip := int(c - '0')
				file += skip
			} else if piece, ok := pieceFromChar[c]; ok {
				if file >= 8 {
					return fmt.Errorf("%w: too many pieces on rank %d", ErrInvalidPiecePlacement, 8-rankIdx)
				}
				pos.Board[boardRank*8+file] = piece
				file++
			} else {
				return fmt.Errorf("%w: unknown piece %q", ErrInvalidPiecePlacement, string(c))
			}
		}

		if file != 8 {
			return fmt.Errorf("%w: rank %d has %d files", ErrInvalidPiecePlacement, 8-rankIdx, file)
		}
	}

	return nil
}

// parseCastling parses the castling availability field.
func parseCastling(s string, pos *Position) error {
	if s == "-" {
		pos.Castling = NoCastling
		return nil
	}

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'K':
			pos.Castling |= WhiteKingSide
		case 'Q':
			pos.Castling |= WhiteQueenSide
		case 'k':
			pos.Castling |= BlackKingSide
		case 'q':
			pos.Castling |= BlackQueenSide
		default:
			return fmt.Errorf("%w: unknown castling flag %q", ErrInvalidCastling, string(s[i]))
		}
	}

	return nil
}

// parseEnPassant parses the en passant target square.
func parseEnPassant(s string, pos *Position) error {
	if s == "-" {
		pos.EnPassant = NoSquare
		return nil
	}

	if len(s) != 2 {
		return fmt.Errorf("%w: got %q", ErrInvalidEnPassant, s)
	}

	file := s[0] - 'a'
	rank := s[1] - '1'

	if file > 7 || rank > 7 {
		return fmt.Errorf("%w: got %q", ErrInvalidEnPassant, s)
	}

	// En passant square must be on rank 3 or 6
	if rank != 2 && rank != 5 {
		return fmt.Errorf("%w: en passant must be on rank 3 or 6, got %q", ErrInvalidEnPassant, s)
	}

	pos.EnPassant = Square(int(rank)*8 + int(file))
	return nil
}

// String returns the FEN representation of the position.
func (p *Position) String() string {
	var sb strings.Builder

	// Piece placement
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			piece := p.Board[rank*8+file]
			if piece == NoPiece {
				empty++
			} else {
				if empty > 0 {
					sb.WriteByte('0' + byte(empty))
					empty = 0
				}
				sb.WriteByte(charFromPiece[piece])
			}
		}
		if empty > 0 {
			sb.WriteByte('0' + byte(empty))
		}
		if rank > 0 {
			sb.WriteByte('/')
		}
	}

	// Side to move
	sb.WriteByte(' ')
	if p.SideToMove == White {
		sb.WriteByte('w')
	} else {
		sb.WriteByte('b')
	}

	// Castling
	sb.WriteByte(' ')
	if p.Castling == NoCastling {
		sb.WriteByte('-')
	} else {
		if p.Castling&WhiteKingSide != 0 {
			sb.WriteByte('K')
		}
		if p.Castling&WhiteQueenSide != 0 {
			sb.WriteByte('Q')
		}
		if p.Castling&BlackKingSide != 0 {
			sb.WriteByte('k')
		}
		if p.Castling&BlackQueenSide != 0 {
			sb.WriteByte('q')
		}
	}

	// En passant
	sb.WriteByte(' ')
	if p.EnPassant == NoSquare {
		sb.WriteByte('-')
	} else {
		file := p.EnPassant % 8
		rank := p.EnPassant / 8
		sb.WriteByte('a' + byte(file))
		sb.WriteByte('1' + byte(rank))
	}

	// Halfmove clock and fullmove number
	sb.WriteString(fmt.Sprintf(" %d %d", p.HalfmoveClock, p.FullmoveNum))

	return sb.String()
}

// StartingPosition returns the standard starting position.
func StartingPosition() *Position {
	pos, _ := Parse(StartingFEN)
	return pos
}

// SquareToString converts a square index to algebraic notation.
func SquareToString(sq Square) string {
	if sq == NoSquare {
		return "-"
	}
	file := sq % 8
	rank := sq / 8
	return string([]byte{'a' + byte(file), '1' + byte(rank)})
}

// StringToSquare converts algebraic notation to a square index.
func StringToSquare(s string) Square {
	if len(s) != 2 {
		return NoSquare
	}
	file := s[0] - 'a'
	rank := s[1] - '1'
	if file > 7 || rank > 7 {
		return NoSquare
	}
	return Square(int(rank)*8 + int(file))
}

// PieceAt returns the piece at the given square.
func (p *Position) PieceAt(sq Square) Piece {
	if sq < 0 || sq > 63 {
		return NoPiece
	}
	return p.Board[sq]
}

// IsWhitePiece returns true if the piece is white.
func IsWhitePiece(piece Piece) bool {
	return piece >= WhitePawn && piece <= WhiteKing
}

// IsBlackPiece returns true if the piece is black.
func IsBlackPiece(piece Piece) bool {
	return piece >= BlackPawn && piece <= BlackKing
}

// PieceColor returns the color of a piece.
func PieceColor(piece Piece) Color {
	if IsWhitePiece(piece) {
		return White
	}
	return Black
}
