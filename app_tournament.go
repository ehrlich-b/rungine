package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"rungine/internal/chess"
	"rungine/internal/database"
	"rungine/internal/registry"
	"rungine/internal/tournament"
	"rungine/internal/uci"
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
	NodesLimit    int64                 `json:"nodesLimit"`
	TcInitialMs   int                   `json:"tcInitialMs"`
	TcIncrementMs int                   `json:"tcIncrementMs"`
	// StartFEN, if non-empty, sets the starting position for every game.
	// Use to force unbalanced openings (Chess960-style FENs work too).
	StartFEN      string                `json:"startFen"`
	Event         string                `json:"event"`
	PairMode      bool                  `json:"pairMode"`
	MaxPlies      int                   `json:"maxPlies"`
	ResignScore   int                   `json:"resignScore"`
	ResignMoves   int                   `json:"resignMoves"`
	DrawScore     int                   `json:"drawScore"`
	DrawMoves     int                   `json:"drawMoves"`
	DrawMinPly    int                   `json:"drawMinPly"`

	// SPRT (match format only). Both Alpha and Beta must be > 0 to enable.
	// The first engine in Engines is treated as the candidate.
	SprtElo0  float64 `json:"sprtElo0"`
	SprtElo1  float64 `json:"sprtElo1"`
	SprtAlpha float64 `json:"sprtAlpha"`
	SprtBeta  float64 `json:"sprtBeta"`
}

// SprtState is the live SPRT progress for a tournament that has SPRT
// enabled. Decision strings: "continue", "accept H0", "accept H1".
type SprtState struct {
	LLR        float64 `json:"llr"`
	LowerBound float64 `json:"lowerBound"`
	UpperBound float64 `json:"upperBound"`
	Decision   string  `json:"decision"`
	// Wins/Draws/Losses are tallied from the candidate's POV.
	Wins   int `json:"wins"`
	Draws  int `json:"draws"`
	Losses int `json:"losses"`
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
	Check        bool   `json:"check,omitempty"`
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
	// Elo is the iterative performance-rating fit (mean-anchored at 0).
	Elo float64 `json:"elo"`
	// EloLo and EloHi are the 95% normal-approximation interval endpoints
	// of the rating delta from observed W/D/L.
	EloLo float64 `json:"eloLo"`
	EloHi float64 `json:"eloHi"`
}

// CrosstableCell is W/D/L between two players (i vs j).
type CrosstableCell struct {
	Wins   int     `json:"wins"`
	Draws  int     `json:"draws"`
	Losses int     `json:"losses"`
	Games  int     `json:"games"`
	Points float64 `json:"points"`
}

// CrosstableData is the head-to-head matrix in JSON-friendly form.
// Players is the row/column order. Cells[i][j] is i's record against j;
// Cells[i][i] is the empty diagonal cell.
type CrosstableData struct {
	Players []string             `json:"players"`
	Cells   [][]CrosstableCell   `json:"cells"`
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
	Crosstable  CrosstableData   `json:"crosstable"`
	Sprt        *SprtState       `json:"sprt,omitempty"`
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
	sprt     *SprtState
}

func (r *tournamentRun) snapshot() TournamentSummary {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := make([]GameRow, 0, len(r.outcomes))
	for _, o := range r.outcomes {
		rows = append(rows, gameOutcomeRow(o))
	}
	standings := tournament.BuildStandings(r.outcomes)
	elos := tournament.EstimateElos(r.outcomes, "", 0)
	psRows := make([]PlayerScoreRow, 0, len(standings.Players))
	for _, p := range standings.Players {
		lo, _, hi := tournament.EloInterval(p.Wins, p.Draws, p.Losses, 0.95)
		psRows = append(psRows, PlayerScoreRow{
			Name: p.Name, Wins: p.Wins, Draws: p.Draws,
			Losses: p.Losses, Games: p.Games, Points: p.Points,
			Elo: elos[p.Name], EloLo: lo, EloHi: hi,
		})
	}
	var sprt *SprtState
	if r.sprt != nil {
		s := *r.sprt
		sprt = &s
	}
	return TournamentSummary{
		ID: r.id, Spec: r.spec, Status: r.status, Error: r.errStr,
		StartedAt: r.started, FinishedAt: r.finished,
		GamesTotal: r.total, GamesPlayed: len(r.outcomes),
		Outcomes: rows, Standings: psRows,
		Crosstable: buildCrosstableData(r.outcomes),
		Sprt:       sprt,
	}
}

func buildCrosstableData(outcomes []tournament.GameOutcome) CrosstableData {
	ct := tournament.BuildCrosstable(outcomes)
	if len(ct.Players) == 0 {
		return CrosstableData{Players: []string{}, Cells: [][]CrosstableCell{}}
	}
	// Aggregate W/D/L from outcomes for each ordered pair.
	idx := map[string]int{}
	for i, p := range ct.Players {
		idx[p] = i
	}
	n := len(ct.Players)
	cells := make([][]CrosstableCell, n)
	for i := range n {
		cells[i] = make([]CrosstableCell, n)
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
			cells[wi][bi].Wins++
			cells[bi][wi].Losses++
		case chess.BlackWins:
			cells[bi][wi].Wins++
			cells[wi][bi].Losses++
		case chess.Drawn:
			cells[wi][bi].Draws++
			cells[bi][wi].Draws++
		}
		cells[wi][bi].Games++
		cells[bi][wi].Games++
	}
	for i := range n {
		for j := range n {
			c := &cells[i][j]
			c.Points = float64(c.Wins) + 0.5*float64(c.Draws)
		}
	}
	return CrosstableData{Players: ct.Players, Cells: cells}
}

// sprtTally counts wins/draws/losses for the candidate across the
// outcome list. Foreign games (without the candidate) and errored games
// are ignored.
func sprtTally(outcomes []tournament.GameOutcome, candidate string) (w, d, l int) {
	for _, o := range outcomes {
		if o.Err != nil || o.Result == nil {
			continue
		}
		var candidateIsWhite bool
		switch candidate {
		case o.Pairing.White.Name:
			candidateIsWhite = true
		case o.Pairing.Black.Name:
			candidateIsWhite = false
		default:
			continue
		}
		switch o.Result.Outcome {
		case chess.WhiteWins:
			if candidateIsWhite {
				w++
			} else {
				l++
			}
		case chess.BlackWins:
			if candidateIsWhite {
				l++
			} else {
				w++
			}
		case chess.Drawn:
			d++
		}
	}
	return
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
	db        *database.DB
	ctx       context.Context
}

func newTournamentManager(installer *registry.Installer, db *database.DB) *TournamentManager {
	return &TournamentManager{
		runs:      map[string]*tournamentRun{},
		installer: installer,
		db:        db,
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
	case spec.TcInitialMs > 0:
		return tournament.TimeControl{
			Initial:   time.Duration(spec.TcInitialMs) * time.Millisecond,
			Increment: time.Duration(spec.TcIncrementMs) * time.Millisecond,
		}
	case spec.TimeControlMs > 0:
		return tournament.TimeControl{FixedMovetime: time.Duration(spec.TimeControlMs) * time.Millisecond}
	case spec.DepthLimit > 0:
		return tournament.TimeControl{FixedDepth: spec.DepthLimit}
	case spec.NodesLimit > 0:
		return tournament.TimeControl{FixedNodes: spec.NodesLimit}
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
	if startFen := strings.TrimSpace(spec.StartFEN); startFen != "" {
		// Validate once so a bad FEN fails fast at start time, not in the
		// arbiter goroutine after engines spin up.
		if _, err := chess.FromFEN(startFen); err != nil {
			return "", fmt.Errorf("invalid start FEN: %w", err)
		}
		for i := range pairings {
			pairings[i].StartFEN = startFen
			pairings[i].StartMoves = nil
		}
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

	if err := m.persistTournamentHeader(run); err != nil {
		slog.Warn("persist tournament header", "id", id, "err", err)
	}

	sprtEnabled := spec.Format == "match" && spec.SprtAlpha > 0 && spec.SprtBeta > 0
	candidateName := ""
	if sprtEnabled && len(engineSpecs) >= 1 {
		candidateName = engineSpecs[0].Name
	}

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
			if err := m.persistGame(id, o); err != nil {
				slog.Warn("persist game", "tournament", id, "game", o.Pairing.GameNumber, "err", err)
			}
			m.emit("tournament:gameComplete", map[string]interface{}{
				"tournamentId": id,
				"row":          gameOutcomeRow(o),
				"pgn":          o.PGN,
			})
		},
		OnGameMove: func(p tournament.Pairing, rec tournament.MoveRecord, fen string) {
			payload := map[string]interface{}{
				"tournamentId": id,
				"gameNumber":   p.GameNumber,
				"ply":          rec.Ply,
				"side":         string(rec.Side),
				"uci":          rec.UCI,
				"san":          rec.SAN,
				"fen":          fen,
				"depth":        rec.Info.Depth,
				"elapsedMs":    rec.Elapsed.Milliseconds(),
				"clockAfterMs": rec.ClockAfter.Milliseconds(),
				"check":        rec.Check,
			}
			if rec.HasInfo {
				if rec.Info.Score.Mate != nil {
					payload["evalMate"] = *rec.Info.Score.Mate
				} else if rec.Info.Score.Centipawns != nil {
					payload["evalCp"] = *rec.Info.Score.Centipawns
				}
			}
			m.emit("tournament:move", payload)
		},
		OnGameInfo: func(p tournament.Pairing, side string, info uci.AnalysisInfo) {
			engineName := p.White.Name
			if side == "b" {
				engineName = p.Black.Name
			}
			payload := map[string]interface{}{
				"tournamentId": id,
				"gameNumber":   p.GameNumber,
				"side":         side,
				"engine":       engineName,
				"depth":        info.Depth,
				"selDepth":     info.SelDepth,
				"nodes":        info.Nodes,
				"nps":          info.NPS,
				"timeMs":       info.Time.Milliseconds(),
				"pv":           info.PV,
				"multiPV":      info.MultiPV,
			}
			if info.Score.Mate != nil {
				payload["evalMate"] = *info.Score.Mate
			} else if info.Score.Centipawns != nil {
				payload["evalCp"] = *info.Score.Centipawns
			}
			m.emit("tournament:engineInfo", payload)
		},
	}

	if sprtEnabled {
		sprtCfg := tournament.SPRTConfig{
			Elo0:  spec.SprtElo0,
			Elo1:  spec.SprtElo1,
			Alpha: spec.SprtAlpha,
			Beta:  spec.SprtBeta,
		}
		// Initialize Sprt state with bounds so the UI can render the gauge
		// before any games complete.
		lower, upper := sprtCfg.Bounds()
		run.mu.Lock()
		run.sprt = &SprtState{LowerBound: lower, UpperBound: upper, Decision: "continue"}
		run.mu.Unlock()
		var w, d, l int
		cfg.ShouldStop = tournament.NewSPRTStopperWithProgress(
			sprtCfg, candidateName,
			func(r tournament.SPRTResult) {
				// Tally W/D/L for the candidate using the latest run state.
				run.mu.Lock()
				outcomes := run.outcomes
				w, d, l = sprtTally(outcomes, candidateName)
				run.sprt = &SprtState{
					LLR:        r.LLR,
					LowerBound: r.LowerBound,
					UpperBound: r.UpperBound,
					Decision:   r.Decision.String(),
					Wins:       w, Draws: d, Losses: l,
				}
				snapshot := *run.sprt
				run.mu.Unlock()
				m.emit("tournament:sprt", map[string]interface{}{
					"tournamentId": id,
					"sprt":         snapshot,
				})
			},
			nil,
		)
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
		if err := m.persistTournamentFinal(run); err != nil {
			slog.Warn("persist tournament final", "id", id, "err", err)
		}
		m.emit("tournament:done", map[string]interface{}{
			"tournamentId": id,
			"status":       final,
		})
	}()

	return id, nil
}

// Delete removes a finished tournament from in-memory state and the
// database. Returns an error if the tournament is still running.
func (m *TournamentManager) Delete(id string) error {
	m.mu.Lock()
	run, ok := m.runs[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("tournament not found: %s", id)
	}
	run.mu.Lock()
	status := run.status
	run.mu.Unlock()
	if status == "running" {
		return fmt.Errorf("cannot delete running tournament %s; stop it first", id)
	}
	if m.db != nil {
		if err := m.db.DeleteTournament(context.Background(), id); err != nil && !errors.Is(err, database.ErrNotFound) {
			return fmt.Errorf("delete tournament: %w", err)
		}
	}
	m.mu.Lock()
	delete(m.runs, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	return nil
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

// GetTournamentPGN returns the concatenated PGN of every game in a tournament.
func (m *TournamentManager) GetTournamentPGN(id string) (string, error) {
	m.mu.Lock()
	run, ok := m.runs[id]
	m.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("tournament not found: %s", id)
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	var sb strings.Builder
	for _, o := range run.outcomes {
		if o.PGN == "" {
			continue
		}
		sb.WriteString(o.PGN)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
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
			Check:        game.InCheck(),
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
