package pgn

import (
	"strings"
	"testing"
)

func TestGameStringDefaultsAndRosterOrder(t *testing.T) {
	game := NewGame()
	output := game.String()

	roster := []string{TagEvent, TagSite, TagDate, TagRound, TagWhite, TagBlack, TagResult}
	expected := map[string]string{
		TagEvent:  "?",
		TagSite:   "?",
		TagDate:   "????.??.??",
		TagRound:  "?",
		TagWhite:  "?",
		TagBlack:  "?",
		TagResult: "*",
	}

	lastIndex := -1
	for _, tag := range roster {
		line := "[" + tag + " \"" + expected[tag] + "\"]"
		idx := strings.Index(output, line)
		if idx < 0 {
			t.Fatalf("output missing default tag line %q", line)
		}
		if idx <= lastIndex {
			t.Errorf("tag %q at index %d not strictly after previous tag at index %d; roster order violated", tag, idx, lastIndex)
		}
		lastIndex = idx
	}
}

func TestNagToSymbolFullTable(t *testing.T) {
	cases := map[int]string{
		1:  "!",
		2:  "?",
		3:  "!!",
		4:  "??",
		5:  "!?",
		6:  "?!",
		7:  "$7",
		0:  "$0",
		-1: "$-1",
	}
	for nag, want := range cases {
		if got := nagToSymbol(nag); got != want {
			t.Errorf("nagToSymbol(%d) = %q, want %q", nag, got, want)
		}
	}
}

func TestGameStringRendersNAGs(t *testing.T) {
	game := NewGame()
	game.AddMove("e4").NAGs = []int{1}
	game.AddMove("e5").NAGs = []int{4, 7}

	output := game.String()
	if !strings.Contains(output, "1. e4! e5??$7 *") {
		t.Errorf("output missing NAG-rendered move text:\n%s", output)
	}
}

func TestGameStringMoveWithoutNAGs(t *testing.T) {
	game := NewGame()
	game.AddMove("Nf3")

	output := game.String()
	if !strings.Contains(output, "1. Nf3 *") {
		t.Errorf("output missing plain move text:\n%s", output)
	}
	if strings.Contains(output, "Nf3$") || strings.Contains(output, "Nf3{}") || strings.Contains(output, "$0") {
		t.Errorf("plain move rendered with stray NAG or bracket artifact:\n%s", output)
	}
}

func TestGameStringOverriddenTagsRespectRosterOrder(t *testing.T) {
	game := NewGame()
	game.Tags[TagWhite] = "Alice"
	game.Tags[TagEvent] = "Superbet Chess Classic"

	output := game.String()

	eventIdx := strings.Index(output, `[Event "Superbet Chess Classic"]`)
	whiteIdx := strings.Index(output, `[White "Alice"]`)
	if eventIdx < 0 {
		t.Fatalf("output missing overridden Event tag:\n%s", output)
	}
	if whiteIdx < 0 {
		t.Fatalf("output missing overridden White tag:\n%s", output)
	}
	if eventIdx >= whiteIdx {
		t.Errorf("Event tag at index %d not before White tag at index %d; roster order violated:\n%s", eventIdx, whiteIdx, output)
	}
}
