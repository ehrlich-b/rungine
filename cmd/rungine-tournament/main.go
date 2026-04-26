// rungine-tournament runs a tournament from the command line using the
// internal tournament package. It supports match, round-robin, gauntlet,
// and Swiss formats over a pool of UCI engines.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"rungine/internal/tournament"
)

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var engines stringList
	flag.Var(&engines, "engine", "engine spec, repeatable: name=path or just path")
	games := flag.Int("games", 10, "games per pair (match/gauntlet) or cycles (round-robin)")
	rounds := flag.Int("rounds", 5, "Swiss-only: number of rounds")
	concurrency := flag.Int("concurrency", 1, "max concurrent games")
	tcSpec := flag.String("tc", "movetime=200ms", "time control: movetime=Nms | depth=N | nodes=N | T+I (seconds)")
	format := flag.String("format", "match", "match | round-robin | gauntlet | swiss")
	openingsFile := flag.String("openings", "", "PGN file with openings (optional)")
	samplePly := flag.Int("sample-ply", 8, "plies to take per opening")
	pairMode := flag.Bool("pair-mode", true, "play each opening twice with colors flipped")
	outputPGN := flag.String("output", "", "write all games as concatenated PGN to this file")
	sprtFlag := flag.String("sprt", "", "match-only: elo0,elo1,alpha,beta — early-stop by SPRT")
	event := flag.String("event", "Rungine Match", "PGN [Event] tag")
	site := flag.String("site", "", "PGN [Site] tag")
	maxPlies := flag.Int("max-plies", 400, "abandon games beyond this many half-moves")
	resignScore := flag.Int("resign-score", 0, "resign adjudication threshold in centipawns (0 = off)")
	resignMoves := flag.Int("resign-moves", 4, "consecutive plies for resign adjudication")
	drawScore := flag.Int("draw-score", -1, "draw adjudication threshold in centipawns (-1 = off)")
	drawMoves := flag.Int("draw-moves", 8, "consecutive plies for draw adjudication")
	drawMinPly := flag.Int("draw-min-ply", 60, "earliest ply at which draw adjudication can fire")

	flag.Parse()

	if len(engines) < 2 {
		return errors.New("need at least two --engine flags")
	}

	specs, err := parseEngines(engines)
	if err != nil {
		return err
	}

	tc, err := parseTimeControl(*tcSpec)
	if err != nil {
		return err
	}

	var openings []tournament.Opening
	if *openingsFile != "" {
		f, err := os.Open(*openingsFile)
		if err != nil {
			return fmt.Errorf("open openings: %w", err)
		}
		openings, err = tournament.LoadOpeningsFromPGN(f, *samplePly)
		f.Close()
		if err != nil {
			return fmt.Errorf("parse openings: %w", err)
		}
		fmt.Fprintf(os.Stderr, "loaded %d openings (%d plies each)\n", len(openings), *samplePly)
	}

	cfg := tournament.SchedulerConfig{
		Concurrency: *concurrency,
		TimeControl: tc,
		MaxPlies:    *maxPlies,
		Factory:     tournament.DefaultEngineFactory,
		Event:       *event,
		Site:        *site,
		ResignScore: *resignScore,
		ResignMoves: *resignMoves,
		DrawScore:   *drawScore,
		DrawMoves:   *drawMoves,
		DrawMinPly:  *drawMinPly,
		OnGameComplete: func(o tournament.GameOutcome) {
			label := fmt.Sprintf("[%d r%s]", o.Pairing.GameNumber, o.Pairing.Round)
			if o.Err != nil {
				fmt.Fprintf(os.Stderr, "%s %s vs %s: error %v\n",
					label, o.Pairing.White.Name, o.Pairing.Black.Name, o.Err)
				return
			}
			if o.Result == nil {
				return
			}
			fmt.Fprintf(os.Stderr, "%s %s vs %s: %s (%s, %d plies)\n",
				label, o.Pairing.White.Name, o.Pairing.Black.Name,
				o.Result.Outcome, o.Result.Reason, o.Result.PlyCount)
		},
	}

	if *sprtFlag != "" {
		if *format != "match" {
			return errors.New("--sprt requires --format=match")
		}
		sprtCfg, err := parseSPRT(*sprtFlag)
		if err != nil {
			return err
		}
		cfg.ShouldStop = tournament.NewSPRTStopper(sprtCfg, specs[0].Name, func(r tournament.SPRTResult) {
			fmt.Fprintf(os.Stderr, "SPRT: %s (LLR=%.3f, bounds [%.3f, %.3f])\n",
				r.Decision, r.LLR, r.LowerBound, r.UpperBound)
		})
	}

	sch, err := tournament.NewScheduler(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var outcomes []tournament.GameOutcome
	switch *format {
	case "match":
		if len(specs) != 2 {
			return errors.New("match format requires exactly two engines")
		}
		pairings := tournament.BuildMatch(tournament.MatchSpec{
			White: specs[0], Black: specs[1],
			Games: *games, Openings: openings, PairMode: *pairMode,
		})
		outcomes = sch.Run(ctx, pairings)

	case "round-robin":
		pairings := tournament.BuildRoundRobin(tournament.RoundRobinSpec{
			Engines: specs, Cycles: *games,
			Openings: openings, PairMode: *pairMode,
		})
		outcomes = sch.Run(ctx, pairings)

	case "gauntlet":
		if len(specs) < 2 {
			return errors.New("gauntlet requires at least two engines")
		}
		pairings := tournament.BuildGauntlet(tournament.GauntletSpec{
			Challenger:       specs[0],
			Field:            specs[1:],
			GamesPerOpponent: *games,
			Openings:         openings, PairMode: *pairMode,
		})
		outcomes = sch.Run(ctx, pairings)

	case "swiss":
		gameStart := 1
		for r := 1; r <= *rounds; r++ {
			roundPairings := tournament.BuildSwissRound(tournament.SwissSpec{
				Engines: specs, Round: r, Previous: outcomes,
				StartGameNumber: gameStart,
				Openings:        openings, PairMode: *pairMode,
			})
			if len(roundPairings) == 0 {
				break
			}
			fmt.Fprintf(os.Stderr, "round %d/%d: %d games\n", r, *rounds, len(roundPairings))
			roundOutcomes := sch.Run(ctx, roundPairings)
			outcomes = append(outcomes, roundOutcomes...)
			gameStart += len(roundPairings)
			if ctx.Err() != nil {
				break
			}
		}

	default:
		return fmt.Errorf("unknown format: %s", *format)
	}

	if *outputPGN != "" {
		var sb strings.Builder
		for _, o := range outcomes {
			if o.PGN == "" {
				continue
			}
			sb.WriteString(o.PGN)
			sb.WriteString("\n\n")
		}
		if err := os.WriteFile(*outputPGN, []byte(sb.String()), 0o644); err != nil {
			return fmt.Errorf("write PGN: %w", err)
		}
		fmt.Fprintf(os.Stderr, "wrote %d games to %s\n", len(outcomes), *outputPGN)
	}

	standings := tournament.BuildStandings(outcomes)
	fmt.Println()
	fmt.Println("Standings:")
	fmt.Println(standings.String())

	if len(specs) >= 3 || *format == "round-robin" || *format == "swiss" {
		fmt.Println("Crosstable:")
		fmt.Println(tournament.BuildCrosstable(outcomes).String())
	}

	if len(specs) >= 2 {
		anchor := specs[len(specs)-1].Name
		elos := tournament.EstimateElos(outcomes, anchor, 0)
		fmt.Printf("ELOs (anchored at %s = 0):\n", anchor)
		for _, p := range standings.Players {
			lo, mid, hi := tournament.EloInterval(p.Wins, p.Draws, p.Losses, 0.95)
			fmt.Printf("  %-24s  rating %+7.1f   delta %+6.1f  CI95%% [%+6.1f, %+6.1f]\n",
				p.Name, elos[p.Name], mid, lo, hi)
		}
	}

	return nil
}

func parseEngines(specs []string) ([]tournament.EngineSpec, error) {
	out := make([]tournament.EngineSpec, 0, len(specs))
	for _, s := range specs {
		var name, path string
		if eq := strings.Index(s, "="); eq > 0 {
			name = s[:eq]
			path = s[eq+1:]
		} else {
			path = s
			name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		if path == "" {
			return nil, fmt.Errorf("engine spec missing path: %q", s)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("engine binary %q: %w", path, err)
		}
		out = append(out, tournament.EngineSpec{Name: name, BinaryPath: path})
	}
	return out, nil
}

func parseTimeControl(s string) (tournament.TimeControl, error) {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "movetime="):
		v := strings.TrimPrefix(s, "movetime=")
		if d, err := time.ParseDuration(v); err == nil {
			return tournament.TimeControl{FixedMovetime: d}, nil
		}
		// Bare integer = ms.
		if n, err := strconv.Atoi(v); err == nil {
			return tournament.TimeControl{FixedMovetime: time.Duration(n) * time.Millisecond}, nil
		}
		return tournament.TimeControl{}, fmt.Errorf("invalid movetime: %q", v)
	case strings.HasPrefix(s, "depth="):
		n, err := strconv.Atoi(strings.TrimPrefix(s, "depth="))
		if err != nil {
			return tournament.TimeControl{}, fmt.Errorf("invalid depth: %w", err)
		}
		return tournament.TimeControl{FixedDepth: n}, nil
	case strings.HasPrefix(s, "nodes="):
		n, err := strconv.ParseInt(strings.TrimPrefix(s, "nodes="), 10, 64)
		if err != nil {
			return tournament.TimeControl{}, fmt.Errorf("invalid nodes: %w", err)
		}
		return tournament.TimeControl{FixedNodes: n}, nil
	}
	parts := strings.SplitN(s, "+", 2)
	initial, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return tournament.TimeControl{}, fmt.Errorf("invalid initial time: %w", err)
	}
	tc := tournament.TimeControl{Initial: time.Duration(initial * float64(time.Second))}
	if len(parts) == 2 {
		inc, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return tournament.TimeControl{}, fmt.Errorf("invalid increment: %w", err)
		}
		tc.Increment = time.Duration(inc * float64(time.Second))
	}
	return tc, nil
}

func parseSPRT(s string) (tournament.SPRTConfig, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return tournament.SPRTConfig{}, errors.New("--sprt expects elo0,elo1,alpha,beta")
	}
	cfg := tournament.SPRTConfig{}
	var err error
	if cfg.Elo0, err = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err != nil {
		return cfg, fmt.Errorf("invalid elo0: %w", err)
	}
	if cfg.Elo1, err = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err != nil {
		return cfg, fmt.Errorf("invalid elo1: %w", err)
	}
	if cfg.Alpha, err = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64); err != nil {
		return cfg, fmt.Errorf("invalid alpha: %w", err)
	}
	if cfg.Beta, err = strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err != nil {
		return cfg, fmt.Errorf("invalid beta: %w", err)
	}
	return cfg, nil
}
