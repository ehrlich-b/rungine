package tournament

import (
	"errors"
	"strings"
	"testing"

	"rungine/internal/chess"
)

// outcome is a small helper to build a GameOutcome for scoring tests.
func outcome(white, black string, result chess.Outcome) GameOutcome {
	return GameOutcome{
		Pairing: Pairing{
			White: EngineSpec{Name: white},
			Black: EngineSpec{Name: black},
		},
		Result: &Result{Outcome: result, Reason: chess.ReasonCheckmate},
	}
}

func TestBuildStandingsBasic(t *testing.T) {
	outcomes := []GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.Drawn),
		outcome("A", "C", chess.BlackWins),
		outcome("C", "B", chess.WhiteWins),
	}
	st := BuildStandings(outcomes)

	if len(st.Players) != 3 {
		t.Fatalf("len(Players) = %d, want 3", len(st.Players))
	}

	scoresByName := map[string]float64{}
	for _, p := range st.Players {
		scoresByName[p.Name] = p.Points
	}
	// A: win + draw + loss = 1.5
	// B: loss + draw + loss = 0.5
	// C: win + win = 2
	if got := scoresByName["A"]; got != 1.5 {
		t.Errorf("A points = %v, want 1.5", got)
	}
	if got := scoresByName["B"]; got != 0.5 {
		t.Errorf("B points = %v, want 0.5", got)
	}
	if got := scoresByName["C"]; got != 2 {
		t.Errorf("C points = %v, want 2", got)
	}

	// Order: C (2), A (1.5), B (0.5).
	want := []string{"C", "A", "B"}
	for i, name := range want {
		if st.Players[i].Name != name {
			t.Errorf("Players[%d] = %q, want %q", i, st.Players[i].Name, name)
		}
	}
}

func TestBuildStandingsTieBreaks(t *testing.T) {
	// Two engines with the same points; tie-broken by wins, then name.
	outcomes := []GameOutcome{
		outcome("alpha", "beta", chess.WhiteWins), // alpha wins
		outcome("beta", "alpha", chess.WhiteWins), // beta wins
		outcome("alpha", "gamma", chess.Drawn),
		outcome("gamma", "alpha", chess.Drawn),
		outcome("beta", "gamma", chess.Drawn),
		outcome("gamma", "beta", chess.Drawn),
	}
	st := BuildStandings(outcomes)
	// alpha: W=1, D=2, L=1 → 2.0 pts
	// beta:  W=1, D=2, L=1 → 2.0 pts
	// gamma: W=0, D=4, L=0 → 2.0 pts
	// Tie on points: alpha and beta both have 1 win, gamma 0; alpha < beta alphabetically.
	wantOrder := []string{"alpha", "beta", "gamma"}
	for i, w := range wantOrder {
		if st.Players[i].Name != w {
			t.Errorf("Players[%d] = %q, want %q", i, st.Players[i].Name, w)
		}
	}
}

func TestBuildStandingsSkipsErrorAndOngoing(t *testing.T) {
	failed := GameOutcome{
		Pairing: Pairing{White: EngineSpec{Name: "A"}, Black: EngineSpec{Name: "B"}},
		Err:     errors.New("crash"),
	}
	ongoing := GameOutcome{
		Pairing: Pairing{White: EngineSpec{Name: "A"}, Black: EngineSpec{Name: "B"}},
		Result:  &Result{Outcome: chess.Ongoing},
	}
	good := outcome("A", "B", chess.WhiteWins)

	st := BuildStandings([]GameOutcome{failed, ongoing, good})
	for _, p := range st.Players {
		if p.Games != 1 {
			t.Errorf("%s.Games = %d, want 1 (only the good outcome counts)", p.Name, p.Games)
		}
	}
}

func TestPlayerScorePct(t *testing.T) {
	p := PlayerScore{Wins: 3, Draws: 2, Losses: 5, Games: 10, Points: 4}
	if got := p.ScorePct(); got != 0.4 {
		t.Errorf("ScorePct = %v, want 0.4", got)
	}
	if got := (PlayerScore{}).ScorePct(); got != 0 {
		t.Errorf("empty ScorePct = %v, want 0", got)
	}
}

func TestStandingsString(t *testing.T) {
	st := BuildStandings([]GameOutcome{
		outcome("Stockfish", "Komodo", chess.WhiteWins),
		outcome("Komodo", "Stockfish", chess.Drawn),
	})
	got := st.String()
	if !strings.Contains(got, "Stockfish") || !strings.Contains(got, "Komodo") {
		t.Errorf("Standings.String() missing engine names:\n%s", got)
	}
	// 1.5 points for Stockfish (win + draw)
	if !strings.Contains(got, "1.5") {
		t.Errorf("Standings.String() missing 1.5 points:\n%s", got)
	}
}

func TestBuildCrosstable(t *testing.T) {
	outcomes := []GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.WhiteWins),
		outcome("A", "C", chess.Drawn),
		outcome("C", "A", chess.WhiteWins),
		outcome("B", "C", chess.Drawn),
		outcome("C", "B", chess.Drawn),
	}
	ct := BuildCrosstable(outcomes)

	if len(ct.Players) != 3 {
		t.Fatalf("len(Players) = %d, want 3", len(ct.Players))
	}

	// Build name → index from the crosstable's Player order.
	idx := map[string]int{}
	for i, n := range ct.Players {
		idx[n] = i
	}

	a, b, c := idx["A"], idx["B"], idx["C"]

	// A vs B: A win as white (1), A loss as black (0) → 1/2.
	if ct.Score[a][b] != 1 || ct.Games[a][b] != 2 {
		t.Errorf("A vs B = %.1f/%d, want 1.0/2", ct.Score[a][b], ct.Games[a][b])
	}
	// A vs C: A draw as white (0.5), A loss as black (0) → 0.5/2.
	if ct.Score[a][c] != 0.5 || ct.Games[a][c] != 2 {
		t.Errorf("A vs C = %.1f/%d, want 0.5/2", ct.Score[a][c], ct.Games[a][c])
	}
	// C vs A: C draw as black (0.5), C win as white (1) → 1.5/2.
	if ct.Score[c][a] != 1.5 || ct.Games[c][a] != 2 {
		t.Errorf("C vs A = %.1f/%d, want 1.5/2", ct.Score[c][a], ct.Games[c][a])
	}

	// Diagonal is zero.
	for i := range ct.Players {
		if ct.Score[i][i] != 0 || ct.Games[i][i] != 0 {
			t.Errorf("diagonal[%d] = %.1f/%d, want 0/0", i, ct.Score[i][i], ct.Games[i][i])
		}
	}
}

func TestBuildCrosstableEmpty(t *testing.T) {
	ct := BuildCrosstable(nil)
	if len(ct.Players) != 0 {
		t.Errorf("empty input should produce empty crosstable, got %d players", len(ct.Players))
	}
}

func TestCrosstableString(t *testing.T) {
	ct := BuildCrosstable([]GameOutcome{
		outcome("A", "B", chess.WhiteWins),
		outcome("B", "A", chess.Drawn),
	})
	s := ct.String()
	if !strings.Contains(s, "A") || !strings.Contains(s, "B") {
		t.Errorf("Crosstable.String() missing engine names:\n%s", s)
	}
	if !strings.Contains(s, "—") {
		t.Errorf("Crosstable.String() missing diagonal placeholder:\n%s", s)
	}
}
