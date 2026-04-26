package database

import (
	"context"
	"fmt"
	"time"
)

// SaveGame inserts or replaces a per-game record.
func (d *DB) SaveGame(ctx context.Context, g GameRecord) error {
	if g.TournamentID == "" {
		return fmt.Errorf("save game: tournament id required")
	}
	if g.CompletedAt.IsZero() {
		g.CompletedAt = time.Now()
	}
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO games (tournament_id, game_number, round, white, black,
		                    outcome, reason, plies, error, pgn, detail, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(tournament_id, game_number) DO UPDATE SET
		   round = excluded.round,
		   white = excluded.white,
		   black = excluded.black,
		   outcome = excluded.outcome,
		   reason = excluded.reason,
		   plies = excluded.plies,
		   error = excluded.error,
		   pgn = excluded.pgn,
		   detail = excluded.detail,
		   completed_at = excluded.completed_at`,
		g.TournamentID, g.GameNumber, g.Round, g.White, g.Black,
		g.Outcome, g.Reason, g.Plies, g.Error, g.PGN,
		string(g.Detail), g.CompletedAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("save game: %w", err)
	}
	return nil
}

// ListGames returns every game stored under a tournament, sorted by
// game_number ascending.
func (d *DB) ListGames(ctx context.Context, tournamentID string) ([]GameRecord, error) {
	rows, err := d.sql.QueryContext(ctx,
		`SELECT tournament_id, game_number, round, white, black,
		        outcome, reason, plies, error, pgn, detail, completed_at
		 FROM games WHERE tournament_id = ? ORDER BY game_number ASC`,
		tournamentID)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()
	var out []GameRecord
	for rows.Next() {
		var g GameRecord
		var detail, errMsg string
		var completed int64
		if err := rows.Scan(&g.TournamentID, &g.GameNumber, &g.Round,
			&g.White, &g.Black, &g.Outcome, &g.Reason, &g.Plies, &errMsg,
			&g.PGN, &detail, &completed); err != nil {
			return nil, err
		}
		g.Error = errMsg
		g.Detail = []byte(detail)
		g.CompletedAt = unixMs(completed)
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteTournament removes a tournament and (via FK cascade) its games.
func (d *DB) DeleteTournament(ctx context.Context, id string) error {
	res, err := d.sql.ExecContext(ctx, `DELETE FROM tournaments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete tournament: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func unixMs(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}
