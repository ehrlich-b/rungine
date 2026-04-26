package tournament

import (
	"sort"
	"strconv"

	"rungine/internal/chess"
)

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

// SwissSpec configures BuildSwissRound.
type SwissSpec struct {
	// Engines is the field. Round 1 ordering follows this slice; later
	// rounds re-rank by score-then-name.
	Engines []EngineSpec

	// Round is the 1-based round number being generated.
	Round int

	// Previous holds completed games from earlier rounds. Used to score
	// engines and avoid repeat pairings.
	Previous []GameOutcome

	Openings []Opening
	PairMode bool

	StartGameNumber int
}

// BuildSwissRound generates pairings for one Swiss round given prior
// game outcomes. Engines are sorted by current standing (points desc,
// name asc), then paired greedily within score groups while avoiding
// repeat opponents. Color allocation favors balancing prior white/black
// counts. If the field has odd cardinality the lowest-ranked unpaired
// engine receives an implicit bye (no pairing emitted for it).
func BuildSwissRound(spec SwissSpec) []Pairing {
	if len(spec.Engines) < 2 {
		return nil
	}
	if spec.Round < 1 {
		spec.Round = 1
	}
	if spec.StartGameNumber < 1 {
		spec.StartGameNumber = 1
	}

	type info struct {
		spec       EngineSpec
		score      float64
		whiteCount int
		blackCount int
	}
	state := make(map[string]*info, len(spec.Engines))
	order := make([]*info, 0, len(spec.Engines))
	for _, e := range spec.Engines {
		i := &info{spec: e}
		state[e.Name] = i
		order = append(order, i)
	}

	paired := map[[2]string]bool{}
	for _, o := range spec.Previous {
		if o.Err != nil || o.Result == nil {
			continue
		}
		wn, bn := o.Pairing.White.Name, o.Pairing.Black.Name
		ws, bs := state[wn], state[bn]
		if ws == nil || bs == nil {
			continue
		}
		switch o.Result.Outcome {
		case chess.WhiteWins:
			ws.score += 1
		case chess.BlackWins:
			bs.score += 1
		case chess.Drawn:
			ws.score += 0.5
			bs.score += 0.5
		default:
			continue
		}
		ws.whiteCount++
		bs.blackCount++
		a, b := wn, bn
		if a > b {
			a, b = b, a
		}
		paired[[2]string{a, b}] = true
	}

	// Round 1: keep input order. Later rounds: sort by score desc, name asc.
	sorted := order
	if spec.Round > 1 || len(spec.Previous) > 0 {
		sorted = make([]*info, len(order))
		copy(sorted, order)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].score != sorted[j].score {
				return sorted[i].score > sorted[j].score
			}
			return sorted[i].spec.Name < sorted[j].spec.Name
		})
	}

	used := map[string]bool{}
	var pairings []Pairing
	gameIdx := 0
	for i, p := range sorted {
		if used[p.spec.Name] {
			continue
		}
		var opponent *info
		// Prefer an opponent we haven't faced.
		for j := i + 1; j < len(sorted); j++ {
			q := sorted[j]
			if used[q.spec.Name] {
				continue
			}
			a, b := p.spec.Name, q.spec.Name
			if a > b {
				a, b = b, a
			}
			if paired[[2]string{a, b}] {
				continue
			}
			opponent = q
			break
		}
		// Fall back to nearest opponent regardless of repetition.
		if opponent == nil {
			for j := i + 1; j < len(sorted); j++ {
				q := sorted[j]
				if !used[q.spec.Name] {
					opponent = q
					break
				}
			}
		}
		if opponent == nil {
			used[p.spec.Name] = true // bye
			continue
		}
		used[p.spec.Name] = true
		used[opponent.spec.Name] = true

		// Assign white to whoever has played fewer whites; tiebreak by
		// keeping the higher-ranked player on white.
		white, black := p, opponent
		if white.whiteCount > black.whiteCount {
			white, black = opponent, p
		}

		opening := pickOpening(spec.Openings, gameIdx, spec.PairMode)
		pairings = append(pairings, Pairing{
			GameNumber: spec.StartGameNumber + gameIdx,
			Round:      strconv.Itoa(spec.Round),
			White:      white.spec,
			Black:      black.spec,
			StartFEN:   opening.StartFEN,
			StartMoves: opening.StartMoves,
		})
		gameIdx++
	}
	return pairings
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
