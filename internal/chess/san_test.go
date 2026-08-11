package chess

import "testing"

// TestUCIToSANRare covers SAN corner cases the repo's game fixtures never
// exercise: multi-piece disambiguation, en passant giving check, and
// promotion (with capture) checking/mating. The positions are hand-built
// FENs; each expected SAN is a single token.
func TestUCIToSANRare(t *testing.T) {
	tests := []struct {
		name    string
		fen     string
		uci     []string
		wantSAN []string
	}{
		{
			name:    "plain promotion",
			fen:     "8/4P2k/8/8/8/8/8/4K3 w - - 0 1",
			uci:     []string{"e7e8q"},
			wantSAN: []string{"e8=Q"},
		},
		{
			name:    "promotion with capture",
			fen:     "4q3/3P4/8/8/8/8/8/2k1K3 w - - 0 1",
			uci:     []string{"d7e8q"},
			wantSAN: []string{"dxe8=Q"},
		},
		{
			name:    "promotion with capture and mate",
			fen:     "6rk/5K1P/8/8/8/8/8/8 w - - 0 1",
			uci:     []string{"h7g8q"},
			wantSAN: []string{"hxg8=Q#"},
		},
		{
			name:    "en passant gives check",
			fen:     "8/5k2/8/3Pp3/8/8/8/4K3 w - e6 0 1",
			uci:     []string{"d5e6"},
			wantSAN: []string{"dxe6+"},
		},
		{
			name:    "kingside castling",
			fen:     "4k3/8/8/8/8/8/8/4K2R w K - 0 1",
			uci:     []string{"e1g1"},
			wantSAN: []string{"O-O"},
		},
		{
			name:    "queenside castling",
			fen:     "4k3/8/8/8/8/8/8/R3K3 w Q - 0 1",
			uci:     []string{"e1c1"},
			wantSAN: []string{"O-O-O"},
		},
		{
			name: "fools mate",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			uci:  []string{"f2f3", "e7e5", "g2g4", "d8h4"},
			// The moves above are the UCI script from internal/chess's own
			// TestFoolsMate; the SAN is the authoritative mate Qh4#.
			wantSAN: []string{"f3", "e5", "g4", "Qh4#"},
		},
		{
			name:    "double disambiguation three queens",
			fen:     "3Q4/8/8/8/8/8/3Q1Q2/4K3 w - - 0 1",
			uci:     []string{"d2d4"},
			wantSAN: []string{"Qd2d4"},
		},
		{
			name:    "rank disambiguation rooks same file",
			fen:     "R7/8/8/8/8/7k/8/R3K3 w - - 0 1",
			uci:     []string{"a1a4"},
			wantSAN: []string{"R1a4"},
		},
		{
			name:    "file disambiguation knights same rank",
			fen:     "7k/8/8/8/8/8/8/1N1NK3 w - - 0 1",
			uci:     []string{"b1c3"},
			wantSAN: []string{"Nbc3"},
		},
	}

	for _, tc := range tests {
		got, err := UCIToSAN(tc.fen, tc.uci)
		if err != nil {
			t.Errorf("%s: UCIToSAN error: %v", tc.name, err)
			continue
		}
		if len(got) != len(tc.wantSAN) {
			t.Errorf("%s: got %d tokens %v, want %d %v", tc.name, len(got), got, len(tc.wantSAN), tc.wantSAN)
			continue
		}
		for i := range got {
			if got[i] != tc.wantSAN[i] {
				t.Errorf("%s: ply %d: got %q, want %q", tc.name, i+1, got[i], tc.wantSAN[i])
			}
		}
	}
}
