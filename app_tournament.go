package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rungine/internal/chess"
	"rungine/internal/registry"
	"rungine/internal/tournament"
)

// TournamentEngineRef references an installed engine to use in a tournament.
type TournamentEngineRef struct {
	ID      string            `json:"id"`
	Name    string            `json:"name,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// TournamentSpec is the GUI-side configuration for a tournament. It is
// translated into internal/tournament types when the run starts.
type TournamentSpec struct {
	Format        string                `json:"format"`
	Engines       []TournamentEngineRef `json:"engines"`
	Games         int                   `json:"games"`
	Concurrency   int                   `json:"concurrency"`
	TimeControlMs int                   `json:"timeControlMs"`
	DepthLimit    int                   `json:"depthLimit"`
	Event         string                `json:"event"`
	PairMode      bool                  `json:"pairMode"`
	MaxPlies      int                   `json:"maxPlies"`
	ResignScore   int                   `json:"resignScore"`
	ResignMoves   int                   `json:"resignMoves"`
	DrawScore     int                   `json:"drawScore"`
	DrawMoves     int                   `json:"drawMoves"`
	DrawMinPly    int                   `json:"drawMinPly"`
}

// GameRow is a flattened, JSON-friendly view of a completed game.
type GameRow struct {
	GameNumber int    `json:"gameNumber"`
	Round      string `json:"round"`
	White      string `json:"white"`
	Black      string `json:"black"`
	Outcome    string `json:"outcome"`
	Reason     string `json:"reason"`
	Plies      int    `json:"plies"`
	Error      string `json:"error,omitempty"`
}

// MoveDetail is one ply with the position after it and engine analysis.
type MoveDetail struct {
	Ply          int    `json:"ply"`
	Side         string `json:"side"` // "white" or "black"
	UCI          string `json:"uci"`
	SAN          string `json:"san"`
	FEN          string `json:"fen"`
	Depth        int    `json:"depth,omitempty"`
	EvalCp       *int   `json:"evalCp,omitempty"`
	EvalMate     *int   `json:"evalMate,omitempty"`
	ElapsedMs    int64  `json:"elapsedMs"`
	ClockAfterMs int64  `json:"clockAfterMs"`
}

// GameDetail is everything the GUI needs to replay a single game.
type GameDetail struct {
	GameNumber int          `json:"gameNumber"`
	Round      string       `json:"round"`
	White      string       `json:"white"`
	Black      string       `json:"black"`
	Result     string       `json:"result"`
	Reason     string       `json:"reason,omitempty"`
	Error      string       `json:"error,omitempty"`
	StartFEN   string       `json:"startFen"`
	PGN        string       `json:"pgn,omitempty"`
	Moves      []MoveDetail `json:"moves"`
}

// PlayerScoreRow is a JSON-friendly standings row.
type PlayerScoreRow struct {
	Name   string  `json:"name"`
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
	Games  int     `json:"games"`
	Points float64 `json:"points"`
}

// TournamentSummary is the snapshot of a tournament's current state.
type TournamentSummary struct {
	ID          string           `json:"id"`
	Spec        TournamentSpec   `json:"spec"`
	Status      string           `json:"status"`
	Error       string           `json:"error,omitempty"`
	StartedAt   time.Time        `json:"startedAt"`
	FinishedAt  *time.Time       `json:"finishedAt,omitempty"`
	GamesTotal  int              `json:"gamesTotal"`
	GamesPlayed int              `json:"gamesPlayed"`
	Outcomes    []GameRow        `json:"outcomes"`
	Standings   []PlayerScoreRow `json:"standings"`
}

type tournamentRun struct {
	mu       sync.Mutex
	id       string
	spec     TournamentSpec
	status   string
	errStr   string
	started  time.Time
	finished *time.Time
	total    int
	outcomes []tournament.GameOutcome
	cancel   context.CancelFunc
}

func (r *tournamentRun) snapshot() TournamentSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := make([]GameRow, 0, len(r.outcomes))
	for _, o := range r.outcomes {
		rows = append(rows, gameOutcomeRow(o))
	}
	standings := tournament.BuildStandings(r.outcomes)
	psRows := make([]PlayerScoreRow, 0, len(standings.Players))
	for _, p := range standings.Players {
		psRows = append(psRows, PlayerScoreRow{
			Name: p.Name, Wins: p.Wins, Draws: p.Draws,
			Losses: p.Losses, Games: p.Games, Points: p.Points,
		})
	}
	return TournamentSummary{
		ID: r.id, Spec: r.spec, Status: r.status, Error: r.errStr,
		StartedAt: r.started, FinishedAt: r.finished,
		GamesTotal: r.total, GamesPlayed: len(r.outcomes),
		Outcomes: rows, Standings: psRows,
	}
}

func gameOutcomeRow(o tournament.GameOutcome) GameRow {
	row := GameRow{
		GameNumber: o.Pairing.GameNumber,
		Round:      o.Pairing.Round,
		White:      o.Pairing.White.Name,
		Black:      o.Pairing.Black.Name,
	}
	if o.Err != nil {
		row.Error = o.Err.Error()
	}
	if o.Result != nil {
		row.Outcome = string(o.Result.Outcome)
		row.Reason = string(o.Result.Reason)
		row.Plies = o.Result.PlyCount
	}
	return row
}

// TournamentManager owns running and recent tournaments.
type TournamentManager struct {
	mu        sync.Mutex
	counter   int
	runs      map[string]*tournamentRun
	order     []string
	installer *registry.Installer
	ctx       context.Context
}

func newTournamentManager(installer *registry.Installer) *TournamentManager {
	return &TournamentManager{
		runs:      map[string]*tournamentRun{},
		installer: installer,
	}
}

func (m *TournamentManager) bindContext(ctx context.Context) {
	m.mu.Lock()
	m.ctx = ctx
	m.mu.Unlock()
}

func (m *TournamentManager) emit(event string, data interface{}) {
	m.mu.Lock()
	ctx := m.ctx
	m.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, event, data)
}

func (m *TournamentManager) resolveEngines(specs []TournamentEngineRef) ([]tournament.EngineSpec, error) {
	if m.installer == nil {
		return nil, errors.New("no installer available")
	}
	installed, err := m.installer.ListInstalled()
	if err != nil {
		return nil, fmt.Errorf("list installed engines: %w", err)
	}
	byID := map[string]registry.InstalledEngine{}
	for _, e := range installed {
		byID[e.ID] = e
	}
	out := make([]tournament.EngineSpec, 0, len(specs))
	for _, ref := range specs {
		eng, ok := byID[ref.ID]
		if !ok {
			return nil, fmt.Errorf("engine not installed: %s", ref.ID)
		}
		name := ref.Name
		if name == "" {
			name = eng.Name
		}
		out = append(out, tournament.EngineSpec{
			Name:       name,
			BinaryPath: eng.BinaryPath,
			Options:    ref.Options,
		})
	}
	return out, nil
}

func (m *TournamentManager) buildPairings(spec TournamentSpec, specs []tournament.EngineSpec) ([]tournament.Pairing, error) {
	switch spec.Format {
	case "match":
		if len(specs) != 2 {
			return nil, errors.New("match requires exactly 2 engines")
		}
		games := spec.Games
		if games < 1 {
			games = 2
		}
		return tournament.BuildMatch(tournament.MatchSpec{
			White: specs[0], Black: specs[1],
			Games: games, PairMode: spec.PairMode,
		}), nil
	case "round-robin":
		cycles := spec.Games
		if cycles < 1 {
			cycles = 1
		}
		return tournament.BuildRoundRobin(tournament.RoundRobinSpec{
			Engines: specs, Cycles: cycles, PairMode: spec.PairMode,
		}), nil
	case "gauntlet":
		if len(specs) < 2 {
			return nil, errors.New("gauntlet requires at least 2 engines")
		}
		gpo := spec.Games
		if gpo < 1 {
			gpo = 2
		}
		return tournament.BuildGauntlet(tournament.GauntletSpec{
			Challenger:       specs[0],
			Field:            specs[1:],
			GamesPerOpponent: gpo,
			PairMode:         spec.PairMode,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", spec.Format)
	}
}

func timeControlFromSpec(spec TournamentSpec) tournament.TimeControl {
	switch {
	case spec.TimeControlMs > 0:
		return tournament.TimeControl{FixedMovetime: time.Duration(spec.TimeControlMs) * time.Millisecond}
	case spec.DepthLimit > 0:
		return tournament.TimeControl{FixedDepth: spec.DepthLimit}
	default:
		return tournament.TimeControl{FixedMovetime: 200 * time.Millisecond}
	}
}

// Start kicks off a tournament asynchronously and returns its ID.
func (m *TournamentManager) Start(spec TournamentSpec) (string, error) {
	if len(spec.Engines) < 2 {
		return "", errors.New("need at least 2 engines")
	}
	engineSpecs, err := m.resolveEngines(spec.Engines)
	if err != nil {
		return "", err
	}
	pairings, err := m.buildPairings(spec, engineSpecs)
	if err != nil {
		return "", err
	}
	if len(pairings) == 0 {
		return "", errors.New("no pairings generated")
	}

	concurrency := spec.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	maxPlies := spec.MaxPlies
	if maxPlies <= 0 {
		maxPlies = 400
	}
	event := spec.Event
	if event == "" {
		event = "Rungine Tournament"
	}

	m.mu.Lock()
	m.counter++
	id := fmt.Sprintf("t%d", m.counter)
	parent := m.ctx
	if parent == nil {
		parent = context.Background()
	}
	runCtx, cancel := context.WithCancel(parent)
	run := &tournamentRun{
		id:      id,
		spec:    spec,
		status:  "running",
		started: time.Now(),
		total:   len(pairings),
		cancel:  cancel,
	}
	m.runs[id] = run
	m.order = append(m.order, id)
	m.mu.Unlock()

	cfg := tournament.SchedulerConfig{
		Concurrency: concurrency,
		TimeControl: timeControlFromSpec(spec),
		MaxPlies:    maxPlies,
		Factory:     tournament.DefaultEngineFactory,
		Event:       event,
		ResignScore: spec.ResignScore,
		ResignMoves: spec.ResignMoves,
		DrawScore:   spec.DrawScore,
		DrawMoves:   spec.DrawMoves,
		DrawMinPly:  spec.DrawMinPly,
		OnGameStart: func(p tournament.Pairing) {
			m.emit("tournament:gameStart", map[string]interface{}{
				"tournamentId": id,
				"gameNumber":   p.GameNumber,
				"round":        p.Round,
				"white":        p.White.Name,
				"black":        p.Black.Name,
			})
		},
		OnGameComplete: func(o tournament.GameOutcome) {
			run.mu.Lock()
			run.outcomes = append(run.outcomes, o)
			run.mu.Unlock()
			m.emit("tournament:gameComplete", map[string]interface{}{
				"tournamentId": id,
				"row":          gameOutcomeRow(o),
				"pgn":          o.PGN,
			})
		},
	}

	sch, err := tournament.NewScheduler(cfg)
	if err != nil {
		m.mu.Lock()
		delete(m.runs, id)
		m.mu.Unlock()
		cancel()
		return "", err
	}

	go func() {
		_ = sch.Run(runCtx, pairings)
		run.mu.Lock()
		now := time.Now()
		run.finished = &now
		switch {
		case runCtx.Err() == context.Canceled:
			run.status = "stopped"
		case run.errStr != "":
			run.status = "error"
		default:
			run.status = "done"
		}
		final := run.status
		run.mu.Unlock()
		m.emit("tournament:done", map[string]interface{}{
			"tournamentId": id,
			"status":       final,
		})
	}()

	return id, nil
}

// Stop cancels a running tournament.
func (m *TournamentManager) Stop(id string) error {
	m.mu.Lock()
	run, ok := m.runs[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("tournament not found: %s", id)
	}
	run.cancel()
	return nil
}

// Get returns a snapshot of one tournament.
func (m *TournamentManager) Get(id string) (TournamentSummary, error) {
	m.mu.Lock()
	run, ok := m.runs[id]
	m.mu.Unlock()
	if !ok {
		return TournamentSummary{}, fmt.Errorf("tournament not found: %s", id)
	}
	return run.snapshot(), nil
}

// GetGameDetail reconstructs a per-ply replay from the stored result.
func (m *TournamentManager) GetGameDetail(tournamentID string, gameNumber int) (GameDetail, error) {
	m.mu.Lock()
	run, ok := m.runs[tournamentID]
	m.mu.Unlock()
	if !ok {
		return GameDetail{}, fmt.Errorf("tournament not found: %s", tournamentID)
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	for _, o := range run.outcomes {
		if o.Pairing.GameNumber == gameNumber {
			return buildGameDetail(o)
		}
	}
	return GameDetail{}, fmt.Errorf("game %d not found in tournament %s", gameNumber, tournamentID)
}

func buildGameDetail(o tournament.GameOutcome) (GameDetail, error) {
	d := GameDetail{
		GameNumber: o.Pairing.GameNumber,
		Round:      o.Pairing.Round,
		White:      o.Pairing.White.Name,
		Black:      o.Pairing.Black.Name,
		PGN:        o.PGN,
	}
	if o.Result != nil {
		d.Result = string(o.Result.Outcome)
		d.Reason = string(o.Result.Reason)
	}
	if o.Err != nil {
		d.Error = o.Err.Error()
	}

	var game *chess.Game
	var err error
	if o.Pairing.StartFEN != "" {
		game, err = chess.FromFEN(o.Pairing.StartFEN)
		if err != nil {
			return d, fmt.Errorf("start FEN: %w", err)
		}
	} else {
		game = chess.NewGame()
	}
	for _, mv := range o.Pairing.StartMoves {
		if err := game.PushUCI(mv); err != nil {
			return d, fmt.Errorf("apply opening %s: %w", mv, err)
		}
	}
	d.StartFEN = game.FEN()

	if o.Result == nil {
		return d, nil
	}
	d.Moves = make([]MoveDetail, 0, len(o.Result.Moves))
	for _, mr := range o.Result.Moves {
		if err := game.PushUCI(mr.UCI); err != nil {
			break
		}
		md := MoveDetail{
			Ply: mr.Ply, Side: string(mr.Side),
			UCI: mr.UCI, SAN: mr.SAN, FEN: game.FEN(),
			ElapsedMs:    mr.Elapsed.Milliseconds(),
			ClockAfterMs: mr.ClockAfter.Milliseconds(),
		}
		if mr.HasInfo {
			md.Depth = mr.Info.Depth
			if mr.Info.Score.Mate != nil {
				v := *mr.Info.Score.Mate
				md.EvalMate = &v
			} else if mr.Info.Score.Centipawns != nil {
				v := *mr.Info.Score.Centipawns
				md.EvalCp = &v
			}
		}
		d.Moves = append(d.Moves, md)
	}
	return d, nil
}

// List returns snapshots of all tournaments, oldest first.
func (m *TournamentManager) List() []TournamentSummary {
	m.mu.Lock()
	ids := make([]string, len(m.order))
	copy(ids, m.order)
	runs := make([]*tournamentRun, 0, len(ids))
	for _, id := range ids {
		if r, ok := m.runs[id]; ok {
			runs = append(runs, r)
		}
	}
	m.mu.Unlock()
	out := make([]TournamentSummary, 0, len(runs))
	for _, r := range runs {
		out = append(out, r.snapshot())
	}
	return out
}

// ChessOutcomes is a small re-export so the frontend type generator picks
// up our outcome string set.
var ChessOutcomes = struct {
	WhiteWins string
	BlackWins string
	Drawn     string
	Ongoing   string
}{
	WhiteWins: string(chess.WhiteWins),
	BlackWins: string(chess.BlackWins),
	Drawn:     string(chess.Drawn),
	Ongoing:   string(chess.Ongoing),
}
