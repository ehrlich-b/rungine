// Package pgn provides PGN (Portable Game Notation) parsing and generation.
package pgn

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// Standard 7-tag roster.
const (
	TagEvent  = "Event"
	TagSite   = "Site"
	TagDate   = "Date"
	TagRound  = "Round"
	TagWhite  = "White"
	TagBlack  = "Black"
	TagResult = "Result"
)

// Game results.
const (
	ResultWhiteWins = "1-0"
	ResultBlackWins = "0-1"
	ResultDraw      = "1/2-1/2"
	ResultOngoing   = "*"
)

var (
	ErrInvalidPGN      = errors.New("invalid PGN")
	ErrUnexpectedToken = errors.New("unexpected token")
	ErrUnmatchedParen  = errors.New("unmatched parenthesis")
)

// Game represents a parsed PGN game.
type Game struct {
	Tags   map[string]string // Tag pairs
	Moves  *MoveNode         // Root of move tree (contains first move as Next)
	Result string            // Game result
}

// MoveNode represents a node in the move tree.
type MoveNode struct {
	Move       string      // SAN notation (e.g., "e4", "Nxf7+", "O-O-O")
	Comment    string      // Text annotation after the move
	NAGs       []int       // Numeric Annotation Glyphs ($1, $2, etc.)
	Variations []*MoveNode // Alternative continuations
	Next       *MoveNode   // Main line continuation
	Parent     *MoveNode   // For navigation back
	Ply        int         // Half-move number (0 = before first move)
}

// TokenType represents PGN token types.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenTag           // [Event "..."]
	TokenMove          // e4, Nf3, O-O
	TokenComment       // {text} or ;text
	TokenNAG           // $1, !, ?, !!, ??, !?, ?!
	TokenVariationStart // (
	TokenVariationEnd   // )
	TokenResult        // 1-0, 0-1, 1/2-1/2, *
	TokenMoveNumber    // 1., 1...
)

// Token represents a PGN token.
type Token struct {
	Type  TokenType
	Value string
	Line  int
	Col   int
}

// Tokenizer breaks PGN input into tokens.
type Tokenizer struct {
	reader *bufio.Reader
	line   int
	col    int
	peeked *Token
}

// NewTokenizer creates a new PGN tokenizer.
func NewTokenizer(r io.Reader) *Tokenizer {
	return &Tokenizer{
		reader: bufio.NewReader(r),
		line:   1,
		col:    1,
	}
}

// Next returns the next token.
func (t *Tokenizer) Next() (Token, error) {
	if t.peeked != nil {
		tok := *t.peeked
		t.peeked = nil
		return tok, nil
	}
	return t.scanToken()
}

// Peek returns the next token without consuming it.
func (t *Tokenizer) Peek() (Token, error) {
	if t.peeked != nil {
		return *t.peeked, nil
	}
	tok, err := t.scanToken()
	if err != nil {
		return tok, err
	}
	t.peeked = &tok
	return tok, nil
}

func (t *Tokenizer) scanToken() (Token, error) {
	t.skipWhitespace()

	line, col := t.line, t.col
	r, err := t.peek()
	if err == io.EOF {
		return Token{Type: TokenEOF, Line: line, Col: col}, nil
	}
	if err != nil {
		return Token{}, err
	}

	switch {
	case r == '[':
		return t.scanTag()
	case r == '{':
		return t.scanBraceComment()
	case r == ';':
		return t.scanLineComment()
	case r == '(':
		t.read()
		return Token{Type: TokenVariationStart, Value: "(", Line: line, Col: col}, nil
	case r == ')':
		t.read()
		return Token{Type: TokenVariationEnd, Value: ")", Line: line, Col: col}, nil
	case r == '$':
		return t.scanNAG()
	case r == '!' || r == '?':
		return t.scanSymbolicNAG()
	case r == '*':
		t.read()
		return Token{Type: TokenResult, Value: "*", Line: line, Col: col}, nil
	case r == '1' || r == '0':
		return t.scanMoveNumberOrResult()
	case unicode.IsLetter(r) || r == 'O':
		return t.scanMove()
	default:
		t.read()
		return t.scanToken() // Skip unknown characters
	}
}

func (t *Tokenizer) scanTag() (Token, error) {
	line, col := t.line, t.col
	t.read() // consume '['

	var sb strings.Builder
	sb.WriteByte('[')

	for {
		r, err := t.read()
		if err != nil {
			return Token{}, fmt.Errorf("%w: unclosed tag", ErrInvalidPGN)
		}
		sb.WriteRune(r)
		if r == ']' {
			break
		}
	}

	return Token{Type: TokenTag, Value: sb.String(), Line: line, Col: col}, nil
}

func (t *Tokenizer) scanBraceComment() (Token, error) {
	line, col := t.line, t.col
	t.read() // consume '{'

	var sb strings.Builder
	depth := 1

	for depth > 0 {
		r, err := t.read()
		if err != nil {
			return Token{}, fmt.Errorf("%w: unclosed comment", ErrInvalidPGN)
		}
		if r == '{' {
			depth++
			sb.WriteRune(r)
		} else if r == '}' {
			depth--
			if depth > 0 {
				sb.WriteRune(r)
			}
		} else {
			sb.WriteRune(r)
		}
	}

	return Token{Type: TokenComment, Value: strings.TrimSpace(sb.String()), Line: line, Col: col}, nil
}

func (t *Tokenizer) scanLineComment() (Token, error) {
	line, col := t.line, t.col
	t.read() // consume ';'

	var sb strings.Builder
	for {
		r, err := t.peek()
		if err == io.EOF || r == '\n' {
			break
		}
		t.read()
		sb.WriteRune(r)
	}

	return Token{Type: TokenComment, Value: strings.TrimSpace(sb.String()), Line: line, Col: col}, nil
}

func (t *Tokenizer) scanNAG() (Token, error) {
	line, col := t.line, t.col
	t.read() // consume '$'

	var sb strings.Builder
	sb.WriteByte('$')

	for {
		r, err := t.peek()
		if err != nil || !unicode.IsDigit(r) {
			break
		}
		t.read()
		sb.WriteRune(r)
	}

	return Token{Type: TokenNAG, Value: sb.String(), Line: line, Col: col}, nil
}

func (t *Tokenizer) scanSymbolicNAG() (Token, error) {
	line, col := t.line, t.col

	var sb strings.Builder
	for {
		r, err := t.peek()
		if err != nil || (r != '!' && r != '?') {
			break
		}
		t.read()
		sb.WriteRune(r)
	}

	return Token{Type: TokenNAG, Value: sb.String(), Line: line, Col: col}, nil
}

func (t *Tokenizer) scanMoveNumberOrResult() (Token, error) {
	line, col := t.line, t.col

	var sb strings.Builder
	for {
		r, err := t.peek()
		if err != nil {
			break
		}
		if unicode.IsDigit(r) || r == '.' || r == '/' || r == '-' {
			t.read()
			sb.WriteRune(r)
		} else {
			break
		}
	}

	value := sb.String()

	// Check for result
	if value == "1-0" || value == "0-1" || value == "1/2-1/2" {
		return Token{Type: TokenResult, Value: value, Line: line, Col: col}, nil
	}

	// Must be move number
	return Token{Type: TokenMoveNumber, Value: value, Line: line, Col: col}, nil
}

func (t *Tokenizer) scanMove() (Token, error) {
	line, col := t.line, t.col

	var sb strings.Builder
	for {
		r, err := t.peek()
		if err != nil {
			break
		}
		// Valid move characters: letters, digits, -, +, #, =, x
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '+' || r == '#' || r == '=' || r == 'x' {
			t.read()
			sb.WriteRune(r)
		} else {
			break
		}
	}

	return Token{Type: TokenMove, Value: sb.String(), Line: line, Col: col}, nil
}

func (t *Tokenizer) skipWhitespace() {
	for {
		r, err := t.peek()
		if err != nil || !unicode.IsSpace(r) {
			return
		}
		t.read()
	}
}

func (t *Tokenizer) peek() (rune, error) {
	r, _, err := t.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	t.reader.UnreadRune()
	return r, nil
}

func (t *Tokenizer) read() (rune, error) {
	r, _, err := t.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	if r == '\n' {
		t.line++
		t.col = 1
	} else {
		t.col++
	}
	return r, nil
}

// Parser parses PGN games from tokens.
type Parser struct {
	tokenizer *Tokenizer
}

// NewParser creates a new PGN parser.
func NewParser(r io.Reader) *Parser {
	return &Parser{
		tokenizer: NewTokenizer(r),
	}
}

// ParseGame parses a single game.
func (p *Parser) ParseGame() (*Game, error) {
	game := &Game{
		Tags: make(map[string]string),
	}

	// Parse tag section
	for {
		tok, err := p.tokenizer.Peek()
		if err != nil {
			return nil, err
		}
		if tok.Type != TokenTag {
			break
		}
		p.tokenizer.Next()
		name, value := parseTag(tok.Value)
		game.Tags[name] = value
	}

	// Parse movetext
	root := &MoveNode{Ply: 0}
	current := root

	var variationStack []*MoveNode

	for {
		tok, err := p.tokenizer.Next()
		if err != nil {
			return nil, err
		}

		switch tok.Type {
		case TokenEOF:
			game.Moves = root
			if game.Result == "" {
				game.Result = game.Tags[TagResult]
			}
			return game, nil

		case TokenResult:
			game.Result = tok.Value
			game.Moves = root
			return game, nil

		case TokenMoveNumber:
			// Skip move numbers
			continue

		case TokenMove:
			node := &MoveNode{
				Move:   tok.Value,
				Parent: current,
				Ply:    current.Ply + 1,
			}
			current.Next = node
			current = node

		case TokenComment:
			if current != root {
				current.Comment = tok.Value
			}

		case TokenNAG:
			if current != root {
				nag := parseNAG(tok.Value)
				if nag > 0 {
					current.NAGs = append(current.NAGs, nag)
				}
			}

		case TokenVariationStart:
			// Save position to return to after variation
			variationStack = append(variationStack, current)
			// Branch from parent of current move (variation starts from same position)
			if current.Parent != nil {
				// Create a new branch point - next move in variation goes as child of parent
				branchPoint := current.Parent
				// Create placeholder for variation root
				varRoot := &MoveNode{Parent: branchPoint, Ply: branchPoint.Ply}
				branchPoint.Variations = append(branchPoint.Variations, varRoot)
				current = varRoot
			}

		case TokenVariationEnd:
			if len(variationStack) == 0 {
				return nil, ErrUnmatchedParen
			}
			// Pop from stack - return to saved position
			current = variationStack[len(variationStack)-1]
			variationStack = variationStack[:len(variationStack)-1]

		case TokenTag:
			// Tag in movetext starts new game
			game.Moves = root
			if game.Result == "" {
				game.Result = game.Tags[TagResult]
			}
			return game, nil
		}
	}
}

// parseTag extracts name and value from a tag token like "[Event "World Championship"]".
func parseTag(s string) (string, string) {
	// Remove brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")

	// Split at first space
	parts := strings.SplitN(s, " ", 2)
	if len(parts) < 2 {
		return parts[0], ""
	}

	name := parts[0]
	value := strings.Trim(parts[1], "\"")
	return name, value
}

// parseNAG converts NAG string to integer.
func parseNAG(s string) int {
	// Handle symbolic NAGs
	switch s {
	case "!":
		return 1
	case "?":
		return 2
	case "!!":
		return 3
	case "??":
		return 4
	case "!?":
		return 5
	case "?!":
		return 6
	}

	// Handle numeric NAGs ($N)
	if len(s) > 1 && s[0] == '$' {
		if n, err := strconv.Atoi(s[1:]); err == nil {
			return n
		}
	}

	return 0
}

// String returns the PGN representation of a game.
func (g *Game) String() string {
	var sb strings.Builder

	// Write tags
	roster := []string{TagEvent, TagSite, TagDate, TagRound, TagWhite, TagBlack, TagResult}
	for _, tag := range roster {
		value, ok := g.Tags[tag]
		if !ok {
			value = "?"
		}
		fmt.Fprintf(&sb, "[%s \"%s\"]\n", tag, value)
	}

	// Write other tags
	for name, value := range g.Tags {
		found := false
		for _, tag := range roster {
			if name == tag {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(&sb, "[%s \"%s\"]\n", name, value)
		}
	}

	sb.WriteByte('\n')

	// Write moves
	if g.Moves != nil && g.Moves.Next != nil {
		g.writeMoves(&sb, g.Moves.Next, true)
	}

	// Write result
	if g.Result != "" {
		sb.WriteString(g.Result)
	}

	return sb.String()
}

func (g *Game) writeMoves(sb *strings.Builder, node *MoveNode, needNumber bool) {
	for node != nil {
		// Write move number if needed
		if needNumber || node.Ply%2 == 1 {
			moveNum := (node.Ply + 1) / 2
			if node.Ply%2 == 1 {
				fmt.Fprintf(sb, "%d. ", moveNum)
			} else {
				fmt.Fprintf(sb, "%d... ", moveNum)
			}
			needNumber = false
		}

		// Write move
		sb.WriteString(node.Move)

		// Write NAGs
		for _, nag := range node.NAGs {
			sb.WriteString(nagToSymbol(nag))
		}

		// Write comment
		if node.Comment != "" {
			fmt.Fprintf(sb, " {%s}", node.Comment)
		}

		// Write variations
		for _, variation := range node.Variations {
			sb.WriteString(" (")
			g.writeMoves(sb, variation, true)
			sb.WriteByte(')')
		}

		sb.WriteByte(' ')
		node = node.Next
	}
}

func nagToSymbol(nag int) string {
	switch nag {
	case 1:
		return "!"
	case 2:
		return "?"
	case 3:
		return "!!"
	case 4:
		return "??"
	case 5:
		return "!?"
	case 6:
		return "?!"
	default:
		return fmt.Sprintf("$%d", nag)
	}
}

// MainLine returns the moves of the main line as a slice.
func (g *Game) MainLine() []string {
	var moves []string
	node := g.Moves
	if node != nil {
		node = node.Next
	}
	for node != nil {
		moves = append(moves, node.Move)
		node = node.Next
	}
	return moves
}

// NewGame creates a new empty game.
func NewGame() *Game {
	return &Game{
		Tags: map[string]string{
			TagEvent:  "?",
			TagSite:   "?",
			TagDate:   "????.??.??",
			TagRound:  "?",
			TagWhite:  "?",
			TagBlack:  "?",
			TagResult: "*",
		},
		Moves:  &MoveNode{Ply: 0},
		Result: "*",
	}
}

// AddMove adds a move to the end of the main line.
func (g *Game) AddMove(san string) *MoveNode {
	// Find end of main line
	node := g.Moves
	for node.Next != nil {
		node = node.Next
	}

	newNode := &MoveNode{
		Move:   san,
		Parent: node,
		Ply:    node.Ply + 1,
	}
	node.Next = newNode
	return newNode
}
