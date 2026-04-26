package chess

import (
	"strings"
	"testing"
)

func TestNewGameStartsInProgress(t *testing.T) {
	g := NewGame()
	if g.Outcome() != Ongoing {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Ongoing)
	}
	if g.Reason() != ReasonInProgress {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonInProgress)
	}
	if g.SideToMove() != White {
		t.Errorf("SideToMove() = %q, want %q", g.SideToMove(), White)
	}
}

func TestPushUCIBasic(t *testing.T) {
	g := NewGame()
	if err := g.PushUCI("e2e4"); err != nil {
		t.Fatalf("PushUCI(e2e4): %v", err)
	}
	if g.SideToMove() != Black {
		t.Errorf("SideToMove() = %q, want %q", g.SideToMove(), Black)
	}
	moves := g.MovesUCI()
	if len(moves) != 1 || moves[0] != "e2e4" {
		t.Errorf("MovesUCI() = %v, want [e2e4]", moves)
	}
}

func TestPushSANBasic(t *testing.T) {
	g := NewGame()
	moves := []string{"e4", "e5", "Nf3", "Nc6"}
	for _, san := range moves {
		if err := g.PushSAN(san); err != nil {
			t.Fatalf("PushSAN(%q): %v", san, err)
		}
	}
	uci := g.MovesUCI()
	wantUCI := []string{"e2e4", "e7e5", "g1f3", "b8c6"}
	if len(uci) != len(wantUCI) {
		t.Fatalf("MovesUCI len = %d, want %d", len(uci), len(wantUCI))
	}
	for i, m := range uci {
		if m != wantUCI[i] {
			t.Errorf("MovesUCI[%d] = %q, want %q", i, m, wantUCI[i])
		}
	}
}

func TestPushSANIllegal(t *testing.T) {
	g := NewGame()
	if err := g.PushSAN("Nf6"); err == nil {
		t.Error("PushSAN(Nf6) on starting position should fail (white to move)")
	}
}

func TestPushUCIIllegalMove(t *testing.T) {
	g := NewGame()
	err := g.PushUCI("e2e5")
	if err == nil {
		t.Fatal("PushUCI(e2e5) on starting position should fail")
	}
	if !strings.Contains(err.Error(), "illegal") && !strings.Contains(err.Error(), "decode") {
		t.Errorf("error = %q, want illegal/decode", err)
	}
}

func TestPushUCIMalformed(t *testing.T) {
	g := NewGame()
	if err := g.PushUCI("nonsense"); err == nil {
		t.Error("PushUCI(nonsense) should fail")
	}
}

func TestFoolsMate(t *testing.T) {
	// Fool's mate: 1. f3 e5 2. g4 Qh4#
	g := NewGame()
	moves := []string{"f2f3", "e7e5", "g2g4", "d8h4"}
	for _, m := range moves {
		if err := g.PushUCI(m); err != nil {
			t.Fatalf("PushUCI(%q): %v", m, err)
		}
	}
	if g.Outcome() != BlackWins {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), BlackWins)
	}
	if g.Reason() != ReasonCheckmate {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonCheckmate)
	}
}

func TestStalemate(t *testing.T) {
	// Stalemate position: black king on a8, white queen on b6, white king on c6, black to move
	fen := "k7/8/1QK5/8/8/8/8/8 b - - 0 1"
	g, err := FromFEN(fen)
	if err != nil {
		t.Fatalf("FromFEN: %v", err)
	}
	if g.Outcome() != Drawn {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Drawn)
	}
	if g.Reason() != ReasonStalemate {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonStalemate)
	}
}

func TestInsufficientMaterial(t *testing.T) {
	// King vs king is an automatic draw.
	fen := "8/8/4k3/8/8/4K3/8/8 w - - 0 1"
	g, err := FromFEN(fen)
	if err != nil {
		t.Fatalf("FromFEN: %v", err)
	}
	if g.Outcome() != Drawn {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Drawn)
	}
	if g.Reason() != ReasonInsufficient {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonInsufficient)
	}
}

func TestThreefoldRepetition(t *testing.T) {
	// Repeat the starting position three times by shuffling knights.
	g := NewGame()
	cycle := []string{"g1f3", "g8f6", "f3g1", "f6g8"}
	for range 2 {
		for _, m := range cycle {
			if err := g.PushUCI(m); err != nil {
				t.Fatalf("PushUCI(%q): %v", m, err)
			}
		}
	}
	// After two cycles, the starting position has occurred three times.
	if g.Outcome() != Drawn {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Drawn)
	}
	if g.Reason() != ReasonThreefold {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonThreefold)
	}
}

func TestFiftyMoveRule(t *testing.T) {
	// Position with halfmove clock at 99: one quiet move triggers the 50-move rule.
	fen := "8/8/4k3/8/8/4K3/4P3/8 w - - 99 60"
	g, err := FromFEN(fen)
	if err != nil {
		t.Fatalf("FromFEN: %v", err)
	}
	if err := g.PushUCI("e3d3"); err != nil {
		t.Fatalf("PushUCI(e3d3): %v", err)
	}
	if g.Outcome() != Drawn {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Drawn)
	}
	if g.Reason() != ReasonFiftyMove {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonFiftyMove)
	}
}

func TestResign(t *testing.T) {
	g := NewGame()
	g.PushUCI("e2e4")
	g.Resign(White)
	if g.Outcome() != BlackWins {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), BlackWins)
	}
	if g.Reason() != ReasonResignation {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonResignation)
	}
}

func TestAdjudicateTimeForfeit(t *testing.T) {
	g := NewGame()
	g.PushUCI("e2e4")
	if err := g.Adjudicate(BlackWins, ReasonTimeForfeit); err != nil {
		t.Fatalf("Adjudicate: %v", err)
	}
	if g.Outcome() != BlackWins {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), BlackWins)
	}
	if g.Reason() != ReasonTimeForfeit {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonTimeForfeit)
	}
}

func TestAdjudicateDraw(t *testing.T) {
	g := NewGame()
	g.PushUCI("e2e4")
	if err := g.Adjudicate(Drawn, ReasonAdjudication); err != nil {
		t.Fatalf("Adjudicate: %v", err)
	}
	if g.Outcome() != Drawn {
		t.Errorf("Outcome() = %q, want %q", g.Outcome(), Drawn)
	}
	if g.Reason() != ReasonAdjudication {
		t.Errorf("Reason() = %q, want %q", g.Reason(), ReasonAdjudication)
	}
}

func TestAdjudicateInvalidOutcome(t *testing.T) {
	g := NewGame()
	if err := g.Adjudicate(Ongoing, ReasonAdjudication); err == nil {
		t.Error("Adjudicate(Ongoing) should fail")
	}
}

func TestPushUCIAfterGameOver(t *testing.T) {
	g := NewGame()
	g.PushUCI("f2f3")
	g.PushUCI("e7e5")
	g.PushUCI("g2g4")
	g.PushUCI("d8h4")
	if err := g.PushUCI("e1e2"); err == nil {
		t.Error("PushUCI after checkmate should fail")
	}
}

func TestFENRoundtrip(t *testing.T) {
	tests := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
		"8/8/4k3/8/8/4K3/8/8 w - - 0 1",
	}
	for _, fen := range tests {
		t.Run(fen, func(t *testing.T) {
			g, err := FromFEN(fen)
			if err != nil {
				t.Fatalf("FromFEN(%q): %v", fen, err)
			}
			if got := g.FEN(); got != fen {
				t.Errorf("FEN() = %q, want %q", got, fen)
			}
		})
	}
}

func TestFromFENInvalid(t *testing.T) {
	if _, err := FromFEN("not a fen"); err == nil {
		t.Error("FromFEN(invalid) should fail")
	}
}

func TestPromotion(t *testing.T) {
	// White pawn on a7 ready to promote.
	fen := "4k3/P7/8/8/8/8/8/4K3 w - - 0 1"
	g, err := FromFEN(fen)
	if err != nil {
		t.Fatalf("FromFEN: %v", err)
	}
	if err := g.PushUCI("a7a8q"); err != nil {
		t.Fatalf("PushUCI(a7a8q): %v", err)
	}
	moves := g.MovesUCI()
	if len(moves) != 1 || moves[0] != "a7a8q" {
		t.Errorf("MovesUCI() = %v, want [a7a8q]", moves)
	}
}

func TestCastling(t *testing.T) {
	// Set up a position where white can castle kingside.
	fen := "rnbqkbnr/pppppppp/8/8/8/5NP1/PPPPPPBP/RNBQK2R w KQkq - 0 1"
	g, err := FromFEN(fen)
	if err != nil {
		t.Fatalf("FromFEN: %v", err)
	}
	if err := g.PushUCI("e1g1"); err != nil {
		t.Fatalf("PushUCI(e1g1): %v", err)
	}
	if !strings.Contains(g.FEN(), "RNBQ1RK1") {
		t.Errorf("after castling, FEN = %q, want rook+king on f1/g1", g.FEN())
	}
}

func TestSANEncoding(t *testing.T) {
	g := NewGame()
	g.PushUCI("e2e4")
	g.PushUCI("e7e5")
	g.PushUCI("g1f3")
	san := g.MovesSAN()
	want := []string{"e4", "e5", "Nf3"}
	if len(san) != len(want) {
		t.Fatalf("MovesSAN() = %v, want %v", san, want)
	}
	for i, m := range want {
		if san[i] != m {
			t.Errorf("MovesSAN()[%d] = %q, want %q", i, san[i], m)
		}
	}
}

func TestPGN(t *testing.T) {
	g := NewGame()
	g.PushUCI("e2e4")
	g.PushUCI("e7e5")
	pgn := g.PGN()
	if !strings.Contains(pgn, "e4") || !strings.Contains(pgn, "e5") {
		t.Errorf("PGN() = %q, missing expected moves", pgn)
	}
}
