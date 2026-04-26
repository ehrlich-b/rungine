package tournament

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"rungine/internal/chess"
)

// PlayerScore is one row in a Standings table.
type PlayerScore struct {
	Name   string
	Wins   int
	Draws  int
	Losses int
	Games  int
	Points float64 // wins + 0.5 * draws
}

// ScorePct returns the player's score as a fraction of games played.
// Returns 0 when no games were played.
func (p PlayerScore) ScorePct() float64 {
	if p.Games == 0 {
		return 0
	}
	return p.Points / float64(p.Games)
}

// Standings is a sorted leaderboard.
type Standings struct {
	Players []PlayerScore
}

// BuildStandings aggregates outcomes into a leaderboard. Games with a
// non-nil Err or non-terminal Outcome are skipped. Players are sorted by
// points desc, wins desc, then name asc.
func BuildStandings(outcomes []GameOutcome) Standings {
	scores := map[string]*PlayerScore{}

	ensure := func(name string) *PlayerScore {
		if s, ok := scores[name]; ok {
			return s
		}
		s := &PlayerScore{Name: name}
		scores[name] = s
		return s
	}

	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		w := ensure(o.Pairing.White.Name)
		b := ensure(o.Pairing.Black.Name)

		switch o.Result.Outcome {
		case chess.WhiteWins:
			w.Wins++
			w.Points += 1
			b.Losses++
		case chess.BlackWins:
			b.Wins++
			b.Points += 1
			w.Losses++
		case chess.Drawn:
			w.Draws++
			b.Draws++
			w.Points += 0.5
			b.Points += 0.5
		default:
			// Ongoing or unknown — skip.
			continue
		}
		w.Games++
		b.Games++
	}

	out := Standings{Players: make([]PlayerScore, 0, len(scores))}
	for _, s := range scores {
		out.Players = append(out.Players, *s)
	}
	sort.Slice(out.Players, func(i, j int) bool {
		if out.Players[i].Points != out.Players[j].Points {
			return out.Players[i].Points > out.Players[j].Points
		}
		if out.Players[i].Wins != out.Players[j].Wins {
			return out.Players[i].Wins > out.Players[j].Wins
		}
		return out.Players[i].Name < out.Players[j].Name
	})
	return out
}

// String renders a Standings table as plain text.
func (s Standings) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%-24s %5s %5s %5s %5s %7s %6s\n",
		"Engine", "G", "W", "D", "L", "Pts", "Score")
	for _, p := range s.Players {
		fmt.Fprintf(&sb, "%-24s %5d %5d %5d %5d %7.1f %5.1f%%\n",
			truncate(p.Name, 24), p.Games, p.Wins, p.Draws, p.Losses, p.Points, p.ScorePct()*100)
	}
	return sb.String()
}

// Crosstable is a head-to-head matrix.
type Crosstable struct {
	// Players is the player order; Score[i][j] is player i's points
	// across all games against player j (both colors). Games[i][j] is
	// the number of completed games between i and j.
	Players []string
	Score   [][]float64
	Games   [][]int
}

// BuildCrosstable builds a head-to-head matrix from outcomes. Players
// are listed in the order returned by BuildStandings (top of table to
// bottom) so the matrix lines up with the leaderboard.
func BuildCrosstable(outcomes []GameOutcome) Crosstable {
	standings := BuildStandings(outcomes)
	players := make([]string, len(standings.Players))
	idx := map[string]int{}
	for i, p := range standings.Players {
		players[i] = p.Name
		idx[p.Name] = i
	}

	n := len(players)
	score := make([][]float64, n)
	games := make([][]int, n)
	for i := range n {
		score[i] = make([]float64, n)
		games[i] = make([]int, n)
	}

	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		wi, ok1 := idx[o.Pairing.White.Name]
		bi, ok2 := idx[o.Pairing.Black.Name]
		if !ok1 || !ok2 {
			continue
		}

		switch o.Result.Outcome {
		case chess.WhiteWins:
			score[wi][bi] += 1
		case chess.BlackWins:
			score[bi][wi] += 1
		case chess.Drawn:
			score[wi][bi] += 0.5
			score[bi][wi] += 0.5
		default:
			continue
		}
		games[wi][bi]++
		games[bi][wi]++
	}

	return Crosstable{Players: players, Score: score, Games: games}
}

// String renders a Crosstable as a textual matrix. Cells show points
// like "1.5/2"; the diagonal is blank.
func (c Crosstable) String() string {
	if len(c.Players) == 0 {
		return ""
	}
	colWidth := 8
	var sb strings.Builder

	header := fmt.Sprintf("%-20s", "")
	for _, p := range c.Players {
		header += fmt.Sprintf(" %*s", colWidth, truncate(p, colWidth))
	}
	sb.WriteString(header + "\n")

	for i, p := range c.Players {
		fmt.Fprintf(&sb, "%-20s", truncate(p, 20))
		for j := range c.Players {
			if i == j {
				fmt.Fprintf(&sb, " %*s", colWidth, "—")
				continue
			}
			cell := fmt.Sprintf("%.1f/%d", c.Score[i][j], c.Games[i][j])
			fmt.Fprintf(&sb, " %*s", colWidth, cell)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// EloDelta returns the ELO rating difference implied by a score
// percentage in [0, 1]. 0.5 → 0; saturated scores clamp at ±800.
func EloDelta(scorePct float64) float64 {
	const cap = 800.0
	if scorePct >= 1 {
		return cap
	}
	if scorePct <= 0 {
		return -cap
	}
	delta := -400 * math.Log10(1/scorePct-1)
	if delta > cap {
		return cap
	}
	if delta < -cap {
		return -cap
	}
	return delta
}

// PerformanceRating returns the ELO rating consistent with achieving
// scorePct against opponents averaging opponentRating.
func PerformanceRating(scorePct, opponentRating float64) float64 {
	return opponentRating + EloDelta(scorePct)
}

// DrawRatio returns draws / games. Zero when games == 0.
func DrawRatio(draws, games int) float64 {
	if games <= 0 {
		return 0
	}
	return float64(draws) / float64(games)
}

// LikelihoodOfSuperiority estimates the probability that one engine is
// truly stronger given W wins and L losses against another. Draws are
// ignored. Returns 0.5 when there are no decisive games.
func LikelihoodOfSuperiority(wins, losses int) float64 {
	n := wins + losses
	if n == 0 {
		return 0.5
	}
	z := float64(wins-losses) / math.Sqrt(float64(2*n))
	return 0.5 * (1 + math.Erf(z))
}

// trinomialMeanSE returns the score-percentage mean and standard error
// from W/D/L counts using the trinomial-distribution variance.
func trinomialMeanSE(wins, draws, losses int) (mean, se float64) {
	n := wins + draws + losses
	if n == 0 {
		return 0.5, 0
	}
	nf := float64(n)
	mean = (float64(wins) + 0.5*float64(draws)) / nf
	variance := float64(wins)*math.Pow(1-mean, 2) +
		float64(draws)*math.Pow(0.5-mean, 2) +
		float64(losses)*math.Pow(mean, 2)
	variance /= nf
	se = math.Sqrt(variance / nf)
	return mean, se
}

// zForConfidence returns the standard-normal critical value for a
// two-sided confidence level. Common levels are tabulated; defaults to
// 1.96 (95%) for unknown values.
func zForConfidence(confidence float64) float64 {
	switch {
	case confidence >= 0.99:
		return 2.576
	case confidence >= 0.975:
		return 2.241
	case confidence >= 0.95:
		return 1.96
	case confidence >= 0.90:
		return 1.645
	case confidence >= 0.80:
		return 1.282
	default:
		return 1.96
	}
}

// ScoreInterval returns (lo, mean, hi) on the score percentage at the
// given confidence using a normal approximation with trinomial variance.
func ScoreInterval(wins, draws, losses int, confidence float64) (lo, mean, hi float64) {
	mean, se := trinomialMeanSE(wins, draws, losses)
	z := zForConfidence(confidence)
	lo = math.Max(0, mean-z*se)
	hi = math.Min(1, mean+z*se)
	return lo, mean, hi
}

// EloInterval returns (lo, mid, hi) ELO deltas at the given confidence
// level given W/D/L. mid is the point estimate.
func EloInterval(wins, draws, losses int, confidence float64) (lo, mid, hi float64) {
	pLo, pMid, pHi := ScoreInterval(wins, draws, losses, confidence)
	return EloDelta(pLo), EloDelta(pMid), EloDelta(pHi)
}

// EstimateElos returns ELO ratings for every engine that played in
// outcomes, fit by iterative performance-rating with damping. If
// anchorName is in outcomes, it is held at anchorRating; otherwise the
// mean rating across all engines is fixed at anchorRating. Convergence
// stops at maxChange < 0.01 ELO or 500 iterations.
func EstimateElos(outcomes []GameOutcome, anchorName string, anchorRating float64) map[string]float64 {
	nameIdx := map[string]int{}
	addName := func(n string) {
		if _, ok := nameIdx[n]; !ok {
			nameIdx[n] = len(nameIdx)
		}
	}
	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		addName(o.Pairing.White.Name)
		addName(o.Pairing.Black.Name)
	}
	n := len(nameIdx)
	if n == 0 {
		return map[string]float64{}
	}

	games := make([][]float64, n)
	score := make([][]float64, n)
	for i := range n {
		games[i] = make([]float64, n)
		score[i] = make([]float64, n)
	}
	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		wi, bi := nameIdx[o.Pairing.White.Name], nameIdx[o.Pairing.Black.Name]
		switch o.Result.Outcome {
		case chess.WhiteWins:
			score[wi][bi] += 1
			games[wi][bi]++
			games[bi][wi]++
		case chess.BlackWins:
			score[bi][wi] += 1
			games[wi][bi]++
			games[bi][wi]++
		case chess.Drawn:
			score[wi][bi] += 0.5
			score[bi][wi] += 0.5
			games[wi][bi]++
			games[bi][wi]++
		default:
			continue
		}
	}

	ratings := make([]float64, n)
	for i := range ratings {
		ratings[i] = anchorRating
	}

	const (
		maxIter = 500
		tol     = 0.01
		damping = 0.5
	)

	for range maxIter {
		newRatings := make([]float64, n)
		for i := range n {
			totalScore, totalGames, oppRatingSum := 0.0, 0.0, 0.0
			for j := range n {
				if games[i][j] == 0 {
					continue
				}
				totalScore += score[i][j]
				totalGames += games[i][j]
				oppRatingSum += games[i][j] * ratings[j]
			}
			if totalGames == 0 {
				newRatings[i] = ratings[i]
				continue
			}
			scorePct := totalScore / totalGames
			avgOpp := oppRatingSum / totalGames
			newRatings[i] = PerformanceRating(scorePct, avgOpp)
		}

		maxChange := 0.0
		for i := range n {
			next := damping*ratings[i] + (1-damping)*newRatings[i]
			if d := math.Abs(next - ratings[i]); d > maxChange {
				maxChange = d
			}
			ratings[i] = next
		}

		// Pin the anchor (or pin the mean to anchorRating if no anchor).
		if idx, ok := nameIdx[anchorName]; ok && anchorName != "" {
			shift := anchorRating - ratings[idx]
			for i := range ratings {
				ratings[i] += shift
			}
		} else {
			sum := 0.0
			for _, r := range ratings {
				sum += r
			}
			shift := anchorRating - sum/float64(n)
			for i := range ratings {
				ratings[i] += shift
			}
		}

		if maxChange < tol {
			break
		}
	}

	out := make(map[string]float64, n)
	for name, idx := range nameIdx {
		out[name] = ratings[idx]
	}
	return out
}
