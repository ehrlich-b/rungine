package database

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "rungine.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestOpenAndMigrateIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rungine.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	if err := db2.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestSaveAndListTournaments(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	specA, _ := json.Marshal(map[string]any{"format": "match", "games": 4})
	specB, _ := json.Marshal(map[string]any{"format": "round-robin", "games": 1})

	if err := db.SaveTournament(ctx, TournamentRecord{
		ID: "t1", Spec: specA, Status: "running", StartedAt: now, GamesTotal: 4,
	}); err != nil {
		t.Fatalf("save t1: %v", err)
	}
	if err := db.SaveTournament(ctx, TournamentRecord{
		ID: "t2", Spec: specB, Status: "done", StartedAt: now.Add(time.Second), GamesTotal: 6,
	}); err != nil {
		t.Fatalf("save t2: %v", err)
	}

	list, err := db.ListTournaments(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 tournaments, got %d", len(list))
	}
	// Newest start time first.
	if list[0].ID != "t2" || list[1].ID != "t1" {
		t.Fatalf("ordering wrong: %v", []string{list[0].ID, list[1].ID})
	}
	if list[1].Status != "running" || list[0].Status != "done" {
		t.Fatalf("status mismatch: %s / %s", list[1].Status, list[0].Status)
	}
}

func TestUpdateTournamentStatus(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.SaveTournament(ctx, TournamentRecord{
		ID: "t1", Spec: json.RawMessage(`{}`), Status: "running",
		StartedAt: time.Now(), GamesTotal: 2,
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	finished := time.Now().UnixMilli()
	if err := db.UpdateTournamentStatus(ctx, "t1", "done", "", &finished, []byte(`{"llr":1.2}`)); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := db.GetTournament(ctx, "t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "done" || got.FinishedAt == nil {
		t.Fatalf("status/finishedAt not updated: %+v", got)
	}
	if string(got.Sprt) != `{"llr":1.2}` {
		t.Fatalf("sprt mismatch: %s", got.Sprt)
	}
}

func TestSaveAndListGames(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.SaveTournament(ctx, TournamentRecord{
		ID: "t1", Spec: json.RawMessage(`{}`), Status: "running",
		StartedAt: time.Now(), GamesTotal: 2,
	}); err != nil {
		t.Fatalf("save tournament: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	game1 := GameRecord{
		TournamentID: "t1", GameNumber: 1, Round: "1.1",
		White: "Stockfish", Black: "Lc0", Outcome: "1-0",
		Reason: "checkmate", Plies: 73, PGN: "[Event ...] ...",
		Detail: json.RawMessage(`{"moves":[]}`), CompletedAt: now,
	}
	game2 := GameRecord{
		TournamentID: "t1", GameNumber: 2, Round: "1.2",
		White: "Lc0", Black: "Stockfish", Outcome: "1/2-1/2",
		Reason: "threefold repetition", Plies: 42,
		Detail: json.RawMessage(`{"moves":[]}`), CompletedAt: now.Add(time.Second),
	}
	if err := db.SaveGame(ctx, game1); err != nil {
		t.Fatalf("save game1: %v", err)
	}
	if err := db.SaveGame(ctx, game2); err != nil {
		t.Fatalf("save game2: %v", err)
	}

	games, err := db.ListGames(ctx, "t1")
	if err != nil {
		t.Fatalf("list games: %v", err)
	}
	if len(games) != 2 {
		t.Fatalf("want 2 games, got %d", len(games))
	}
	if games[0].GameNumber != 1 || games[1].GameNumber != 2 {
		t.Fatalf("game order wrong: %d %d", games[0].GameNumber, games[1].GameNumber)
	}
	if string(games[0].Detail) != `{"moves":[]}` {
		t.Fatalf("detail not preserved: %s", games[0].Detail)
	}

	// Upsert: replay save with new outcome should overwrite.
	game1.Outcome = "0-1"
	game1.Reason = "time forfeit"
	if err := db.SaveGame(ctx, game1); err != nil {
		t.Fatalf("upsert game1: %v", err)
	}
	games, _ = db.ListGames(ctx, "t1")
	if games[0].Outcome != "0-1" || games[0].Reason != "time forfeit" {
		t.Fatalf("upsert did not overwrite: %+v", games[0])
	}
}

func TestDeleteTournamentCascadesGames(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := db.SaveTournament(ctx, TournamentRecord{
		ID: "t1", Spec: json.RawMessage(`{}`), Status: "running",
		StartedAt: time.Now(), GamesTotal: 1,
	}); err != nil {
		t.Fatalf("save tournament: %v", err)
	}
	if err := db.SaveGame(ctx, GameRecord{
		TournamentID: "t1", GameNumber: 1, Round: "1.1",
		White: "A", Black: "B", Outcome: "1-0", Reason: "checkmate",
		Detail: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("save game: %v", err)
	}
	if err := db.DeleteTournament(ctx, "t1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	games, err := db.ListGames(ctx, "t1")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(games) != 0 {
		t.Fatalf("FK cascade did not remove games: %d", len(games))
	}
}

func TestGetTournamentNotFound(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.GetTournament(context.Background(), "missing"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
