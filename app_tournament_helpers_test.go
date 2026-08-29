package main

import (
	"errors"
	"slices"
	"testing"
	"time"

	"rungine/internal/chess"
	"rungine/internal/tournament"
)

const startposFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func mkHelperOutcome(num int, white, black string, out chess.Outcome, reason chess.Reason) tournament.GameOutcome {
	return tournament.GameOutcome{
		Pairing: tournament.Pairing{
			GameNumber: num, Round: "1.1",
			White: tournament.EngineSpec{Name: white, BinaryPath: "/usr/bin/" + white},
			Black: tournament.EngineSpec{Name: black, BinaryPath: "/usr/bin/" + black},
		},
		Result: &tournament.Result{Outcome: out, Reason: reason, PlyCount: 2},
	}
}

func fiveGameFixture() []tournament.GameOutcome {
	// 5th game between A and B crashed (Err set, no Result).
	return []tournament.GameOutcome{
		mkHelperOutcome(1, "A", "B", chess.WhiteWins, chess.ReasonCheckmate),
		mkHelperOutcome(2, "B", "A", chess.BlackWins, chess.ReasonResignation),
		mkHelperOutcome(3, "A", "B", chess.Drawn, chess.ReasonThreefold),
		mkHelperOutcome(4, "C", "D", chess.WhiteWins, chess.ReasonCheckmate),
		{
			Pairing: tournament.Pairing{
				GameNumber: 5, Round: "1.5",
				White: tournament.EngineSpec{Name: "A", BinaryPath: "/usr/bin/A"},
				Black: tournament.EngineSpec{Name: "B", BinaryPath: "/usr/bin/B"},
			},
			Err: errors.New("engine crashed"),
		},
	}
}

func TestParseRunCounter(t *testing.T) {
	cases := []struct {
		id   string
		want int
	}{
		{"t1", 1},
		{"t42", 42},
		{"t", 0},
		{"", 0},
		{"x1", 0},
		{"t1x", 0},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if got := parseRunCounter(c.id); got != c.want {
				t.Errorf("parseRunCounter(%q) = %d, want %d", c.id, got, c.want)
			}
		})
	}
}

func assertTimeControl(t *testing.T, name string, got, want tournament.TimeControl) {
	t.Helper()
	if got.Initial != want.Initial ||
		got.Increment != want.Increment ||
		got.MovesPerPeriod != want.MovesPerPeriod ||
		got.FixedDepth != want.FixedDepth ||
		got.FixedNodes != want.FixedNodes ||
		got.FixedMovetime != want.FixedMovetime {
		t.Errorf("%s: got %+v, want %+v", name, got, want)
	}
}

func TestTimeControlFromSpec(t *testing.T) {
	cases := []struct {
		name string
		spec TournamentSpec
		want tournament.TimeControl
	}{
		{"initial+increment", TournamentSpec{TcInitialMs: 90000, TcIncrementMs: 600},
			tournament.TimeControl{Initial: 90 * time.Second, Increment: 600 * time.Millisecond}},
		{"fixed movetime", TournamentSpec{TimeControlMs: 1000},
			tournament.TimeControl{FixedMovetime: time.Second}},
		{"fixed depth", TournamentSpec{DepthLimit: 10},
			tournament.TimeControl{FixedDepth: 10}},
		{"fixed nodes", TournamentSpec{NodesLimit: 50000},
			tournament.TimeControl{FixedNodes: 50000}},
		{"empty default", TournamentSpec{},
			tournament.TimeControl{FixedMovetime: 200 * time.Millisecond}},
		{"precedence tcInitial wins", TournamentSpec{TcInitialMs: 90000, TimeControlMs: 1000, DepthLimit: 10},
			tournament.TimeControl{Initial: 90 * time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertTimeControl(t, "timeControlFromSpec", timeControlFromSpec(c.spec), c.want)
		})
	}
}

func TestOpeningFEN(t *testing.T) {
	cases := []struct {
		name    string
		pairing tournament.Pairing
		want    string
	}{
		{"empty pair default startpos", tournament.Pairing{}, startposFEN},
		{"after 1.e4", tournament.Pairing{StartMoves: []string{"e2e4"}},
			"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"},
		{"invalid startfen", tournament.Pairing{StartFEN: "not-a-fen"}, ""},
		{"invalid startmove returns pos before failure", tournament.Pairing{StartMoves: []string{"z9z9"}}, startposFEN},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := openingFEN(c.pairing); got != c.want {
				t.Errorf("openingFEN(%+v) = %q, want %q", c.pairing, got, c.want)
			}
		})
	}
}

func TestSprtTally(t *testing.T) {
	outcomes := fiveGameFixture()
	cases := []struct {
		candidate           string
		wantW, wantD, wantL int
	}{
		{"A", 2, 1, 0},
		{"B", 0, 1, 2},
		{"NotPlaying", 0, 0, 0},
	}
	for _, c := range cases {
		w, d, l := sprtTally(outcomes, c.candidate)
		if w != c.wantW || d != c.wantD || l != c.wantL {
			t.Errorf("sprtTally(%q) = (%d, %d, %d), want (%d, %d, %d)",
				c.candidate, w, d, l, c.wantW, c.wantD, c.wantL)
		}
	}
}

func TestBuildCrosstableDataNil(t *testing.T) {
	ct := buildCrosstableData(nil)
	if ct.Players == nil {
		t.Errorf("Players is nil, want non-nil empty slice")
	}
	if len(ct.Players) != 0 {
		t.Errorf("Players len = %d, want 0", len(ct.Players))
	}
	if ct.Cells == nil {
		t.Errorf("Cells is nil, want non-nil empty slice")
	}
	if len(ct.Cells) != 0 {
		t.Errorf("Cells len = %d, want 0", len(ct.Cells))
	}
}

func TestBuildCrosstableDataPlayers(t *testing.T) {
	ct := buildCrosstableData(fiveGameFixture())
	if !slices.Contains(ct.Players, "A") || !slices.Contains(ct.Players, "B") {
		t.Fatalf("crosstable players = %v, want both A and B present", ct.Players)
	}
	iA := slices.Index(ct.Players, "A")
	iB := slices.Index(ct.Players, "B")
	cell := ct.Cells[iA][iB]
	if cell.Wins != 2 || cell.Draws != 1 || cell.Losses != 0 {
		t.Errorf("cell[A][B] W/D/L = %d/%d/%d, want 2/1/0", cell.Wins, cell.Draws, cell.Losses)
	}
	if cell.Games != 3 {
		t.Errorf("cell[A][B].Games = %d, want 3 (C-vs-D game leaked into A's row)", cell.Games)
	}
	if cell.Points != 2.5 {
		t.Errorf("cell[A][B].Points = %v, want 2.5", cell.Points)
	}
}

func TestGameOutcomeRow(t *testing.T) {
	outcomes := fiveGameFixture()

	row := gameOutcomeRow(outcomes[0])
	if row.GameNumber != 1 {
		t.Errorf("GameNumber = %d, want 1", row.GameNumber)
	}
	if row.White != "A" || row.Black != "B" {
		t.Errorf("White/Black = %q/%q, want A/B", row.White, row.Black)
	}
	if row.Outcome != "1-0" {
		t.Errorf("Outcome = %q, want %q", row.Outcome, "1-0")
	}
	if row.Reason != string(chess.ReasonCheckmate) {
		t.Errorf("Reason = %q, want %q", row.Reason, string(chess.ReasonCheckmate))
	}
	if row.Plies != 2 {
		t.Errorf("Plies = %d, want 2", row.Plies)
	}
	if row.Error != "" {
		t.Errorf("Error = %q, want empty", row.Error)
	}

	errRow := gameOutcomeRow(outcomes[4])
	if errRow.Error != "engine crashed" {
		t.Errorf("Error = %q, want %q", errRow.Error, "engine crashed")
	}
	if errRow.Outcome != "" {
		t.Errorf("Outcome = %q, want empty for errored game", errRow.Outcome)
	}
	if errRow.Reason != "" {
		t.Errorf("Reason = %q, want empty for errored game", errRow.Reason)
	}
	if errRow.Plies != 0 {
		t.Errorf("Plies = %d, want 0 for errored game", errRow.Plies)
	}
	if errRow.GameNumber != 5 {
		t.Errorf("GameNumber = %d, want 5", errRow.GameNumber)
	}
	if errRow.White != "A" || errRow.Black != "B" {
		t.Errorf("White/Black = %q/%q, want A/B", errRow.White, errRow.Black)
	}
}
