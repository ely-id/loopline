package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migration is one ordered, versioned schema change. Each migration runs
// inside its own transaction and, once applied, is never edited again — a
// schema change ships as a new migration with the next version number.
type migration struct {
	Version int
	SQL     string
}

var migrations = []migration{
	{Version: 1, SQL: schemaV1},
}

const schemaV1 = `
CREATE TABLE routines (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  created_on TEXT NOT NULL,
  archived_on TEXT
);
CREATE TABLE habits (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  routine_id INTEGER REFERENCES routines(id) ON DELETE SET NULL,
  schedule_kind TEXT NOT NULL,
  schedule_weekdays INTEGER NOT NULL DEFAULT 127,
  position INTEGER NOT NULL DEFAULT 0,
  created_on TEXT NOT NULL,
  archived_on TEXT,
  notes TEXT NOT NULL DEFAULT ''
);
CREATE TABLE completions (
  habit_id INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  date TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  PRIMARY KEY (habit_id, date)
);
CREATE INDEX idx_completions_date ON completions(date);
CREATE TABLE reminders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  date TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  completed INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_reminders_date ON reminders(date);
CREATE TABLE daily_summaries (
  date TEXT PRIMARY KEY,
  scheduled INTEGER NOT NULL,
  completed INTEGER NOT NULL
);
`

// migrate applies every migration not yet recorded in schema_migrations, each
// inside its own transaction, tracked by version number.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range migrations {
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration checks whether m is already recorded and, if not, applies it
// and records it, all inside a single transaction. Doing the check and the
// apply together stops two concurrent first-time opens from both seeing
// "not applied" and racing to run the same migration twice.
func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.Version, err)
	}
	defer tx.Rollback()

	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, m.Version).Scan(&n); err != nil {
		return fmt.Errorf("check migration %d: %w", m.Version, err)
	}
	if n > 0 {
		return nil
	}

	if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
		return fmt.Errorf("apply migration %d: %w", m.Version, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, m.Version); err != nil {
		return fmt.Errorf("record migration %d: %w", m.Version, err)
	}
	return tx.Commit()
}
