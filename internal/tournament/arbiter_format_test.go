package tournament

import (
	"strconv"
	"testing"
	"time"

	"rungine/internal/chess"
	"rungine/internal/uci"
)

func intPtr(v int) *int {
	return &v
}

func TestArbiterFormatEval(t *testing.T) {
	tests := []struct {
		name  string
		score uci.Score
		mover chess.Side
		want  string
	}{
		{"centipawns white positive", uci.Score{Centipawns: intPtr(42)}, chess.White, "+0.42"},
		{"centipawns white negative", uci.Score{Centipawns: intPtr(-42)}, chess.White, "-0.42"},
		{"centipawns black positive", uci.Score{Centipawns: intPtr(42)}, chess.Black, "-0.42"},
		{"mate white positive", uci.Score{Mate: intPtr(5)}, chess.White, "#5"},
		{"mate black positive", uci.Score{Mate: intPtr(5)}, chess.Black, "#-5"},
		{"mate white negative", uci.Score{Mate: intPtr(-3)}, chess.White, "#-3"},
		{"empty score", uci.Score{}, chess.White, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatEval(tt.score, tt.mover); got != tt.want {
				t.Errorf("formatEval(%+v, %s) = %q, want %q", tt.score, tt.mover, got, tt.want)
			}
		})
	}
}

func TestArbiterFormatSeconds(t *testing.T) {
	// A non-round value the test constructs itself: 1s + 250ms. The
	// documented algorithm falls through to strconv.FormatFloat with
	// precision -1, so the expected string is derived from that, not from a
	// fixed-decimals format.
	nonRound := time.Second + 250*time.Millisecond
	wantNonRound := strconv.FormatFloat(nonRound.Seconds(), 'f', -1, 64)

	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero", 0, "0"},
		{"whole seconds", 90 * time.Second, "90"},
		{"subsecond tenths", 600 * time.Millisecond, "0.6"},
		{"subsecond one and a half", 1500 * time.Millisecond, "1.5"},
		{"self-constructed non-round", nonRound, wantNonRound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSeconds(tt.d); got != tt.want {
				t.Errorf("formatSeconds(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestArbiterFormatTimeControlPGN(t *testing.T) {
	tests := []struct {
		name string
		tc   TimeControl
		want string
	}{
		{"fixed movetime", TimeControl{FixedMovetime: 100 * time.Millisecond}, "movetime/100ms"},
		{"fixed nodes", TimeControl{FixedNodes: 100000}, "nodes/100000"},
		{"fixed depth", TimeControl{FixedDepth: 10}, "depth/10"},
		{"nothing set", TimeControl{}, ""},
		{"clock with increment", TimeControl{Initial: 90 * time.Second, Increment: 600 * time.Millisecond}, "90+0.6"},
		{"clocl with moves per period", TimeControl{Initial: 900 * time.Second, MovesPerPeriod: 40}, "40/900"},
		{"fixed mode beats clock mode", TimeControl{FixedDepth: 10, Initial: 90 * time.Second}, "depth/10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTimeControlPGN(tt.tc); got != tt.want {
				t.Errorf("formatTimeControlPGN(%+v) = %q, want %q", tt.tc, got, tt.want)
			}
		})
	}
}

func TestArbiterFormatAnnotation(t *testing.T) {
	tests := []struct {
		name string
		rec  MoveRecord
		want string
	}{
		{
			"eval and clock",
			MoveRecord{
				Side:       chess.White,
				Info:       uci.AnalysisInfo{Score: uci.Score{Centipawns: intPtr(42)}},
				ClockAfter: 90 * time.Second,
			},
			"[%eval +0.42] [%clk 0:01:30.000]",
		},
		{
			"no eval and no clock",
			MoveRecord{Side: chess.White, ClockAfter: 0},
			"",
		},
		{
			"clock only",
			MoveRecord{Side: chess.White, ClockAfter: 90 * time.Second},
			"[%clk 0:01:30.000]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatAnnotation(tt.rec); got != tt.want {
				t.Errorf("formatAnnotation(%+v) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}

func TestArbiterParseFENMoveNumber(t *testing.T) {
	tests := []struct {
		name     string
		fen      string
		wantNum  int
		wantSide chess.Side
	}{
		{"empty", "", 1, chess.White},
		{"black to move", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 5", 5, chess.Black},
		{"malformed", "garbage fen", 1, chess.White},
		{"white to move", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 10", 10, chess.White},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotSide := parseFENMoveNumber(tt.fen)
			if gotNum != tt.wantNum || gotSide != tt.wantSide {
				t.Errorf("parseFENMoveNumber(%q) = (%d, %s), want (%d, %s)", tt.fen, gotNum, gotSide, tt.wantNum, tt.wantSide)
			}
		})
	}
}

func TestArbiterScoreShowsLoss(t *testing.T) {
	tests := []struct {
		name        string
		score       uci.Score
		thresholdCp int
		want        bool
	}{
		{"down 500 cp", uci.Score{Centipawns: intPtr(-500)}, 300, true},
		{"down 100 cp", uci.Score{Centipawns: intPtr(-100)}, 300, false},
		{"threshold disabled", uci.Score{Centipawns: intPtr(-500)}, 0, false},
		{"negative mate", uci.Score{Mate: intPtr(-2)}, 300, true},
		{"positive mate", uci.Score{Mate: intPtr(2)}, 300, false},
		{"boundary exactly at threshold", uci.Score{Centipawns: intPtr(-300)}, 300, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreShowsLoss(tt.score, tt.thresholdCp); got != tt.want {
				t.Errorf("scoreShowsLoss(%+v, %d) = %t, want %t", tt.score, tt.thresholdCp, got, tt.want)
			}
		})
	}
}

func TestArbiterScoreShowsDraw(t *testing.T) {
	tests := []struct {
		name        string
		score       uci.Score
		thresholdCp int
		want        bool
	}{
		{"within threshold", uci.Score{Centipawns: intPtr(10)}, 20, true},
		{"within threshold negative", uci.Score{Centipawns: intPtr(-10)}, 20, true},
		{"outside threshold", uci.Score{Centipawns: intPtr(50)}, 20, false},
		{"mate score", uci.Score{Mate: intPtr(1)}, 20, false},
		{"threshold disabled", uci.Score{Centipawns: intPtr(0)}, -1, false},
		{"boundary exactly at threshold", uci.Score{Centipawns: intPtr(20)}, 20, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreShowsDraw(tt.score, tt.thresholdCp); got != tt.want {
				t.Errorf("scoreShowsDraw(%+v, %d) = %t, want %t", tt.score, tt.thresholdCp, got, tt.want)
			}
		})
	}
}
