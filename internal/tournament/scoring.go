package tournament

import (
	"fmt"
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
