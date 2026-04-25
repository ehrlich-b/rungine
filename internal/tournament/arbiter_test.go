package tournament

import (
	"context"
	"errors"
	"slices"
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
}

func newScripted(moves []string) *scripted {
	return &scripted{
		moves:      moves,
		bestMoveCh: make(chan uci.BestMove, 4),
	}
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
