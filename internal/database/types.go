package database

import (
	"encoding/json"
	"time"
)

// TournamentRecord is the persisted header for a tournament run.
//
// Spec and Sprt are opaque JSON payloads owned by the caller (the
// app-layer struct shapes are not imported here to keep the database
// package independent of the rest of the codebase).
type TournamentRecord struct {
	ID         string
	Spec       json.RawMessage
	Status     string
	Error      string
	StartedAt  time.Time
	FinishedAt *time.Time
	GamesTotal int
	Sprt       json.RawMessage
}

// GameRecord is one finished game inside a tournament. Detail is the
// JSON-encoded blob the app needs to reconstruct a per-ply replay.
type GameRecord struct {
	TournamentID string
	GameNumber   int
	Round        string
	White        string
	Black        string
	Outcome      string
	Reason       string
	Plies        int
	Error        string
	PGN          string
	Detail       json.RawMessage
	CompletedAt  time.Time
}
