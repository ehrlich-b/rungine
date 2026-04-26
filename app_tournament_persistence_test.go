package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"rungine/internal/chess"
	"rungine/internal/database"
	"rungine/internal/tournament"
	"rungine/internal/uci"
)

func mkOutcome(num int, white, black string, result chess.Outcome, reason chess.Reason) tournament.GameOutcome {
	cp := 25
	return tournament.GameOutcome{
		Pairing: tournament.Pairing{
			GameNumber: num, Round: "1.1",
			White: tournament.EngineSpec{Name: white, BinaryPath: "/usr/bin/" + white},
			Black: tournament.EngineSpec{Name: black, BinaryPath: "/usr/bin/" + black},
		},
		PGN: "[Event \"Test\"]\n1. e4 e5\n",
		Result: &tournament.Result{
			Outcome:    result,
			Reason:     reason,
			PlyCount:   2,
			WhiteClock: 60 * time.Second,
			BlackClock: 59500 * time.Millisecond,
			StartedAt:  time.Now().UTC().Truncate(time.Millisecond),
			EndedAt:    time.Now().UTC().Truncate(time.Millisecond).Add(time.Second),
			Moves: []tournament.MoveRecord{
				{
					Ply: 1, Side: chess.White, UCI: "e2e4", SAN: "e4",
					HasInfo: true,
					Info:    uci.AnalysisInfo{Depth: 12, Score: uci.Score{Centipawns: &cp}},
					Elapsed: 500 * time.Millisecond, ClockAfter: 59500 * time.Millisecond,
				},
				{
					Ply: 2, Side: chess.Black, UCI: "e7e5", SAN: "e5",
					Elapsed: 500 * time.Millisecond, ClockAfter: 59 * time.Second,
				},
			},
		},
	}
}

func TestPersistRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "rungine.db")

	// First "session": persist a tournament + games.
	db1, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}
	mgr1 := newTournamentManager(nil, db1)
	mgr1.bindContext(context.Background())

	// Manually plant a tournament header (Start() requires installer; bypass).
	run := &tournamentRun{
		id:      "t1",
		spec:    TournamentSpec{Format: "match", Engines: []TournamentEngineRef{{ID: "stockfish"}, {ID: "berserk"}}, Games: 2},
		status:  "running",
		started: time.Now().UTC().Truncate(time.Millisecond),
		total:   2,
	}
	mgr1.mu.Lock()
	mgr1.runs[run.id] = run
	mgr1.order = append(mgr1.order, run.id)
	mgr1.counter = 1
	mgr1.mu.Unlock()
	if err := mgr1.persistTournamentHeader(run); err != nil {
		t.Fatalf("persist header: %v", err)
	}

	o1 := mkOutcome(1, "Stockfish", "Berserk", chess.WhiteWins, chess.ReasonCheckmate)
	o2 := mkOutcome(2, "Berserk", "Stockfish", chess.Drawn, chess.ReasonThreefold)
	if err := mgr1.persistGame("t1", o1); err != nil {
		t.Fatalf("persist game1: %v", err)
	}
	if err := mgr1.persistGame("t1", o2); err != nil {
		t.Fatalf("persist game2: %v", err)
	}

	// Mark final.
	finished := time.Now().UTC().Truncate(time.Millisecond)
	run.mu.Lock()
	run.status = "done"
	run.finished = &finished
	run.mu.Unlock()
	if err := mgr1.persistTournamentFinal(run); err != nil {
		t.Fatalf("persist final: %v", err)
	}
	_ = db1.Close()

	// Second "session": fresh manager hydrates from disk.
	db2, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()
	mgr2 := newTournamentManager(nil, db2)
	mgr2.bindContext(context.Background())
	if err := mgr2.hydrateFromDB(context.Background()); err != nil {
		t.Fatalf("hydrate: %v", err)
	}

	list := mgr2.List()
	if len(list) != 1 {
		t.Fatalf("hydrate: want 1 tournament, got %d", len(list))
	}
	got := list[0]
	if got.ID != "t1" || got.Status != "done" {
		t.Fatalf("hydrated status: %+v", got)
	}
	if got.GamesTotal != 2 || got.GamesPlayed != 2 {
		t.Fatalf("hydrated counts: total=%d played=%d", got.GamesTotal, got.GamesPlayed)
	}
	if len(got.Outcomes) != 2 {
		t.Fatalf("hydrated outcomes: %d", len(got.Outcomes))
	}
	if got.Outcomes[0].Outcome != "1-0" || got.Outcomes[0].Reason != "checkmate" {
		t.Fatalf("game1 outcome wrong: %+v", got.Outcomes[0])
	}
	if got.Outcomes[1].Outcome != "1/2-1/2" || got.Outcomes[1].Reason != "threefold repetition" {
		t.Fatalf("game2 outcome wrong: %+v", got.Outcomes[1])
	}
	if got.Spec.Format != "match" || got.Spec.Games != 2 {
		t.Fatalf("hydrated spec wrong: %+v", got.Spec)
	}

	// Counter should advance past hydrated IDs so a new Start uses t2.
	if mgr2.counter < 1 {
		t.Fatalf("counter not advanced: %d", mgr2.counter)
	}

	// GetGameDetail should reconstruct the per-ply replay.
	detail, err := mgr2.GetGameDetail("t1", 1)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if len(detail.Moves) != 2 {
		t.Fatalf("detail moves: %d", len(detail.Moves))
	}
	if detail.Moves[0].UCI != "e2e4" || detail.Moves[0].SAN != "e4" {
		t.Fatalf("move 1 mismatch: %+v", detail.Moves[0])
	}
	if detail.Moves[0].EvalCp == nil || *detail.Moves[0].EvalCp != 25 {
		t.Fatalf("eval round-trip lost: %+v", detail.Moves[0])
	}
}

func TestHydrateMarksRunningInterrupted(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "rungine.db")
	db1, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	mgr := newTournamentManager(nil, db1)
	run := &tournamentRun{
		id: "t9", spec: TournamentSpec{Format: "match"},
		status: "running", started: time.Now(), total: 1,
	}
	mgr.runs[run.id] = run
	mgr.order = append(mgr.order, run.id)
	if err := mgr.persistTournamentHeader(run); err != nil {
		t.Fatalf("persist: %v", err)
	}
	_ = db1.Close()

	db2, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()
	mgr2 := newTournamentManager(nil, db2)
	if err := mgr2.hydrateFromDB(context.Background()); err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	got, err := mgr2.Get("t9")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "interrupted" {
		t.Fatalf("expected interrupted, got %q", got.Status)
	}
}

func TestDeleteTournament(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "rungine.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	mgr := newTournamentManager(nil, db)
	mgr.bindContext(context.Background())

	run := &tournamentRun{id: "t1", spec: TournamentSpec{}, status: "done", started: time.Now(), total: 0}
	mgr.runs[run.id] = run
	mgr.order = append(mgr.order, run.id)
	if err := mgr.persistTournamentHeader(run); err != nil {
		t.Fatalf("persist: %v", err)
	}

	if err := mgr.Delete("t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(mgr.List()) != 0 {
		t.Fatalf("delete left in-memory entry")
	}
	tlist, _ := db.ListTournaments(context.Background())
	if len(tlist) != 0 {
		t.Fatalf("delete left db row: %d", len(tlist))
	}
	// Cannot delete running tournaments.
	mgr.runs["t2"] = &tournamentRun{id: "t2", status: "running"}
	mgr.order = append(mgr.order, "t2")
	if err := mgr.Delete("t2"); err == nil {
		t.Fatalf("expected error deleting running tournament")
	}
}
