package fen

import (
	"testing"
)

func TestParseStartingPosition(t *testing.T) {
	pos, err := Parse(StartingFEN)
	if err != nil {
		t.Fatalf("Parse(StartingFEN) error: %v", err)
	}

	// Check side to move
	if pos.SideToMove != White {
		t.Errorf("SideToMove = %v, want White", pos.SideToMove)
	}

	// Check castling
	if pos.Castling != AllCastling {
		t.Errorf("Castling = %v, want AllCastling", pos.Castling)
	}

	// Check en passant
	if pos.EnPassant != NoSquare {
		t.Errorf("EnPassant = %v, want NoSquare", pos.EnPassant)
	}

	// Check halfmove clock
	if pos.HalfmoveClock != 0 {
		t.Errorf("HalfmoveClock = %d, want 0", pos.HalfmoveClock)
	}

	// Check fullmove number
	if pos.FullmoveNum != 1 {
		t.Errorf("FullmoveNum = %d, want 1", pos.FullmoveNum)
	}

	// Check some specific pieces
	tests := []struct {
		sq   string
		want Piece
	}{
		{"a1", WhiteRook},
		{"b1", WhiteKnight},
		{"c1", WhiteBishop},
		{"d1", WhiteQueen},
		{"e1", WhiteKing},
		{"a2", WhitePawn},
		{"h2", WhitePawn},
		{"a8", BlackRook},
		{"e8", BlackKing},
		{"a7", BlackPawn},
		{"e4", NoPiece},
		{"d5", NoPiece},
	}

	for _, tc := range tests {
		sq := StringToSquare(tc.sq)
		got := pos.PieceAt(sq)
		if got != tc.want {
			t.Errorf("PieceAt(%s) = %v, want %v", tc.sq, got, tc.want)
		}
	}
}

func TestParseRoundTrip(t *testing.T) {
	fens := []string{
		StartingFEN,
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		"rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq c6 0 2",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"r2q1rk1/pP1p2pp/Q4n2/bbp1p3/Np6/1B3NBn/pPPP1PPP/R3K2R b KQ - 0 1",
		"rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8",
		"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
	}

	for _, fen := range fens {
		pos, err := Parse(fen)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", fen, err)
			continue
		}

		got := pos.String()
		if got != fen {
			t.Errorf("Round trip failed:\n  input:  %q\n  output: %q", fen, got)
		}
	}
}

func TestParseInvalidFEN(t *testing.T) {
	tests := []struct {
		name string
		fen  string
	}{
		{"empty string", ""},
		{"too few fields", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w"},
		{"too few ranks", "rnbqkbnr/pppppppp/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"too many ranks", "rnbqkbnr/pppppppp/8/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"invalid piece", "rnbqkbnr/pppppppp/8/8/8/8/PPPPXPPP/RNBQKBNR w KQkq - 0 1"},
		{"rank too long", "rnbqkbnrr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"rank too short", "rnbqkbn/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"invalid side to move", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1"},
		{"invalid castling", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w XYZq - 0 1"},
		{"invalid en passant file", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq z3 0 1"},
		{"invalid en passant rank", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq a9 0 1"},
		{"en passant wrong rank", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e4 0 1"},
		{"negative halfmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - -1 1"},
		{"zero fullmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 0"},
		{"negative fullmove", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 -1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.fen)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", tc.fen)
			}
		})
	}
}

func TestParseCastling(t *testing.T) {
	tests := []struct {
		fen      string
		castling CastlingRights
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", AllCastling},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w K - 0 1", WhiteKingSide},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w Q - 0 1", WhiteQueenSide},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w k - 0 1", BlackKingSide},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w q - 0 1", BlackQueenSide},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w Kq - 0 1", WhiteKingSide | BlackQueenSide},
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1", NoCastling},
	}

	for _, tc := range tests {
		pos, err := Parse(tc.fen)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.fen, err)
			continue
		}
		if pos.Castling != tc.castling {
			t.Errorf("Parse(%q).Castling = %v, want %v", tc.fen, pos.Castling, tc.castling)
		}
	}
}

func TestParseEnPassant(t *testing.T) {
	tests := []struct {
		fen   string
		epSq  Square
		epStr string
	}{
		{"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1", 20, "e3"}, // e3 = rank 2, file 4
		{"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 1", 44, "e6"}, // e6 = rank 5, file 4
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", NoSquare, "-"},
	}

	for _, tc := range tests {
		pos, err := Parse(tc.fen)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.fen, err)
			continue
		}
		if pos.EnPassant != tc.epSq {
			t.Errorf("Parse(%q).EnPassant = %d (%s), want %d (%s)",
				tc.fen, pos.EnPassant, SquareToString(pos.EnPassant), tc.epSq, tc.epStr)
		}
	}
}

func TestSquareConversion(t *testing.T) {
	tests := []struct {
		sq  Square
		str string
	}{
		{0, "a1"},
		{7, "h1"},
		{8, "a2"},
		{63, "h8"},
		{27, "d4"},
		{36, "e5"},
		{NoSquare, "-"},
	}

	for _, tc := range tests {
		// Test SquareToString
		got := SquareToString(tc.sq)
		if got != tc.str {
			t.Errorf("SquareToString(%d) = %q, want %q", tc.sq, got, tc.str)
		}

		// Test StringToSquare (skip NoSquare case for reverse)
		if tc.sq != NoSquare {
			gotSq := StringToSquare(tc.str)
			if gotSq != tc.sq {
				t.Errorf("StringToSquare(%q) = %d, want %d", tc.str, gotSq, tc.sq)
			}
		}
	}
}

func TestStartingPosition(t *testing.T) {
	pos := StartingPosition()
	if pos == nil {
		t.Fatal("StartingPosition() returned nil")
	}

	expected := StartingFEN
	got := pos.String()
	if got != expected {
		t.Errorf("StartingPosition().String() = %q, want %q", got, expected)
	}
}

func TestPieceColors(t *testing.T) {
	whitePieces := []Piece{WhitePawn, WhiteKnight, WhiteBishop, WhiteRook, WhiteQueen, WhiteKing}
	blackPieces := []Piece{BlackPawn, BlackKnight, BlackBishop, BlackRook, BlackQueen, BlackKing}

	for _, p := range whitePieces {
		if !IsWhitePiece(p) {
			t.Errorf("IsWhitePiece(%v) = false, want true", p)
		}
		if IsBlackPiece(p) {
			t.Errorf("IsBlackPiece(%v) = true, want false", p)
		}
		if PieceColor(p) != White {
			t.Errorf("PieceColor(%v) = %v, want White", p, PieceColor(p))
		}
	}

	for _, p := range blackPieces {
		if IsWhitePiece(p) {
			t.Errorf("IsWhitePiece(%v) = true, want false", p)
		}
		if !IsBlackPiece(p) {
			t.Errorf("IsBlackPiece(%v) = false, want true", p)
		}
		if PieceColor(p) != Black {
			t.Errorf("PieceColor(%v) = %v, want Black", p, PieceColor(p))
		}
	}
}

func TestMinimalFEN(t *testing.T) {
	// FEN with only 4 required fields (no halfmove/fullmove)
	fen := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3"
	pos, err := Parse(fen)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", fen, err)
	}

	// Should use defaults for missing fields
	if pos.HalfmoveClock != 0 {
		t.Errorf("HalfmoveClock = %d, want 0 (default)", pos.HalfmoveClock)
	}
	if pos.FullmoveNum != 1 {
		t.Errorf("FullmoveNum = %d, want 1 (default)", pos.FullmoveNum)
	}
}
