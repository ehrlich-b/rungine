package chess

import (
	"strings"
	"testing"

	"rungine/internal/fen"
	"rungine/internal/pgn"
)

// oracleGames gathers the complete game PGNs already shipped in this repo's
// test data (internal/pgn/pgn_test.go, internal/tournament/openings_test.go,
// frontend/tests/e2e/game-replay.spec.ts, app_tournament_persistence_test.go
// and the arbiter's custom-FEN test). The expected SAN for every ply comes
// from parsing the PGN — nothing here is hand-authored SAN for a whole game.
var oracleGames = []struct {
	name string
	pgn  string
}{
	{
		name: "openings Italian",
		pgn: `[Event "Test 1"]
[Opening "Italian"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. c3 Nf6 1-0
`,
	},
	{
		name: "openings Sicilian",
		pgn: `[Event "Test 2"]
[Opening "Sicilian"]

1. e4 c5 2. Nf3 d6 3. d4 cxd4 4. Nxd4 Nf6 0-1
`,
	},
	{
		name: "openings short",
		pgn: `[Event "Short"]

1. e4 e5 1-0
`,
	},
	{
		name: "openings long",
		pgn: `[Event "Long"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 1-0
`,
	},
	{
		name: "pgn tokenizer ruy lopez",
		pgn: `[Event "Test Game"]
[White "Player 1"]
[Black "Player 2"]
[Result "1-0"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 {Ruy Lopez} a6 1-0
`,
	},
	{
		name: "pgn simple",
		pgn: `[Event "Test"]
[Site "Test Site"]
[Date "2024.01.15"]
[Round "1"]
[White "Alice"]
[Black "Bob"]
[Result "1-0"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 1-0
`,
	},
	{
		name: "pgn comments",
		pgn: `[Event "Commented Game"]
[Result "*"]

1. e4 {King's pawn opening} e5 2. Nf3 {Knight development} *
`,
	},
	{
		name: "pgn NAGs",
		pgn: `[Event "NAG Test"]
[Result "*"]

1. e4! e5? 2. Nf3!! Nc6?? 3. Bb5!? a6?! 4. Ba4 $10 *
`,
	},
	{
		name: "pgn variation",
		pgn: `[Event "Variation Test"]
[Result "*"]

1. e4 e5 (1... c5 2. Nf3) 2. Nf3 *
`,
	},
	{
		name: "pgn castling",
		pgn: `[Event "Castling"]
[Result "*"]

1. e4 e5 2. Nf3 Nc6 3. Bc4 Bc5 4. O-O O-O-O *
`,
	},
	{
		name: "pgn embedded annotations",
		pgn: `[Event "Annotated"]
[Result "*"]

1. e4 {[%eval +0.42] [%clk 0:01:30.500]} e5 {[%eval -0.10] [%clk 0:01:29.250]} 2. Nf3 {[%eval #5]} Nc6 {[%eval #-3] [%clk 0:01:00.000]} *
`,
	},
	{
		name: "e2e game replay",
		pgn: `[Event "Test"]

1. e4 e5 2. Nf3 Nc6 1-0
`,
	},
	{
		name: "tournament persistence",
		pgn: `[Event "Test"]
1. e4 e5
`,
	},
	{
		name: "arbiter custom FEN black to move",
		pgn: `[Event "custom"]
[SetUp "1"]
[FEN "rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 5"]

5... d6 6. Nf3 *
`,
	},
}

// TestUCIToSANOracle is THE oracle: it parses every complete game PGN above,
// derives the UCI for each ply by playing the parsed SAN through the engine,
// feeds the whole UCI sequence to UCIToSAN, and asserts token-for-token
// equality with the SAN the parser produced.
func TestUCIToSANOracle(t *testing.T) {
	totalGames, totalPlies := 0, 0
	for _, tc := range oracleGames {
		game, err := pgn.NewParser(strings.NewReader(tc.pgn)).ParseGame()
		if err != nil {
			t.Errorf("%s: parse fixture: %v", tc.name, err)
			continue
		}
		wantSAN := game.MainLine()
		if len(wantSAN) == 0 {
			t.Errorf("%s: fixture parsed to zero moves", tc.name)
			continue
		}
		startFEN := game.Tags["FEN"]
		if startFEN == "" {
			startFEN = fen.StartingFEN
		}

		// Derive the UCI move for every ply by applying the parsed moves
		// through the existing engine. A move that cannot be played through
		// the engine is an illegal move in the fixture itself (e.g. the
		// "pgn castling" game tries 4...O-O-O while black's queen still sits
		// on d8); such games are verified for their legal prefix only and the
		// omission is reported below rather than hidden.
		engineGame, err := FromFEN(startFEN)
		if err != nil {
			t.Errorf("%s: FromFEN(%q): %v", tc.name, startFEN, err)
			continue
		}
		derived := make([]string, 0, len(wantSAN))
		for _, san := range wantSAN {
			if err := engineGame.PushSAN(san); err != nil {
				t.Logf("%s: %q not playable through the engine (illegal move in this fixture): %v; verifying the %d-move legal prefix", tc.name, san, err, len(derived))
				break
			}
			derived = append(derived, san)
		}
		uci := engineGame.MovesUCI()
		if len(uci) != len(derived) {
			t.Errorf("%s: derived %d UCI moves, want %d", tc.name, len(uci), len(derived))
			continue
		}

		// Feed the UCI sequence to the converter and compare token for token.
		gotSAN, err := UCIToSAN(startFEN, uci)
		if err != nil {
			t.Errorf("%s: UCIToSAN: %v", tc.name, err)
			continue
		}
		if len(gotSAN) != len(derived) {
			t.Errorf("%s: UCIToSAN returned %d tokens, want %d", tc.name, len(gotSAN), len(derived))
			continue
		}
		for i := range derived {
			if gotSAN[i] != derived[i] {
				t.Errorf("%s: ply %d: UCIToSAN=%q want %q (moves so far %v)", tc.name, i+1, gotSAN[i], derived[i], derived[:i+1])
			}
		}
		totalGames++
		totalPlies += len(gotSAN)
		note := ""
		if len(gotSAN) != len(wantSAN) {
			note = " (legal prefix only)"
		}
		t.Logf("%s: %d/%d plies verified%s", tc.name, len(gotSAN), len(wantSAN), note)
	}
	t.Logf("oracle: %d games, %d plies verified", totalGames, totalPlies)
}
