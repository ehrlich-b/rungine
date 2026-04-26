package tournament

import (
	"fmt"
	"io"

	"rungine/internal/chess"
	"rungine/internal/pgn"
)

// LoadOpeningsFromPGN reads a stream of PGN games from r and returns
// one Opening per game whose mainline is at least samplePly half-moves
// long. The opening's StartMoves contains the first samplePly moves in
// UCI form so the arbiter can apply them with PushUCI.
//
// Games shorter than samplePly are skipped silently (typical
// tournament books contain a few short games which are uninteresting
// as openings). samplePly counts in plies, not full moves: 16 plies =
// 8 moves by each side.
//
// The returned slice is in PGN file order. Tag values White/Black are
// not used; only the Event tag (if present) becomes Opening.Name so
// books can label their lines (most books leave it blank).
func LoadOpeningsFromPGN(r io.Reader, samplePly int) ([]Opening, error) {
	if samplePly < 1 {
		return nil, fmt.Errorf("openings: samplePly must be >= 1, got %d", samplePly)
	}

	var openings []Opening
	parser := pgn.NewParser(r)
	for {
		game, err := parser.ParseGame()
		if err == io.EOF {
			break
		}
		if err != nil {
			return openings, fmt.Errorf("openings: parse PGN: %w", err)
		}
		if game == nil {
			break
		}
		// Parser returns an empty Game at end of stream; detect and stop.
		if len(game.Tags) == 0 && (game.Moves == nil || game.Moves.Next == nil) {
			break
		}

		mainline := game.MainLine()
		if len(mainline) < samplePly {
			continue
		}

		cg := chess.NewGame()
		uciMoves := make([]string, 0, samplePly)
		ok := true
		for i := range samplePly {
			if err := cg.PushSAN(mainline[i]); err != nil {
				ok = false
				break
			}
			uciHistory := cg.MovesUCI()
			uciMoves = append(uciMoves, uciHistory[len(uciHistory)-1])
		}
		if !ok {
			continue
		}

		name := game.Tags["Opening"]
		if name == "" {
			name = game.Tags["Event"]
		}
		openings = append(openings, Opening{Name: name, StartMoves: uciMoves})
	}
	return openings, nil
}
