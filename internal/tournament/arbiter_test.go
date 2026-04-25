package tournament

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"rungine/internal/chess"
	"rungine/internal/uci"
)

// scripted is a mock UCI engine that returns moves from a pre-computed
// queue. It supports failure injection (errors, crashes, slow responses)
// and records every SetPosition call for assertions.
type scripted struct {
	moves []string
	// infos[i] is the slice of info lines emitted before the i-th move's
	// bestmove. Out-of-range indices emit nothing.
	infos [][]uci.AnalysisInfo

	// Failure injection.
	delay     time.Duration // sleep before responding to Go()
	goErr     error         // returned by Go()
	setPosErr error         // returned by SetPosition()
	crashOnGo bool          // close BestMoveChannel instead of responding

	mu        sync.Mutex
	idx       int
	closed    bool
	positions [][]string // moves arg from each SetPosition call
	fenSeen   []string   // fen arg from each SetPosition call

	bestMoveCh chan uci.BestMove
	infoCh     chan uci.AnalysisInfo
}

func newScripted(moves []string) *scripted {
	return &scripted{
		moves:      moves,
		bestMoveCh: make(chan uci.BestMove, 4),
		infoCh:     make(chan uci.AnalysisInfo, 64),
	}
}

// scriptInfos sets per-move analysis info (one slice per move).
func (m *scripted) scriptInfos(infos [][]uci.AnalysisInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.infos = infos
}

func (m *scripted) SetPosition(fen string, moves []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fenSeen = append(m.fenSeen, fen)
	m.positions = append(m.positions, slices.Clone(moves))
	return m.setPosErr
}

func (m *scripted) Go(params uci.GoParams) error {
	m.mu.Lock()
	if m.goErr != nil {
		err := m.goErr
		m.mu.Unlock()
		return err
	}
	if m.crashOnGo {
		if !m.closed {
			close(m.bestMoveCh)
			m.closed = true
		}
		m.mu.Unlock()
		return nil
	}
	delay := m.delay
	m.mu.Unlock()

	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.closed {
			return
		}
		// Emit any queued info lines for this move before bestmove.
		if m.idx < len(m.infos) {
			for _, info := range m.infos[m.idx] {
				select {
				case m.infoCh <- info:
				default:
				}
			}
		}
		var move string
		if m.idx < len(m.moves) {
			move = m.moves[m.idx]
			m.idx++
		} else {
			move = "0000" // exhausted script — null move triggers forfeit
		}
		m.bestMoveCh <- uci.BestMove{Move: move}
	}()
	return nil
}

func (m *scripted) StopSearch() error { return nil }

func (m *scripted) BestMoveChannel() <-chan uci.BestMove { return m.bestMoveCh }

func (m *scripted) InfoChannel() <-chan uci.AnalysisInfo { return m.infoCh }

func TestArbiterFoolsMate(t *testing.T) {
	// 1. f3 e5 2. g4 Qh4#
	white := newScripted([]string{"f2f3", "g2g4"})
	black := newScripted([]string{"e7e5", "d8h4"})

	arb, err := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want %q", res.Outcome, chess.BlackWins)
	}
	if res.Reason != chess.ReasonCheckmate {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonCheckmate)
	}
	if res.PlyCount != 4 {
		t.Errorf("PlyCount = %d, want 4", res.PlyCount)
	}
	// Natural termination shouldn't set Loser.
	if res.Loser != "" {
		t.Errorf("Loser = %q, want empty (natural termination)", res.Loser)
	}
}

func TestArbiterIllegalMoveForfeit(t *testing.T) {
	white := newScripted([]string{"e2e5"}) // illegal — pawn jumps to wrong square
	black := newScripted(nil)

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want BlackWins", res.Outcome)
	}
	if res.Reason != chess.ReasonIllegalMove {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonIllegalMove)
	}
	if res.Loser != "W" {
		t.Errorf("Loser = %q, want W", res.Loser)
	}
}

func TestArbiterEmptyBestMoveForfeit(t *testing.T) {
	// White has no scripted moves — mock returns the null move "0000".
	white := newScripted(nil)
	black := newScripted(nil)

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	res, _ := arb.Run(context.Background())
	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want BlackWins", res.Outcome)
	}
	if res.Reason != chess.ReasonIllegalMove {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonIllegalMove)
	}
}

func TestArbiterEngineCrashForfeit(t *testing.T) {
	white := newScripted(nil)
	white.crashOnGo = true
	black := newScripted(nil)

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	res, _ := arb.Run(context.Background())
	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want BlackWins (white crashed)", res.Outcome)
	}
	if res.Loser != "W" {
		t.Errorf("Loser = %q, want W", res.Loser)
	}
	if !errors.Is(res.Cause, errEngineCrashed) {
		t.Errorf("Cause = %v, want errEngineCrashed", res.Cause)
	}
}

func TestArbiterGoErrorForfeit(t *testing.T) {
	white := newScripted(nil)
	white.goErr = errors.New("pipe closed")
	black := newScripted(nil)

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	res, _ := arb.Run(context.Background())
	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want BlackWins", res.Outcome)
	}
	if res.Loser != "W" {
		t.Errorf("Loser = %q, want W", res.Loser)
	}
}

func TestArbiterTimeForfeit(t *testing.T) {
	// Black is given a 50ms clock but its mock waits 500ms before
	// responding — the move-deadline timer fires first.
	white := newScripted([]string{"e2e4"})
	black := newScripted([]string{"e7e5"})
	black.delay = 500 * time.Millisecond

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{Initial: 50 * time.Millisecond},
		MoveGrace:   10 * time.Millisecond,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != chess.WhiteWins {
		t.Errorf("Outcome = %q, want WhiteWins", res.Outcome)
	}
	if res.Reason != chess.ReasonTimeForfeit {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonTimeForfeit)
	}
	if res.Loser != "B" {
		t.Errorf("Loser = %q, want B", res.Loser)
	}
}

func TestArbiterThreefoldRepetition(t *testing.T) {
	// Knight shuffle returns to startpos on plies 4 and 8 — three
	// occurrences of the starting position.
	white := newScripted([]string{"g1f3", "f3g1", "g1f3", "f3g1"})
	black := newScripted([]string{"g8f6", "f6g8", "g8f6", "f6g8"})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != chess.Drawn {
		t.Errorf("Outcome = %q, want Drawn", res.Outcome)
	}
	if res.Reason != chess.ReasonThreefold {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonThreefold)
	}
	if res.PlyCount != 8 {
		t.Errorf("PlyCount = %d, want 8", res.PlyCount)
	}
}

func TestArbiterMaxPliesDraw(t *testing.T) {
	white := newScripted([]string{"e2e4", "g1f3"})
	black := newScripted([]string{"e7e5", "g8f6"})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Outcome != chess.Drawn {
		t.Errorf("Outcome = %q, want Drawn", res.Outcome)
	}
	if res.PlyCount != 4 {
		t.Errorf("PlyCount = %d, want 4", res.PlyCount)
	}
}

func TestArbiterContextCancellation(t *testing.T) {
	white := newScripted([]string{"e2e4"})
	white.delay = 5 * time.Second
	black := newScripted(nil)

	ctx, cancel := context.WithCancel(context.Background())
	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := arb.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v, want context.Canceled", err)
	}
}

func TestArbiterBroadcastsAccumulatedMoves(t *testing.T) {
	white := newScripted([]string{"e2e4"})
	black := newScripted([]string{"e7e5"})

	arb, _ := New(Config{
		White: white, Black: black,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
	})
	if _, err := arb.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Both engines see: initial empty position, then [e2e4], then [e2e4 e7e5].
	for _, eng := range []*scripted{white, black} {
		if len(eng.positions) < 3 {
			t.Fatalf("SetPosition called %d times, want >= 3", len(eng.positions))
		}
		if len(eng.positions[0]) != 0 {
			t.Errorf("first broadcast moves = %v, want empty", eng.positions[0])
		}
		if got := eng.positions[1]; len(got) != 1 || got[0] != "e2e4" {
			t.Errorf("second broadcast moves = %v, want [e2e4]", got)
		}
		if got := eng.positions[2]; len(got) != 2 || got[0] != "e2e4" || got[1] != "e7e5" {
			t.Errorf("third broadcast moves = %v, want [e2e4 e7e5]", got)
		}
	}
}

func TestArbiterStartFENPropagated(t *testing.T) {
	// Ruy Lopez position with white to move. White plays Bxc6.
	startFEN := "r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 3 3"
	white := newScripted([]string{"b5c6"})
	black := newScripted([]string{"d7c6"})

	arb, err := New(Config{
		White: white, Black: black,
		StartFEN:    startFEN,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := arb.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(white.fenSeen) == 0 || white.fenSeen[0] != startFEN {
		t.Errorf("white first FEN = %q, want %q", white.fenSeen, startFEN)
	}
	if len(black.fenSeen) == 0 || black.fenSeen[0] != startFEN {
		t.Errorf("black first FEN = %q, want %q", black.fenSeen, startFEN)
	}
}

func TestArbiterRequiresEngines(t *testing.T) {
	if _, err := New(Config{Black: newScripted(nil)}); err == nil {
		t.Error("New with nil White should fail")
	}
	if _, err := New(Config{White: newScripted(nil)}); err == nil {
		t.Error("New with nil Black should fail")
	}
}

func TestArbiterInvalidStartFEN(t *testing.T) {
	if _, err := New(Config{
		White: newScripted(nil), Black: newScripted(nil),
		StartFEN: "not-a-fen",
	}); err == nil {
		t.Error("New with invalid StartFEN should fail")
	}
}

func cpScore(cp int) uci.Score    { return uci.Score{Centipawns: &cp} }
func mateScore(n int) uci.Score   { return uci.Score{Mate: &n} }
func info(depth int, s uci.Score) uci.AnalysisInfo {
	return uci.AnalysisInfo{Depth: depth, Score: s}
}

func TestArbiterCapturesAnalysisInfo(t *testing.T) {
	white := newScripted([]string{"e2e4", "g1f3"})
	black := newScripted([]string{"e7e5", "g8f6"})
	white.scriptInfos([][]uci.AnalysisInfo{
		{info(8, cpScore(20)), info(12, cpScore(35))},
		{info(10, cpScore(50))},
	})
	black.scriptInfos([][]uci.AnalysisInfo{
		{info(11, cpScore(-30))},
		{info(9, cpScore(-25))},
	})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    4,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Moves) != 4 {
		t.Fatalf("len(Moves) = %d, want 4", len(res.Moves))
	}
	want := []struct {
		uci   string
		side  chess.Side
		depth int
		cp    int
	}{
		{"e2e4", chess.White, 12, 35},
		{"e7e5", chess.Black, 11, -30},
		{"g1f3", chess.White, 10, 50},
		{"g8f6", chess.Black, 9, -25},
	}
	for i, w := range want {
		got := res.Moves[i]
		if got.UCI != w.uci || got.Side != w.side {
			t.Errorf("Moves[%d] = (%s %s), want (%s %s)", i, got.UCI, got.Side, w.uci, w.side)
		}
		if !got.HasInfo {
			t.Errorf("Moves[%d].HasInfo = false, want true", i)
		}
		if got.Info.Depth != w.depth {
			t.Errorf("Moves[%d].Depth = %d, want %d", i, got.Info.Depth, w.depth)
		}
		if got.Info.Score.Centipawns == nil || *got.Info.Score.Centipawns != w.cp {
			t.Errorf("Moves[%d].Score = %v, want cp %d", i, got.Info.Score, w.cp)
		}
	}

	// SAN should be populated.
	if res.Moves[0].SAN != "e4" {
		t.Errorf("Moves[0].SAN = %q, want e4", res.Moves[0].SAN)
	}
}

func TestArbiterResignAdjudicationWhiteResigns(t *testing.T) {
	// White consistently sees -8.00 (cp -800); ResignScore=600, ResignMoves=2.
	// White's eval after move 1 is -800 → streak 1.
	// White's eval after move 3 is -800 → streak 2 → resign.
	white := newScripted([]string{"e2e4", "g1f3", "b1c3"})
	black := newScripted([]string{"e7e5", "g8f6", "b8c6"})
	white.scriptInfos([][]uci.AnalysisInfo{
		{info(15, cpScore(-800))},
		{info(15, cpScore(-900))},
	})
	black.scriptInfos([][]uci.AnalysisInfo{
		{info(15, cpScore(800))},
	})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		ResignScore: 600,
		ResignMoves: 2,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Outcome != chess.BlackWins {
		t.Errorf("Outcome = %q, want BlackWins (white resigns)", res.Outcome)
	}
	if res.Reason != chess.ReasonResignation {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonResignation)
	}
	if res.Loser != "W" {
		t.Errorf("Loser = %q, want W", res.Loser)
	}
	// Three plies: white(-800), black(+800), white(-900) — adjudicated after white's 2nd move.
	if res.PlyCount != 3 {
		t.Errorf("PlyCount = %d, want 3", res.PlyCount)
	}
}

func TestArbiterResignAdjudicationOnMate(t *testing.T) {
	// Black's eval after one move shows mate-against-self → streak counts;
	// after two such moves with ResignMoves=2, black resigns.
	white := newScripted([]string{"e2e4", "g1f3"})
	black := newScripted([]string{"e7e5", "g8f6"})
	black.scriptInfos([][]uci.AnalysisInfo{
		{info(20, mateScore(-3))},
		{info(20, mateScore(-2))},
	})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		ResignScore: 600,
		ResignMoves: 2,
	})
	res, _ := arb.Run(context.Background())

	if res.Outcome != chess.WhiteWins {
		t.Errorf("Outcome = %q, want WhiteWins (black resigns on mate)", res.Outcome)
	}
	if res.Loser != "B" {
		t.Errorf("Loser = %q, want B", res.Loser)
	}
}

func TestArbiterDrawAdjudication(t *testing.T) {
	// Six plies of near-zero eval, after DrawMinPly=4 → adjudicate draw.
	white := newScripted([]string{"e2e4", "g1f3", "b1c3"})
	black := newScripted([]string{"e7e5", "g8f6", "b8c6"})
	white.scriptInfos([][]uci.AnalysisInfo{
		{info(12, cpScore(5))},
		{info(12, cpScore(-3))},
		{info(12, cpScore(0))},
	})
	black.scriptInfos([][]uci.AnalysisInfo{
		{info(12, cpScore(-2))},
		{info(12, cpScore(4))},
		{info(12, cpScore(1))},
	})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		DrawScore:   10,
		DrawMoves:   6,
		DrawMinPly:  4,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Outcome != chess.Drawn {
		t.Errorf("Outcome = %q, want Drawn", res.Outcome)
	}
	if res.Reason != chess.ReasonAdjudication {
		t.Errorf("Reason = %q, want %q", res.Reason, chess.ReasonAdjudication)
	}
	if res.PlyCount != 6 {
		t.Errorf("PlyCount = %d, want 6", res.PlyCount)
	}
}

func TestArbiterDrawAdjudicationRequiresMinPly(t *testing.T) {
	// Even with flat evals, DrawMinPly=10 prevents early draw; MaxPlies caps the game.
	white := newScripted([]string{"e2e4", "g1f3", "b1c3", "f1c4"})
	black := newScripted([]string{"e7e5", "g8f6", "b8c6", "f8c5"})
	for _, e := range []*scripted{white, black} {
		e.scriptInfos([][]uci.AnalysisInfo{
			{info(12, cpScore(0))},
			{info(12, cpScore(0))},
			{info(12, cpScore(0))},
			{info(12, cpScore(0))},
		})
	}

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		TimeControl: TimeControl{FixedDepth: 1},
		DrawScore:   10,
		DrawMoves:   2,
		DrawMinPly:  10,
		MaxPlies:    8,
	})
	res, _ := arb.Run(context.Background())

	if res.Outcome != chess.Drawn {
		t.Errorf("Outcome = %q, want Drawn", res.Outcome)
	}
	// Should hit MaxPlies cap before DrawMinPly is reached.
	if res.PlyCount != 8 {
		t.Errorf("PlyCount = %d, want 8 (MaxPlies cap)", res.PlyCount)
	}
}

func TestArbiterAnnotatedPGN(t *testing.T) {
	white := newScripted([]string{"e2e4", "g1f3"})
	black := newScripted([]string{"e7e5", "g8f6"})
	white.scriptInfos([][]uci.AnalysisInfo{
		{info(10, cpScore(42))},
		{info(10, cpScore(35))},
	})
	black.scriptInfos([][]uci.AnalysisInfo{
		{info(10, cpScore(-15))}, // black POV → +0.15 from white POV
		{info(10, mateScore(-5))}, // black sees black being mated in 5 → #5 from white POV
	})

	arb, _ := New(Config{
		White: white, Black: black,
		WhiteName: "Stockfish-W", BlackName: "Stockfish-B",
		Event: "Test Match", Site: "localhost", Round: "1",
		TimeControl: TimeControl{FixedDepth: 10},
		MaxPlies:    4,
	})
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	pgn := arb.AnnotatedPGN(res)

	mustContain := []string{
		`[Event "Test Match"]`,
		`[Site "localhost"]`,
		`[Round "1"]`,
		`[White "Stockfish-W"]`,
		`[Black "Stockfish-B"]`,
		`[Result "1/2-1/2"]`,
		`[TimeControl "depth/10"]`,
		`[Termination "adjudication"]`,
		`1. e4 {[%eval +0.42]}`,
		`e5 {[%eval +0.15]}`, // sign-flipped to white POV
		`2. Nf3 {[%eval +0.35]}`,
		`Nf6 {[%eval #5]}`,
		`1/2-1/2`,
	}
	for _, want := range mustContain {
		if !strings.Contains(pgn, want) {
			t.Errorf("PGN missing %q\nfull PGN:\n%s", want, pgn)
		}
	}
}

func TestArbiterAnnotatedPGNFromCustomFEN(t *testing.T) {
	// Black-to-move starting position, full move number 5.
	startFEN := "rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 5"
	white := newScripted([]string{"g1f3"})
	black := newScripted([]string{"d7d6"})

	arb, err := New(Config{
		White: white, Black: black,
		WhiteName: "W", BlackName: "B",
		StartFEN:    startFEN,
		TimeControl: TimeControl{FixedDepth: 1},
		MaxPlies:    2,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	res, err := arb.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	pgn := arb.AnnotatedPGN(res)
	for _, want := range []string{
		`[FEN "` + startFEN + `"]`,
		`[SetUp "1"]`,
		"5... ",
		"6. Nf3",
	} {
		if !strings.Contains(pgn, want) {
			t.Errorf("PGN missing %q\nfull PGN:\n%s", want, pgn)
		}
	}
}

func TestFormatClock(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00:00.000"},
		{500 * time.Millisecond, "0:00:00.500"},
		{90 * time.Second, "0:01:30.000"},
		{(2*time.Hour + 3*time.Minute + 4*time.Second + 567*time.Millisecond), "2:03:04.567"},
		{-1 * time.Second, "0:00:00.000"},
	}
	for _, tt := range tests {
		got := formatClock(tt.d)
		if got != tt.want {
			t.Errorf("formatClock(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
