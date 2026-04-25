package tournament

import "strconv"

// Opening describes a starting position for a game. Empty StartFEN
// means the standard starting position; StartMoves are applied on top.
type Opening struct {
	Name       string
	StartFEN   string
	StartMoves []string
}

// pickOpening returns the opening to use for game index gameIdx (0-based).
// In pair mode, two consecutive games share an opening (so colors can be
// flipped on the same line). With no openings configured, it returns
// the zero value (standard startpos).
func pickOpening(openings []Opening, gameIdx int, pairMode bool) Opening {
	if len(openings) == 0 {
		return Opening{}
	}
	idx := gameIdx
	if pairMode {
		idx = gameIdx / 2
	}
	return openings[idx%len(openings)]
}

// MatchSpec configures BuildMatch.
type MatchSpec struct {
	// White and Black name the two engines. White plays the white side
	// of game 1; colors alternate each game.
	White EngineSpec
	Black EngineSpec

	// Games is the total number of games to play (>= 1).
	Games int

	// Openings, if non-empty, are rotated through and assigned to games.
	// In PairMode each opening is used twice (once for each color).
	Openings []Opening
	PairMode bool

	// StartGameNumber is the GameNumber to assign to the first game.
	// Defaults to 1.
	StartGameNumber int
}

// BuildMatch returns the pairings for a two-engine match. Colors
// alternate each game so White plays white in game 1, black in game 2,
// and so on. Round tags are set to the 1-based game number.
func BuildMatch(spec MatchSpec) []Pairing {
	if spec.Games < 1 {
		return nil
	}
	if spec.StartGameNumber < 1 {
		spec.StartGameNumber = 1
	}
	pairings := make([]Pairing, 0, spec.Games)
	for g := range spec.Games {
		w, b := spec.White, spec.Black
		if g%2 == 1 {
			w, b = spec.Black, spec.White
		}
		opening := pickOpening(spec.Openings, g, spec.PairMode)
		gameNum := spec.StartGameNumber + g
		pairings = append(pairings, Pairing{
			GameNumber: gameNum,
			Round:      strconv.Itoa(gameNum),
			White:      w,
			Black:      b,
			StartFEN:   opening.StartFEN,
			StartMoves: opening.StartMoves,
		})
	}
	return pairings
}

// RoundRobinSpec configures BuildRoundRobin.
type RoundRobinSpec struct {
	// Engines is the field; each pair plays both directions (each
	// engine takes white once against every other engine per cycle).
	Engines []EngineSpec

	// Cycles is the number of times the full round-robin is repeated.
	// Defaults to 1.
	Cycles int

	// Openings cycle across games. In PairMode each opening is used
	// twice — once when (i, j) plays it, then again when (j, i) plays
	// the same line with colors flipped.
	Openings []Opening
	PairMode bool

	// StartGameNumber is the GameNumber assigned to the first game.
	// Defaults to 1.
	StartGameNumber int
}

// BuildRoundRobin returns pairings for a round-robin: every ordered
// pair of distinct engines plays once per cycle. With N engines and C
// cycles, the result has N*(N-1)*C games. The Round tag holds the cycle
// number (1-based).
func BuildRoundRobin(spec RoundRobinSpec) []Pairing {
	if len(spec.Engines) < 2 {
		return nil
	}
	cycles := max(spec.Cycles, 1)
	if spec.StartGameNumber < 1 {
		spec.StartGameNumber = 1
	}

	n := len(spec.Engines)
	totalGames := n * (n - 1) * cycles
	pairings := make([]Pairing, 0, totalGames)

	gameIdx := 0
	for c := range cycles {
		for i := range n {
			for j := range n {
				if i == j {
					continue
				}
				opening := pickOpening(spec.Openings, gameIdx, spec.PairMode)
				pairings = append(pairings, Pairing{
					GameNumber: spec.StartGameNumber + gameIdx,
					Round:      strconv.Itoa(c + 1),
					White:      spec.Engines[i],
					Black:      spec.Engines[j],
					StartFEN:   opening.StartFEN,
					StartMoves: opening.StartMoves,
				})
				gameIdx++
			}
		}
	}
	return pairings
}

// GauntletSpec configures BuildGauntlet.
type GauntletSpec struct {
	// Challenger plays against everyone in Field. Within each pairing
	// the challenger and opponent split games equally — colors
	// alternate the same way as in a match.
	Challenger EngineSpec
	Field      []EngineSpec

	// GamesPerOpponent is the number of games the challenger plays
	// against each opponent (>= 1).
	GamesPerOpponent int

	Openings []Opening
	PairMode bool

	StartGameNumber int
}

// BuildGauntlet returns pairings for a gauntlet: the challenger plays
// GamesPerOpponent games against each opponent in Field. Colors
// alternate within each opponent's series.
func BuildGauntlet(spec GauntletSpec) []Pairing {
	if spec.GamesPerOpponent < 1 || len(spec.Field) == 0 {
		return nil
	}
	if spec.StartGameNumber < 1 {
		spec.StartGameNumber = 1
	}

	pairings := make([]Pairing, 0, spec.GamesPerOpponent*len(spec.Field))
	gameIdx := 0
	for _, opp := range spec.Field {
		for g := range spec.GamesPerOpponent {
			w, b := spec.Challenger, opp
			if g%2 == 1 {
				w, b = opp, spec.Challenger
			}
			opening := pickOpening(spec.Openings, gameIdx, spec.PairMode)
			pairings = append(pairings, Pairing{
				GameNumber: spec.StartGameNumber + gameIdx,
				Round:      strconv.Itoa(gameIdx + 1),
				White:      w,
				Black:      b,
				StartFEN:   opening.StartFEN,
				StartMoves: opening.StartMoves,
			})
			gameIdx++
		}
	}
	return pairings
}
