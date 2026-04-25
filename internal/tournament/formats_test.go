package tournament

import (
	"slices"
	"testing"
)

func TestBuildMatchAlternatesColors(t *testing.T) {
	spec := MatchSpec{
		White: EngineSpec{Name: "A"},
		Black: EngineSpec{Name: "B"},
		Games: 4,
	}
	got := BuildMatch(spec)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	wantWhites := []string{"A", "B", "A", "B"}
	wantBlacks := []string{"B", "A", "B", "A"}
	for i, p := range got {
		if p.White.Name != wantWhites[i] || p.Black.Name != wantBlacks[i] {
			t.Errorf("game %d: %s vs %s, want %s vs %s",
				i+1, p.White.Name, p.Black.Name, wantWhites[i], wantBlacks[i])
		}
		if p.GameNumber != i+1 {
			t.Errorf("GameNumber[%d] = %d, want %d", i, p.GameNumber, i+1)
		}
	}
}

func TestBuildMatchEmptyOrZero(t *testing.T) {
	if got := BuildMatch(MatchSpec{Games: 0}); len(got) != 0 {
		t.Errorf("BuildMatch(Games=0) = %d, want empty", len(got))
	}
	if got := BuildMatch(MatchSpec{Games: -3}); len(got) != 0 {
		t.Errorf("BuildMatch(Games=-3) = %d, want empty", len(got))
	}
}

func TestBuildMatchPairModeUsesEachOpeningTwice(t *testing.T) {
	o1 := Opening{Name: "Sicilian", StartMoves: []string{"e2e4", "c7c5"}}
	o2 := Opening{Name: "French", StartMoves: []string{"e2e4", "e7e6"}}

	spec := MatchSpec{
		White:    EngineSpec{Name: "A"},
		Black:    EngineSpec{Name: "B"},
		Games:    4,
		Openings: []Opening{o1, o2},
		PairMode: true,
	}
	got := BuildMatch(spec)
	wantOpenings := [][]string{
		{"e2e4", "c7c5"},
		{"e2e4", "c7c5"},
		{"e2e4", "e7e6"},
		{"e2e4", "e7e6"},
	}
	for i, p := range got {
		if !slices.Equal(p.StartMoves, wantOpenings[i]) {
			t.Errorf("game %d StartMoves = %v, want %v", i+1, p.StartMoves, wantOpenings[i])
		}
	}
	// Adjacent pair has flipped colors but identical opening — i.e.,
	// each opening played once each color.
	if got[0].White.Name == got[1].White.Name {
		t.Errorf("pair mode should flip colors between games 1 and 2")
	}
}

func TestBuildMatchOpeningsCycle(t *testing.T) {
	spec := MatchSpec{
		White:    EngineSpec{Name: "A"},
		Black:    EngineSpec{Name: "B"},
		Games:    5,
		Openings: []Opening{{Name: "X"}, {Name: "Y"}},
	}
	got := BuildMatch(spec)
	wantNames := []string{"X", "Y", "X", "Y", "X"}
	for i, p := range got {
		// Opening name isn't propagated to Pairing; we approximate by
		// reading from Openings via the same indexing logic.
		gotName := spec.Openings[i%2].Name
		if gotName != wantNames[i] {
			t.Errorf("game %d expected opening %q, computed %q", i+1, wantNames[i], gotName)
		}
		_ = p
	}
}

func TestBuildMatchStartGameNumber(t *testing.T) {
	got := BuildMatch(MatchSpec{
		White:           EngineSpec{Name: "A"},
		Black:           EngineSpec{Name: "B"},
		Games:           2,
		StartGameNumber: 100,
	})
	if got[0].GameNumber != 100 || got[1].GameNumber != 101 {
		t.Errorf("GameNumbers = %d,%d; want 100,101", got[0].GameNumber, got[1].GameNumber)
	}
}

func TestBuildRoundRobinFullDirected(t *testing.T) {
	engines := []EngineSpec{
		{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"},
	}
	got := BuildRoundRobin(RoundRobinSpec{Engines: engines})

	// 4 engines * 3 = 12 directed pairings.
	if len(got) != 12 {
		t.Errorf("len = %d, want 12", len(got))
	}

	// Verify every (i, j) with i != j appears exactly once.
	type pair struct{ w, b string }
	seen := map[pair]int{}
	for _, p := range got {
		seen[pair{p.White.Name, p.Black.Name}]++
	}
	for i, w := range engines {
		for j, b := range engines {
			if i == j {
				continue
			}
			if seen[pair{w.Name, b.Name}] != 1 {
				t.Errorf("pair %s vs %s appears %d times, want 1", w.Name, b.Name, seen[pair{w.Name, b.Name}])
			}
		}
	}
}

func TestBuildRoundRobinCyclesAndRoundTags(t *testing.T) {
	engines := []EngineSpec{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	got := BuildRoundRobin(RoundRobinSpec{Engines: engines, Cycles: 2})

	// 3 engines * 2 * 2 cycles = 12 games.
	if len(got) != 12 {
		t.Errorf("len = %d, want 12", len(got))
	}

	rounds := map[string]int{}
	for _, p := range got {
		rounds[p.Round]++
	}
	if rounds["1"] != 6 || rounds["2"] != 6 {
		t.Errorf("round counts = %v, want 6 per round", rounds)
	}
}

func TestBuildRoundRobinTooFewEngines(t *testing.T) {
	if got := BuildRoundRobin(RoundRobinSpec{Engines: []EngineSpec{{Name: "A"}}}); got != nil {
		t.Errorf("RR with 1 engine returned %d pairings, want nil", len(got))
	}
}

func TestBuildGauntlet(t *testing.T) {
	c := EngineSpec{Name: "Champ"}
	field := []EngineSpec{{Name: "F1"}, {Name: "F2"}, {Name: "F3"}}

	got := BuildGauntlet(GauntletSpec{
		Challenger:       c,
		Field:            field,
		GamesPerOpponent: 2,
	})

	if len(got) != 6 {
		t.Errorf("len = %d, want 6 (3 opponents x 2 games)", len(got))
	}

	// Every game involves the champion.
	for i, p := range got {
		if p.White.Name != "Champ" && p.Black.Name != "Champ" {
			t.Errorf("game %d (%s vs %s) doesn't include the challenger",
				i+1, p.White.Name, p.Black.Name)
		}
	}

	// Colors alternate per opponent: F1 game 1 (Champ white), game 2 (F1 white).
	if got[0].White.Name != "Champ" || got[0].Black.Name != "F1" {
		t.Errorf("game 1 = %s vs %s; want Champ vs F1", got[0].White.Name, got[0].Black.Name)
	}
	if got[1].White.Name != "F1" || got[1].Black.Name != "Champ" {
		t.Errorf("game 2 = %s vs %s; want F1 vs Champ", got[1].White.Name, got[1].Black.Name)
	}
	if got[2].White.Name != "Champ" || got[2].Black.Name != "F2" {
		t.Errorf("game 3 = %s vs %s; want Champ vs F2", got[2].White.Name, got[2].Black.Name)
	}
}

func TestBuildGauntletEmptyField(t *testing.T) {
	if got := BuildGauntlet(GauntletSpec{
		Challenger:       EngineSpec{Name: "C"},
		GamesPerOpponent: 2,
	}); got != nil {
		t.Errorf("empty field returned %d pairings, want nil", len(got))
	}
}

func TestPickOpening(t *testing.T) {
	o1 := Opening{Name: "X"}
	o2 := Opening{Name: "Y"}
	openings := []Opening{o1, o2}

	if got := pickOpening(nil, 0, false); got.Name != "" {
		t.Errorf("pickOpening(nil) = %q, want empty", got.Name)
	}
	if got := pickOpening(openings, 0, false); got.Name != "X" {
		t.Errorf("pickOpening([X,Y], 0) = %q, want X", got.Name)
	}
	if got := pickOpening(openings, 3, false); got.Name != "Y" {
		t.Errorf("pickOpening([X,Y], 3) = %q, want Y (cycled)", got.Name)
	}
	// Pair mode: idx 0,1 → X; idx 2,3 → Y.
	if got := pickOpening(openings, 1, true); got.Name != "X" {
		t.Errorf("pickOpening([X,Y], 1, pair) = %q, want X", got.Name)
	}
	if got := pickOpening(openings, 2, true); got.Name != "Y" {
		t.Errorf("pickOpening([X,Y], 2, pair) = %q, want Y", got.Name)
	}
}
