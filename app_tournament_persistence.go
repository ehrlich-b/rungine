package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"rungine/internal/chess"
	"rungine/internal/database"
	"rungine/internal/tournament"
	"rungine/internal/uci"
)

// persistedOutcome is the JSON shape stored in games.detail. It carries
// just enough data to rebuild a tournament.GameOutcome for replay and
// standings/crosstable computation.
type persistedOutcome struct {
	GameNumber int                 `json:"gameNumber"`
	Round      string              `json:"round"`
	White      persistedEngineSpec `json:"white"`
	Black      persistedEngineSpec `json:"black"`
	StartFEN   string              `json:"startFen,omitempty"`
	StartMoves []string            `json:"startMoves,omitempty"`
	PGN        string              `json:"pgn,omitempty"`
	Error      string              `json:"error,omitempty"`
	Result     *persistedResult    `json:"result,omitempty"`
}

type persistedEngineSpec struct {
	Name       string            `json:"name"`
	BinaryPath string            `json:"binaryPath,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type persistedResult struct {
	Outcome      string          `json:"outcome"`
	Reason       string          `json:"reason"`
	Loser        string          `json:"loser,omitempty"`
	PlyCount     int             `json:"plyCount"`
	WhiteClockMs int64           `json:"whiteClockMs"`
	BlackClockMs int64           `json:"blackClockMs"`
	StartedAt    time.Time       `json:"startedAt,omitempty"`
	EndedAt      time.Time       `json:"endedAt,omitempty"`
	Moves        []persistedMove `json:"moves,omitempty"`
	Cause        string          `json:"cause,omitempty"`
}

type persistedMove struct {
	Ply          int    `json:"ply"`
	Side         string `json:"side"`
	UCI          string `json:"uci"`
	SAN          string `json:"san"`
	HasInfo      bool   `json:"hasInfo,omitempty"`
	Depth        int    `json:"depth,omitempty"`
	EvalCp       *int   `json:"evalCp,omitempty"`
	EvalMate     *int   `json:"evalMate,omitempty"`
	ElapsedMs    int64  `json:"elapsedMs"`
	ClockAfterMs int64  `json:"clockAfterMs"`
}

func toPersistedOutcome(o tournament.GameOutcome) persistedOutcome {
	po := persistedOutcome{
		GameNumber: o.Pairing.GameNumber,
		Round:      o.Pairing.Round,
		White:      toPersistedEngineSpec(o.Pairing.White),
		Black:      toPersistedEngineSpec(o.Pairing.Black),
		StartFEN:   o.Pairing.StartFEN,
		StartMoves: append([]string(nil), o.Pairing.StartMoves...),
		PGN:        o.PGN,
	}
	if o.Err != nil {
		po.Error = o.Err.Error()
	}
	if o.Result != nil {
		pr := &persistedResult{
			Outcome:      string(o.Result.Outcome),
			Reason:       string(o.Result.Reason),
			Loser:        o.Result.Loser,
			PlyCount:     o.Result.PlyCount,
			WhiteClockMs: o.Result.WhiteClock.Milliseconds(),
			BlackClockMs: o.Result.BlackClock.Milliseconds(),
			StartedAt:    o.Result.StartedAt,
			EndedAt:      o.Result.EndedAt,
		}
		if o.Result.Cause != nil {
			pr.Cause = o.Result.Cause.Error()
		}
		pr.Moves = make([]persistedMove, 0, len(o.Result.Moves))
		for _, m := range o.Result.Moves {
			pm := persistedMove{
				Ply: m.Ply, Side: string(m.Side),
				UCI: m.UCI, SAN: m.SAN,
				HasInfo:      m.HasInfo,
				ElapsedMs:    m.Elapsed.Milliseconds(),
				ClockAfterMs: m.ClockAfter.Milliseconds(),
			}
			if m.HasInfo {
				pm.Depth = m.Info.Depth
				if m.Info.Score.Mate != nil {
					v := *m.Info.Score.Mate
					pm.EvalMate = &v
				} else if m.Info.Score.Centipawns != nil {
					v := *m.Info.Score.Centipawns
					pm.EvalCp = &v
				}
			}
			pr.Moves = append(pr.Moves, pm)
		}
		po.Result = pr
	}
	return po
}

func toPersistedEngineSpec(s tournament.EngineSpec) persistedEngineSpec {
	out := persistedEngineSpec{Name: s.Name, BinaryPath: s.BinaryPath}
	if len(s.Options) > 0 {
		out.Options = map[string]string{}
		for k, v := range s.Options {
			out.Options[k] = v
		}
	}
	return out
}

func fromPersistedOutcome(po persistedOutcome) tournament.GameOutcome {
	out := tournament.GameOutcome{
		Pairing: tournament.Pairing{
			GameNumber: po.GameNumber,
			Round:      po.Round,
			White:      fromPersistedEngineSpec(po.White),
			Black:      fromPersistedEngineSpec(po.Black),
			StartFEN:   po.StartFEN,
			StartMoves: append([]string(nil), po.StartMoves...),
		},
		PGN: po.PGN,
	}
	if po.Error != "" {
		out.Err = errors.New(po.Error)
	}
	if po.Result != nil {
		r := &tournament.Result{
			Outcome:    chess.Outcome(po.Result.Outcome),
			Reason:     chess.Reason(po.Result.Reason),
			Loser:      po.Result.Loser,
			PlyCount:   po.Result.PlyCount,
			WhiteClock: time.Duration(po.Result.WhiteClockMs) * time.Millisecond,
			BlackClock: time.Duration(po.Result.BlackClockMs) * time.Millisecond,
			StartedAt:  po.Result.StartedAt,
			EndedAt:    po.Result.EndedAt,
		}
		if po.Result.Cause != "" {
			r.Cause = errors.New(po.Result.Cause)
		}
		r.Moves = make([]tournament.MoveRecord, 0, len(po.Result.Moves))
		for _, m := range po.Result.Moves {
			mr := tournament.MoveRecord{
				Ply:        m.Ply,
				Side:       chess.Side(m.Side),
				UCI:        m.UCI,
				SAN:        m.SAN,
				HasInfo:    m.HasInfo,
				Elapsed:    time.Duration(m.ElapsedMs) * time.Millisecond,
				ClockAfter: time.Duration(m.ClockAfterMs) * time.Millisecond,
			}
			if m.HasInfo {
				mr.Info = uci.AnalysisInfo{Depth: m.Depth}
				if m.EvalMate != nil {
					v := *m.EvalMate
					mr.Info.Score.Mate = &v
				} else if m.EvalCp != nil {
					v := *m.EvalCp
					mr.Info.Score.Centipawns = &v
				}
			}
			r.Moves = append(r.Moves, mr)
		}
		out.Result = r
	}
	return out
}

func fromPersistedEngineSpec(p persistedEngineSpec) tournament.EngineSpec {
	out := tournament.EngineSpec{Name: p.Name, BinaryPath: p.BinaryPath}
	if len(p.Options) > 0 {
		out.Options = map[string]string{}
		for k, v := range p.Options {
			out.Options[k] = v
		}
	}
	return out
}

// persistGame stores one finished game's outcome to the database.
func (m *TournamentManager) persistGame(tournamentID string, o tournament.GameOutcome) error {
	if m.db == nil {
		return nil
	}
	po := toPersistedOutcome(o)
	detail, err := json.Marshal(po)
	if err != nil {
		return fmt.Errorf("marshal game detail: %w", err)
	}
	rec := database.GameRecord{
		TournamentID: tournamentID,
		GameNumber:   o.Pairing.GameNumber,
		Round:        o.Pairing.Round,
		White:        o.Pairing.White.Name,
		Black:        o.Pairing.Black.Name,
		PGN:          o.PGN,
		Detail:       detail,
		CompletedAt:  time.Now(),
	}
	if o.Err != nil {
		rec.Error = o.Err.Error()
	}
	if o.Result != nil {
		rec.Outcome = string(o.Result.Outcome)
		rec.Reason = string(o.Result.Reason)
		rec.Plies = o.Result.PlyCount
	}
	return m.db.SaveGame(context.Background(), rec)
}

// persistTournamentHeader saves the initial tournament header on Start.
func (m *TournamentManager) persistTournamentHeader(run *tournamentRun) error {
	if m.db == nil {
		return nil
	}
	spec, err := json.Marshal(run.spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}
	rec := database.TournamentRecord{
		ID:         run.id,
		Spec:       spec,
		Status:     run.status,
		Error:      run.errStr,
		StartedAt:  run.started,
		GamesTotal: run.total,
	}
	return m.db.SaveTournament(context.Background(), rec)
}

// persistTournamentFinal updates the header after the scheduler exits.
func (m *TournamentManager) persistTournamentFinal(run *tournamentRun) error {
	if m.db == nil {
		return nil
	}
	run.mu.Lock()
	id := run.id
	status := run.status
	errMsg := run.errStr
	var finished *int64
	if run.finished != nil {
		v := run.finished.UnixMilli()
		finished = &v
	}
	var sprt []byte
	if run.sprt != nil {
		if b, err := json.Marshal(run.sprt); err == nil {
			sprt = b
		}
	}
	run.mu.Unlock()
	return m.db.UpdateTournamentStatus(context.Background(), id, status, errMsg, finished, sprt)
}

// hydrateFromDB loads stored tournaments and games into the in-memory
// runs map so List/Get serve historical entries after a restart.
func (m *TournamentManager) hydrateFromDB(ctx context.Context) error {
	if m.db == nil {
		return nil
	}
	tournaments, err := m.db.ListTournaments(ctx)
	if err != nil {
		return fmt.Errorf("list tournaments: %w", err)
	}
	// ListTournaments returns newest first. We want oldest first in m.order
	// so the GUI's append-on-create model still works.
	for i := len(tournaments) - 1; i >= 0; i-- {
		t := tournaments[i]
		var spec TournamentSpec
		if len(t.Spec) > 0 {
			if err := json.Unmarshal(t.Spec, &spec); err != nil {
				return fmt.Errorf("decode spec for %s: %w", t.ID, err)
			}
		}
		run := &tournamentRun{
			id:      t.ID,
			spec:    spec,
			status:  t.Status,
			errStr:  t.Error,
			started: t.StartedAt,
			total:   t.GamesTotal,
		}
		if t.FinishedAt != nil {
			f := *t.FinishedAt
			run.finished = &f
		}
		// Status fixup: a "running" row from a prior process is now stale.
		if run.status == "running" {
			run.status = "interrupted"
		}
		if len(t.Sprt) > 0 {
			var s SprtState
			if err := json.Unmarshal(t.Sprt, &s); err == nil {
				run.sprt = &s
			}
		}
		games, err := m.db.ListGames(ctx, t.ID)
		if err != nil {
			return fmt.Errorf("list games for %s: %w", t.ID, err)
		}
		for _, g := range games {
			var po persistedOutcome
			if len(g.Detail) > 0 {
				if err := json.Unmarshal(g.Detail, &po); err != nil {
					return fmt.Errorf("decode game %s/%d: %w", t.ID, g.GameNumber, err)
				}
			} else {
				po = persistedOutcome{
					GameNumber: g.GameNumber, Round: g.Round,
					White: persistedEngineSpec{Name: g.White},
					Black: persistedEngineSpec{Name: g.Black},
					PGN:   g.PGN,
				}
			}
			run.outcomes = append(run.outcomes, fromPersistedOutcome(po))
		}
		m.mu.Lock()
		if _, exists := m.runs[t.ID]; !exists {
			m.runs[t.ID] = run
			m.order = append(m.order, t.ID)
			// Bump counter past hydrated IDs so new tournaments don't collide.
			if n := parseRunCounter(t.ID); n > m.counter {
				m.counter = n
			}
		}
		m.mu.Unlock()
	}
	return nil
}

// parseRunCounter pulls the numeric suffix from "tNNN" run IDs, returning
// 0 for unparseable IDs (e.g. user-supplied or future formats).
func parseRunCounter(id string) int {
	if len(id) < 2 || id[0] != 't' {
		return 0
	}
	n := 0
	for _, c := range id[1:] {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
