package tournament

import (
	"slices"
	"strings"
	"testing"
)

const twoGamesPGN = `[Event "Test 1"]
[Opening "Italian"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. c3 Nf6 1-0

[Event "Test 2"]
[Opening "Sicilian"]

1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 0-1
`

func TestLoadOpeningsFromPGNFromTwoGames(t *testing.T) {
	openings, err := LoadOpeningsFromPGN(strings.NewReader(twoGamesPGN), 6)
	if err != nil {
		t.Fatalf("LoadOpeningsFromPGN: %v", err)
	}
	if len(openings) != 2 {
		t.Fatalf("len(openings) = %d, want 2", len(openings))
	}

	// Game 1: Italian opening, 6 plies = e4 e5 Nf3 Nc6 Bc4 Bc5.
	wantItalian := []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1c4", "f8c5"}
	if !slices.Equal(openings[0].StartMoves, wantItalian) {
		t.Errorf("openings[0].StartMoves = %v, want %v", openings[0].StartMoves, wantItalian)
	}
	if openings[0].Name != "Italian" {
		t.Errorf("openings[0].Name = %q, want %q", openings[0].Name, "Italian")
	}

	// Game 2: Sicilian, 6 plies.
	wantSicilian := []string{"e2e4", "c7c5", "g1f3", "d7d6", "d2d4", "c5d4"}
	if !slices.Equal(openings[1].StartMoves, wantSicilian) {
		t.Errorf("openings[1].StartMoves = %v, want %v", openings[1].StartMoves, wantSicilian)
	}
	if openings[1].Name != "Sicilian" {
		t.Errorf("openings[1].Name = %q, want %q", openings[1].Name, "Sicilian")
	}
}

func TestLoadOpeningsFromPGNSkipsShortGames(t *testing.T) {
	pgn := `[Event "Short"]

1. e4 e5 1-0

[Event "Long"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 1-0
`
	openings, err := LoadOpeningsFromPGN(strings.NewReader(pgn), 6)
	if err != nil {
		t.Fatalf("LoadOpeningsFromPGN: %v", err)
	}
	if len(openings) != 1 {
		t.Fatalf("len(openings) = %d, want 1 (short game skipped)", len(openings))
	}
	if openings[0].Name != "Long" {
		t.Errorf("openings[0].Name = %q, want %q", openings[0].Name, "Long")
	}
}

func TestLoadOpeningsFromPGNFallsBackToEvent(t *testing.T) {
	// No Opening tag; loader uses Event as the opening name.
	pgn := `[Event "Friendly"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 1-0
`
	openings, err := LoadOpeningsFromPGN(strings.NewReader(pgn), 6)
	if err != nil {
		t.Fatalf("LoadOpeningsFromPGN: %v", err)
	}
	if len(openings) != 1 || openings[0].Name != "Friendly" {
		t.Errorf("openings = %+v, want one with Name=Friendly", openings)
	}
}

func TestLoadOpeningsFromPGNRejectsBadSamplePly(t *testing.T) {
	if _, err := LoadOpeningsFromPGN(strings.NewReader(""), 0); err == nil {
		t.Error("samplePly=0 should error")
	}
	if _, err := LoadOpeningsFromPGN(strings.NewReader(""), -3); err == nil {
		t.Error("negative samplePly should error")
	}
}

func TestLoadOpeningsFromPGNEmptyInput(t *testing.T) {
	openings, err := LoadOpeningsFromPGN(strings.NewReader(""), 4)
	if err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if len(openings) != 0 {
		t.Errorf("len(openings) = %d, want 0", len(openings))
	}
}

func TestLoadOpeningsFromPGNUsableInBuildMatch(t *testing.T) {
	// End-to-end: load openings from PGN, hand them to BuildMatch with
	// PairMode, confirm StartMoves propagate to pairings.
	openings, err := LoadOpeningsFromPGN(strings.NewReader(twoGamesPGN), 4)
	if err != nil {
		t.Fatalf("LoadOpeningsFromPGN: %v", err)
	}
	pairings := BuildMatch(MatchSpec{
		White:    EngineSpec{Name: "A"},
		Black:    EngineSpec{Name: "B"},
		Games:    4,
		Openings: openings,
		PairMode: true,
	})
	if len(pairings) != 4 {
		t.Fatalf("len(pairings) = %d, want 4", len(pairings))
	}
	// Pair mode: games 0 and 1 share opening 0 (Italian); games 2 and 3
	// share opening 1 (Sicilian).
	if !slices.Equal(pairings[0].StartMoves, pairings[1].StartMoves) {
		t.Errorf("pair-mode games 1-2 should share an opening")
	}
	if !slices.Equal(pairings[2].StartMoves, pairings[3].StartMoves) {
		t.Errorf("pair-mode games 3-4 should share an opening")
	}
	if slices.Equal(pairings[0].StartMoves, pairings[2].StartMoves) {
		t.Errorf("pair-mode games 1 and 3 should have different openings")
	}
}
