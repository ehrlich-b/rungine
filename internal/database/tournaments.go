package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by Get* when no row matches.
var ErrNotFound = errors.New("not found")

// SaveTournament inserts or replaces the tournament header. Use this on
// tournament start (status=running) and again whenever the header
// metadata changes.
func (d *DB) SaveTournament(ctx context.Context, t TournamentRecord) error {
	if t.ID == "" {
		return errors.New("tournament id required")
	}
	var finished *int64
	if t.FinishedAt != nil {
		v := t.FinishedAt.UnixMilli()
		finished = &v
	}
	var sprt any
	if len(t.Sprt) > 0 {
		sprt = string(t.Sprt)
	}
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO tournaments (id, spec, status, error, started_at, finished_at, games_total, sprt)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   spec = excluded.spec,
		   status = excluded.status,
		   error = excluded.error,
		   started_at = excluded.started_at,
		   finished_at = excluded.finished_at,
		   games_total = excluded.games_total,
		   sprt = excluded.sprt`,
		t.ID, string(t.Spec), t.Status, t.Error,
		t.StartedAt.UnixMilli(), finished, t.GamesTotal, sprt,
	)
	if err != nil {
		return fmt.Errorf("save tournament: %w", err)
	}
	return nil
}

// UpdateTournamentStatus updates only the mutable fields of a tournament.
// finished may be nil to clear it.
func (d *DB) UpdateTournamentStatus(ctx context.Context, id, status, errMsg string, finished *int64, sprt []byte) error {
	var sprtVal any
	if len(sprt) > 0 {
		sprtVal = string(sprt)
	}
	res, err := d.sql.ExecContext(ctx,
		`UPDATE tournaments SET status = ?, error = ?, finished_at = ?, sprt = ? WHERE id = ?`,
		status, errMsg, finished, sprtVal, id,
	)
	if err != nil {
		return fmt.Errorf("update tournament status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListTournaments returns all tournaments, newest start time first.
func (d *DB) ListTournaments(ctx context.Context) ([]TournamentRecord, error) {
	rows, err := d.sql.QueryContext(ctx,
		`SELECT id, spec, status, error, started_at, finished_at, games_total, sprt
		 FROM tournaments ORDER BY started_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list tournaments: %w", err)
	}
	defer rows.Close()
	var out []TournamentRecord
	for rows.Next() {
		t, err := scanTournament(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTournament fetches a single tournament header. Returns ErrNotFound
// when id is unknown.
func (d *DB) GetTournament(ctx context.Context, id string) (TournamentRecord, error) {
	row := d.sql.QueryRowContext(ctx,
		`SELECT id, spec, status, error, started_at, finished_at, games_total, sprt
		 FROM tournaments WHERE id = ?`, id)
	t, err := scanTournament(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TournamentRecord{}, ErrNotFound
	}
	return t, err
}

// rowScanner abstracts *sql.Row and *sql.Rows so scanTournament can serve both.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTournament(r rowScanner) (TournamentRecord, error) {
	var t TournamentRecord
	var spec, errMsg, status string
	var startedAt int64
	var finishedAt sql.NullInt64
	var sprt sql.NullString
	var id string
	var gamesTotal int
	if err := r.Scan(&id, &spec, &status, &errMsg, &startedAt, &finishedAt, &gamesTotal, &sprt); err != nil {
		return t, err
	}
	t.ID = id
	t.Spec = []byte(spec)
	t.Status = status
	t.Error = errMsg
	t.StartedAt = unixMs(startedAt)
	if finishedAt.Valid {
		v := unixMs(finishedAt.Int64)
		t.FinishedAt = &v
	}
	t.GamesTotal = gamesTotal
	if sprt.Valid {
		t.Sprt = []byte(sprt.String)
	}
	return t, nil
}
