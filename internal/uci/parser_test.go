package uci

import (
	"testing"
	"time"
)

func intPtr(n int) *int {
	return &n
}

func TestParseInfoLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		check    func(t *testing.T, info AnalysisInfo)
	}{
		{
			name:     "basic depth and score",
			input:    "info depth 20 score cp 35",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.Depth != 20 {
					t.Errorf("Depth = %d, want 20", info.Depth)
				}
				if info.Score.Centipawns == nil || *info.Score.Centipawns != 35 {
					t.Errorf("Score.Centipawns = %v, want 35", info.Score.Centipawns)
				}
			},
		},
		{
			name:     "full info line",
			input:    "info depth 20 seldepth 25 score cp 35 nodes 1500000 nps 2500000 time 600 pv e2e4 e7e5 g1f3",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.Depth != 20 {
					t.Errorf("Depth = %d, want 20", info.Depth)
				}
				if info.SelDepth != 25 {
					t.Errorf("SelDepth = %d, want 25", info.SelDepth)
				}
				if info.Score.Centipawns == nil || *info.Score.Centipawns != 35 {
					t.Errorf("Score.Centipawns = %v, want 35", info.Score.Centipawns)
				}
				if info.Nodes != 1500000 {
					t.Errorf("Nodes = %d, want 1500000", info.Nodes)
				}
				if info.NPS != 2500000 {
					t.Errorf("NPS = %d, want 2500000", info.NPS)
				}
				if info.Time != 600*time.Millisecond {
					t.Errorf("Time = %v, want 600ms", info.Time)
				}
				wantPV := []string{"e2e4", "e7e5", "g1f3"}
				if len(info.PV) != len(wantPV) {
					t.Errorf("PV length = %d, want %d", len(info.PV), len(wantPV))
				}
				for i, m := range wantPV {
					if i < len(info.PV) && info.PV[i] != m {
						t.Errorf("PV[%d] = %s, want %s", i, info.PV[i], m)
					}
				}
			},
		},
		{
			name:     "mate score",
			input:    "info depth 30 score mate 5 pv d8h4 g2g3 h4g3",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.Depth != 30 {
					t.Errorf("Depth = %d, want 30", info.Depth)
				}
				if info.Score.Mate == nil || *info.Score.Mate != 5 {
					t.Errorf("Score.Mate = %v, want 5", info.Score.Mate)
				}
				if info.Score.Centipawns != nil {
					t.Errorf("Score.Centipawns should be nil for mate score")
				}
			},
		},
		{
			name:     "negative mate",
			input:    "info depth 25 score mate -3",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.Score.Mate == nil || *info.Score.Mate != -3 {
					t.Errorf("Score.Mate = %v, want -3", info.Score.Mate)
				}
			},
		},
		{
			name:     "multipv",
			input:    "info depth 15 multipv 2 score cp -10 pv d7d5",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.MultiPV != 2 {
					t.Errorf("MultiPV = %d, want 2", info.MultiPV)
				}
			},
		},
		{
			name:     "bounds",
			input:    "info depth 10 score cp 100 lowerbound",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if !info.Score.LowerBound {
					t.Error("Score.LowerBound should be true")
				}
			},
		},
		{
			name:     "currmove",
			input:    "info depth 5 currmove e2e4 currmovenumber 1",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.CurrMove != "e2e4" {
					t.Errorf("CurrMove = %s, want e2e4", info.CurrMove)
				}
				if info.CurrMoveNumber != 1 {
					t.Errorf("CurrMoveNumber = %d, want 1", info.CurrMoveNumber)
				}
			},
		},
		{
			name:     "hashfull and tbhits",
			input:    "info depth 20 hashfull 500 tbhits 1234",
			wantType: "info",
			check: func(t *testing.T, info AnalysisInfo) {
				if info.HashFull != 500 {
					t.Errorf("HashFull = %d, want 500", info.HashFull)
				}
				if info.TBHits != 1234 {
					t.Errorf("TBHits = %d, want 1234", info.TBHits)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseLine(tc.input)
			if result.Type != tc.wantType {
				t.Errorf("ParseLine().Type = %s, want %s", result.Type, tc.wantType)
				return
			}
			if tc.check != nil {
				info := result.Data.(AnalysisInfo)
				tc.check(t, info)
			}
		})
	}
}

func TestParseOptionLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantType UCIOptionType
		check    func(t *testing.T, opt UCIOption)
	}{
		{
			name:     "spin option",
			input:    "option name Hash type spin default 16 min 1 max 33554432",
			wantName: "Hash",
			wantType: OptionTypeSpin,
			check: func(t *testing.T, opt UCIOption) {
				if opt.Default != "16" {
					t.Errorf("Default = %s, want 16", opt.Default)
				}
				if opt.Min == nil || *opt.Min != 1 {
					t.Errorf("Min = %v, want 1", opt.Min)
				}
				if opt.Max == nil || *opt.Max != 33554432 {
					t.Errorf("Max = %v, want 33554432", opt.Max)
				}
			},
		},
		{
			name:     "check option",
			input:    "option name Ponder type check default false",
			wantName: "Ponder",
			wantType: OptionTypeCheck,
			check: func(t *testing.T, opt UCIOption) {
				if opt.Default != "false" {
					t.Errorf("Default = %s, want false", opt.Default)
				}
			},
		},
		{
			name:     "combo option",
			input:    "option name Analysis Contempt type combo default Both var Off var White var Black var Both",
			wantName: "Analysis Contempt",
			wantType: OptionTypeCombo,
			check: func(t *testing.T, opt UCIOption) {
				if opt.Default != "Both" {
					t.Errorf("Default = %s, want Both", opt.Default)
				}
				wantVars := []string{"Off", "White", "Black", "Both"}
				if len(opt.Vars) != len(wantVars) {
					t.Errorf("Vars length = %d, want %d", len(opt.Vars), len(wantVars))
				}
				for i, v := range wantVars {
					if i < len(opt.Vars) && opt.Vars[i] != v {
						t.Errorf("Vars[%d] = %s, want %s", i, opt.Vars[i], v)
					}
				}
			},
		},
		{
			name:     "string option",
			input:    "option name SyzygyPath type string default <empty>",
			wantName: "SyzygyPath",
			wantType: OptionTypeString,
			check: func(t *testing.T, opt UCIOption) {
				if opt.Default != "<empty>" {
					t.Errorf("Default = %s, want <empty>", opt.Default)
				}
			},
		},
		{
			name:     "button option",
			input:    "option name Clear Hash type button",
			wantName: "Clear Hash",
			wantType: OptionTypeButton,
			check:    nil,
		},
		{
			name:     "multi-word name",
			input:    "option name UCI_AnalyseMode type check default false",
			wantName: "UCI_AnalyseMode",
			wantType: OptionTypeCheck,
			check:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ParseLine(tc.input)
			if result.Type != "option" {
				t.Errorf("ParseLine().Type = %s, want option", result.Type)
				return
			}
			opt := result.Data.(UCIOption)
			if opt.Name != tc.wantName {
				t.Errorf("Name = %s, want %s", opt.Name, tc.wantName)
			}
			if opt.Type != tc.wantType {
				t.Errorf("Type = %s, want %s", opt.Type, tc.wantType)
			}
			if tc.check != nil {
				tc.check(t, opt)
			}
		})
	}
}

func TestParseIDLine(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantData string
	}{
		{"id name Stockfish 17", "id_name", "Stockfish 17"},
		{"id author the Stockfish developers", "id_author", "the Stockfish developers"},
		{"id name Leela Chess Zero v0.31.0", "id_name", "Leela Chess Zero v0.31.0"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseLine(tc.input)
			if result.Type != tc.wantType {
				t.Errorf("Type = %s, want %s", result.Type, tc.wantType)
			}
			if result.Data.(string) != tc.wantData {
				t.Errorf("Data = %s, want %s", result.Data, tc.wantData)
			}
		})
	}
}

func TestParseBestMove(t *testing.T) {
	tests := []struct {
		input      string
		wantMove   string
		wantPonder string
	}{
		{"bestmove e2e4", "e2e4", ""},
		{"bestmove e2e4 ponder e7e5", "e2e4", "e7e5"},
		{"bestmove g1f3 ponder b8c6", "g1f3", "b8c6"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseLine(tc.input)
			if result.Type != "bestmove" {
				t.Errorf("Type = %s, want bestmove", result.Type)
				return
			}
			bm := result.Data.(BestMove)
			if bm.Move != tc.wantMove {
				t.Errorf("Move = %s, want %s", bm.Move, tc.wantMove)
			}
			if bm.Ponder != tc.wantPonder {
				t.Errorf("Ponder = %s, want %s", bm.Ponder, tc.wantPonder)
			}
		})
	}
}

func TestParseSimpleResponses(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
	}{
		{"uciok", "uciok"},
		{"readyok", "readyok"},
		{"", "empty"},
		{"   ", "empty"},
		{"unknown command", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := ParseLine(tc.input)
			if result.Type != tc.wantType {
				t.Errorf("Type = %s, want %s", result.Type, tc.wantType)
			}
		})
	}
}

func TestBuildGoCommand(t *testing.T) {
	tests := []struct {
		name   string
		params GoParams
		want   string
	}{
		{
			name:   "infinite",
			params: GoParams{Infinite: true},
			want:   "go infinite",
		},
		{
			name:   "depth",
			params: GoParams{Depth: 20},
			want:   "go depth 20",
		},
		{
			name:   "movetime",
			params: GoParams{MoveTime: 5 * time.Second},
			want:   "go movetime 5000",
		},
		{
			name: "time control",
			params: GoParams{
				WhiteTime: 5 * time.Minute,
				BlackTime: 5 * time.Minute,
				WhiteInc:  3 * time.Second,
				BlackInc:  3 * time.Second,
			},
			want: "go wtime 300000 btime 300000 winc 3000 binc 3000",
		},
		{
			name:   "nodes",
			params: GoParams{Nodes: 1000000},
			want:   "go nodes 1000000",
		},
		{
			name:   "searchmoves",
			params: GoParams{Depth: 10, SearchMoves: []string{"e2e4", "d2d4"}},
			want:   "go depth 10 searchmoves e2e4 d2d4",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildGoCommand(tc.params)
			if got != tc.want {
				t.Errorf("BuildGoCommand() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildPositionCommand(t *testing.T) {
	tests := []struct {
		name  string
		fen   string
		moves []string
		want  string
	}{
		{
			name: "startpos",
			fen:  "startpos",
			want: "position startpos",
		},
		{
			name: "empty fen means startpos",
			fen:  "",
			want: "position startpos",
		},
		{
			name:  "startpos with moves",
			fen:   "startpos",
			moves: []string{"e2e4", "e7e5"},
			want:  "position startpos moves e2e4 e7e5",
		},
		{
			name: "fen",
			fen:  "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
			want: "position fen rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		},
		{
			name:  "fen with moves",
			fen:   "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
			moves: []string{"e7e5"},
			want:  "position fen rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1 moves e7e5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildPositionCommand(tc.fen, tc.moves)
			if got != tc.want {
				t.Errorf("BuildPositionCommand() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestBuildSetOptionCommand(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"Hash", "256", "setoption name Hash value 256"},
		{"Threads", "4", "setoption name Threads value 4"},
		{"Clear Hash", "", "setoption name Clear Hash"},
		{"SyzygyPath", "/path/to/syzygy", "setoption name SyzygyPath value /path/to/syzygy"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildSetOptionCommand(tc.name, tc.value)
			if got != tc.want {
				t.Errorf("BuildSetOptionCommand() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestScoreString(t *testing.T) {
	tests := []struct {
		name  string
		score Score
		want  string
	}{
		{"positive cp", Score{Centipawns: intPtr(35)}, "+0.35"},
		{"negative cp", Score{Centipawns: intPtr(-120)}, "-1.20"},
		{"zero cp", Score{Centipawns: intPtr(0)}, "0.00"},
		{"mate in 5", Score{Mate: intPtr(5)}, "M5"},
		{"mated in 3", Score{Mate: intPtr(-3)}, "-M3"},
		{"no score", Score{}, "?"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.score.String()
			if got != tc.want {
				t.Errorf("Score.String() = %s, want %s", got, tc.want)
			}
		})
	}
}
