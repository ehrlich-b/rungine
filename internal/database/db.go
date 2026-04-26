// Package database persists tournament runs and games to SQLite.
//
// The DB is intentionally narrow: it stores tournament headers, per-game
// outcomes, and enough JSON to reconstruct a replay. Live state (engines,
// runtime context) lives in TournamentManager and is not persisted.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB is a handle to the rungine SQLite database.
type DB struct {
	sql *sql.DB
}

// Open opens the database at path, creating the file (and parent
// directories) if missing, and applies any pending migrations. If path is
// empty it defaults to ~/.rungine/rungine.db.
func Open(path string) (*DB, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		path = filepath.Join(home, ".rungine", "rungine.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	sqldb.SetMaxOpenConns(1) // SQLite write serialization; cheaper than retry.

	d := &DB{sql: sqldb}
	if err := d.migrate(context.Background()); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// Close releases the underlying connections.
func (d *DB) Close() error {
	if d == nil || d.sql == nil {
		return nil
	}
	return d.sql.Close()
}

// migrations is the ordered list of SQL statements to bring the schema
// up to date. Each entry is one migration; never edit a published entry,
// always append.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY);

	CREATE TABLE IF NOT EXISTS tournaments (
		id TEXT PRIMARY KEY,
		spec TEXT NOT NULL,
		status TEXT NOT NULL,
		error TEXT NOT NULL DEFAULT '',
		started_at INTEGER NOT NULL,
		finished_at INTEGER,
		games_total INTEGER NOT NULL,
		sprt TEXT
	);

	CREATE TABLE IF NOT EXISTS games (
		tournament_id TEXT NOT NULL,
		game_number INTEGER NOT NULL,
		round TEXT NOT NULL,
		white TEXT NOT NULL,
		black TEXT NOT NULL,
		outcome TEXT NOT NULL,
		reason TEXT NOT NULL,
		plies INTEGER NOT NULL,
		error TEXT NOT NULL DEFAULT '',
		pgn TEXT NOT NULL DEFAULT '',
		detail TEXT NOT NULL,
		completed_at INTEGER NOT NULL,
		PRIMARY KEY (tournament_id, game_number),
		FOREIGN KEY (tournament_id) REFERENCES tournaments(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS games_tournament_idx ON games(tournament_id);`,
}

func (d *DB) migrate(ctx context.Context) error {
	current := 0
	// schema_version may not exist yet on a fresh database; either way we
	// just need the highest applied version, defaulting to 0.
	var hasTable int
	if err := d.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_version'`,
	).Scan(&hasTable); err != nil {
		return fmt.Errorf("schema probe: %w", err)
	}
	if hasTable == 1 {
		if err := d.sql.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(version), 0) FROM schema_version",
		).Scan(&current); err != nil {
			return fmt.Errorf("read schema version: %w", err)
		}
	}
	for i, stmt := range migrations {
		ver := i + 1
		if ver <= current {
			continue
		}
		tx, err := d.sql.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d: %w", ver, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_version(version) VALUES (?)", ver); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d record: %w", ver, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
