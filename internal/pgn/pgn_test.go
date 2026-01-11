package pgn

import (
	"strings"
	"testing"
)

func TestTokenizer(t *testing.T) {
	input := `[Event "Test Game"]
[White "Player 1"]
[Black "Player 2"]
[Result "1-0"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 {Ruy Lopez} a6 1-0`

	tokenizer := NewTokenizer(strings.NewReader(input))

	// Just verify we can tokenize without errors and get expected types
	var tokens []Token
	for {
		tok, err := tokenizer.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}

	// Check we got tags
	tagCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenTag {
			tagCount++
		}
	}
	if tagCount != 4 {
		t.Errorf("expected 4 tags, got %d", tagCount)
	}

	// Check we got moves
	moveCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenMove {
			moveCount++
		}
	}
	if moveCount != 6 { // e4, e5, Nf3, Nc6, Bb5, a6
		t.Errorf("expected 6 moves, got %d", moveCount)
	}

	// Check we got the comment
	commentCount := 0
	for _, tok := range tokens {
		if tok.Type == TokenComment && tok.Value == "Ruy Lopez" {
			commentCount++
		}
	}
	if commentCount != 1 {
		t.Errorf("expected 1 'Ruy Lopez' comment, got %d", commentCount)
	}

	// Check we got the result
	resultFound := false
	for _, tok := range tokens {
		if tok.Type == TokenResult && tok.Value == "1-0" {
			resultFound = true
		}
	}
	if !resultFound {
		t.Error("expected result '1-0' not found")
	}
}

func TestParseSimpleGame(t *testing.T) {
	input := `[Event "Test"]
[Site "Test Site"]
[Date "2024.01.15"]
[Round "1"]
[White "Alice"]
[Black "Bob"]
[Result "1-0"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 1-0`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	// Check tags
	if game.Tags[TagEvent] != "Test" {
		t.Errorf("Event = %q, want %q", game.Tags[TagEvent], "Test")
	}
	if game.Tags[TagWhite] != "Alice" {
		t.Errorf("White = %q, want %q", game.Tags[TagWhite], "Alice")
	}
	if game.Tags[TagBlack] != "Bob" {
		t.Errorf("Black = %q, want %q", game.Tags[TagBlack], "Bob")
	}

	// Check result
	if game.Result != "1-0" {
		t.Errorf("Result = %q, want %q", game.Result, "1-0")
	}

	// Check main line
	mainLine := game.MainLine()
	expected := []string{"e4", "e5", "Nf3", "Nc6", "Bb5"}
	if len(mainLine) != len(expected) {
		t.Fatalf("MainLine() len = %d, want %d", len(mainLine), len(expected))
	}
	for i, move := range expected {
		if mainLine[i] != move {
			t.Errorf("MainLine()[%d] = %q, want %q", i, mainLine[i], move)
		}
	}
}

func TestParseWithComments(t *testing.T) {
	input := `[Event "Commented Game"]
[Result "*"]

1. e4 {King's pawn opening} e5 2. Nf3 {Knight development} *`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	// Find e4 move
	node := game.Moves.Next
	if node == nil || node.Move != "e4" {
		t.Fatal("Expected first move to be e4")
	}
	if node.Comment != "King's pawn opening" {
		t.Errorf("e4 comment = %q, want %q", node.Comment, "King's pawn opening")
	}

	// Find Nf3 move
	node = node.Next.Next // e4 -> e5 -> Nf3
	if node == nil || node.Move != "Nf3" {
		t.Fatal("Expected third move to be Nf3")
	}
	if node.Comment != "Knight development" {
		t.Errorf("Nf3 comment = %q, want %q", node.Comment, "Knight development")
	}
}

func TestParseWithNAGs(t *testing.T) {
	input := `[Event "NAG Test"]
[Result "*"]

1. e4! e5? 2. Nf3!! Nc6?? 3. Bb5!? a6?! 4. Ba4 $10 *`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	tests := []struct {
		move string
		nags []int
	}{
		{"e4", []int{1}},    // !
		{"e5", []int{2}},    // ?
		{"Nf3", []int{3}},   // !!
		{"Nc6", []int{4}},   // ??
		{"Bb5", []int{5}},   // !?
		{"a6", []int{6}},    // ?!
		{"Ba4", []int{10}},  // $10
	}

	node := game.Moves.Next
	for _, tc := range tests {
		if node == nil {
			t.Fatalf("Expected move %s but reached end", tc.move)
		}
		if node.Move != tc.move {
			t.Errorf("Move = %q, want %q", node.Move, tc.move)
		}
		if len(node.NAGs) != len(tc.nags) {
			t.Errorf("%s: NAGs len = %d, want %d", tc.move, len(node.NAGs), len(tc.nags))
		} else {
			for i, nag := range tc.nags {
				if node.NAGs[i] != nag {
					t.Errorf("%s: NAG[%d] = %d, want %d", tc.move, i, node.NAGs[i], nag)
				}
			}
		}
		node = node.Next
	}
}

func TestParseWithVariations(t *testing.T) {
	input := `[Event "Variation Test"]
[Result "*"]

1. e4 e5 (1... c5 2. Nf3) 2. Nf3 *`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	// Main line: e4, e5, Nf3
	mainLine := game.MainLine()
	if len(mainLine) != 3 {
		t.Fatalf("MainLine() len = %d, want 3", len(mainLine))
	}
	if mainLine[0] != "e4" || mainLine[1] != "e5" || mainLine[2] != "Nf3" {
		t.Errorf("MainLine() = %v, want [e4 e5 Nf3]", mainLine)
	}
}

func TestParseResults(t *testing.T) {
	tests := []struct {
		input  string
		result string
	}{
		{`[Result "1-0"] 1. e4 1-0`, "1-0"},
		{`[Result "0-1"] 1. e4 0-1`, "0-1"},
		{`[Result "1/2-1/2"] 1. e4 1/2-1/2`, "1/2-1/2"},
		{`[Result "*"] 1. e4 *`, "*"},
	}

	for _, tc := range tests {
		parser := NewParser(strings.NewReader(tc.input))
		game, err := parser.ParseGame()
		if err != nil {
			t.Errorf("ParseGame(%q) error: %v", tc.input, err)
			continue
		}
		if game.Result != tc.result {
			t.Errorf("ParseGame(%q).Result = %q, want %q", tc.input, game.Result, tc.result)
		}
	}
}

func TestGameString(t *testing.T) {
	game := NewGame()
	game.Tags[TagEvent] = "Test Event"
	game.Tags[TagWhite] = "Alice"
	game.Tags[TagBlack] = "Bob"
	game.Tags[TagResult] = "1-0"
	game.Result = "1-0"

	game.AddMove("e4")
	game.AddMove("e5")
	game.AddMove("Nf3")

	output := game.String()

	// Check that tags are present
	if !strings.Contains(output, `[Event "Test Event"]`) {
		t.Error("Output missing Event tag")
	}
	if !strings.Contains(output, `[White "Alice"]`) {
		t.Error("Output missing White tag")
	}

	// Check that moves are present
	if !strings.Contains(output, "1. e4") {
		t.Error("Output missing first move")
	}
	if !strings.Contains(output, "e5") {
		t.Error("Output missing second move")
	}
	if !strings.Contains(output, "2. Nf3") {
		t.Error("Output missing third move")
	}

	// Check result
	if !strings.Contains(output, "1-0") {
		t.Error("Output missing result")
	}
}

func TestLineComment(t *testing.T) {
	input := `[Event "Test"]
[Result "*"]

1. e4 ; This is a line comment
e5 *`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	mainLine := game.MainLine()
	if len(mainLine) != 2 {
		t.Errorf("MainLine() len = %d, want 2", len(mainLine))
	}
}

func TestCastling(t *testing.T) {
	input := `[Event "Castling"]
[Result "*"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. O-O O-O-O *`

	parser := NewParser(strings.NewReader(input))
	game, err := parser.ParseGame()
	if err != nil {
		t.Fatalf("ParseGame() error: %v", err)
	}

	mainLine := game.MainLine()

	// Find castling moves
	hasKingside := false
	hasQueenside := false
	for _, move := range mainLine {
		if move == "O-O" {
			hasKingside = true
		}
		if move == "O-O-O" {
			hasQueenside = true
		}
	}

	if !hasKingside {
		t.Error("Missing kingside castling (O-O)")
	}
	if !hasQueenside {
		t.Error("Missing queenside castling (O-O-O)")
	}
}

func TestNewGame(t *testing.T) {
	game := NewGame()

	// Check default tags
	if game.Tags[TagEvent] != "?" {
		t.Errorf("Event = %q, want %q", game.Tags[TagEvent], "?")
	}
	if game.Tags[TagDate] != "????.??.??" {
		t.Errorf("Date = %q, want %q", game.Tags[TagDate], "????.??.??")
	}
	if game.Result != "*" {
		t.Errorf("Result = %q, want %q", game.Result, "*")
	}

	// Check that moves root exists
	if game.Moves == nil {
		t.Error("Moves is nil")
	}
}

func TestAddMove(t *testing.T) {
	game := NewGame()

	node1 := game.AddMove("e4")
	if node1.Move != "e4" {
		t.Errorf("First move = %q, want %q", node1.Move, "e4")
	}
	if node1.Ply != 1 {
		t.Errorf("First move ply = %d, want 1", node1.Ply)
	}

	node2 := game.AddMove("e5")
	if node2.Move != "e5" {
		t.Errorf("Second move = %q, want %q", node2.Move, "e5")
	}
	if node2.Ply != 2 {
		t.Errorf("Second move ply = %d, want 2", node2.Ply)
	}

	mainLine := game.MainLine()
	if len(mainLine) != 2 {
		t.Errorf("MainLine() len = %d, want 2", len(mainLine))
	}
}
