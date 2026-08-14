package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"loopline/internal/domain"
)

// CompletionsBetween returns completions in [from, to] inclusive.
func (s *SQLite) CompletionsBetween(ctx context.Context, from, to domain.Date) ([]domain.Completion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT habit_id, date, completed_at FROM completions
		WHERE date >= ? AND date <= ?
		ORDER BY date, habit_id`,
		from.String(), to.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query completions: %w", err)
	}
	defer rows.Close()

	var out []domain.Completion
	for rows.Next() {
		var (
			c           domain.Completion
			date        string
			completedAt string
		)
		if err := rows.Scan(&c.HabitID, &date, &completedAt); err != nil {
			return nil, fmt.Errorf("scan completion: %w", err)
		}
		d, err := domain.ParseDate(date)
		if err != nil {
			return nil, fmt.Errorf("parse completion date: %w", err)
		}
		c.Date = d
		at, err := time.Parse(time.RFC3339Nano, completedAt)
		if err != nil {
			return nil, fmt.Errorf("parse completion timestamp: %w", err)
		}
		c.CompletedAt = at
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completions: %w", err)
	}
	return out, nil
}

// SetCompletion marks a habit complete or incomplete on a date, idempotently:
// marking an already-complete date just refreshes completed_at, and clearing
// an already-incomplete date is a harmless no-op.
func (s *SQLite) SetCompletion(ctx context.Context, habitID int64, on domain.Date, done bool, at time.Time) error {
	if done {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO completions (habit_id, date, completed_at)
			VALUES (?, ?, ?)
			ON CONFLICT (habit_id, date) DO UPDATE SET completed_at = excluded.completed_at`,
			habitID, on.String(), at.UTC().Format(time.RFC3339Nano),
		)
		if err != nil {
			return fmt.Errorf("set completion habit=%d date=%s: %w", habitID, on, err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM completions WHERE habit_id = ? AND date = ?`, habitID, on.String()); err != nil {
		return fmt.Errorf("clear completion habit=%d date=%s: %w", habitID, on, err)
	}
	return nil
}

// ToggleCompletion flips the completion for a habit on a date and rewrites
// the derived daily summary for that date, both inside one transaction, then
// reports the resulting state. The summary's completed count is recounted
// from the completions table as committed inside this same transaction —
// never from a caller-supplied value — which is what stops daily_summaries
// from drifting from completions when a failure lands between the two writes
// or when two toggles race.
func (s *SQLite) ToggleCompletion(ctx context.Context, habitID int64, on domain.Date, at time.Time, scheduledHabitIDs []int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin toggle completion habit=%d date=%s: %w", habitID, on, err)
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM completions WHERE habit_id = ? AND date = ?`, habitID, on.String()).Scan(&exists)
	var done bool
	switch {
	case err == nil:
		if _, err := tx.ExecContext(ctx, `DELETE FROM completions WHERE habit_id = ? AND date = ?`, habitID, on.String()); err != nil {
			return false, fmt.Errorf("toggle off habit=%d date=%s: %w", habitID, on, err)
		}
		done = false
	case errors.Is(err, sql.ErrNoRows):
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO completions (habit_id, date, completed_at) VALUES (?, ?, ?)`,
			habitID, on.String(), at.UTC().Format(time.RFC3339Nano),
		); err != nil {
			return false, fmt.Errorf("toggle on habit=%d date=%s: %w", habitID, on, err)
		}
		done = true
	default:
		return false, fmt.Errorf("check completion habit=%d date=%s: %w", habitID, on, err)
	}

	if err := writeSummaryTx(ctx, tx, on, scheduledHabitIDs); err != nil {
		return false, fmt.Errorf("toggle completion habit=%d date=%s: %w", habitID, on, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit toggle completion habit=%d date=%s: %w", habitID, on, err)
	}
	return done, nil
}

// RefreshDailySummary recomputes and stores the derived summary for a date
// in one transaction, recounting completions inside it. Used after habit set
// changes, which alter the scheduled count without touching completions.
func (s *SQLite) RefreshDailySummary(ctx context.Context, on domain.Date, scheduledHabitIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refresh daily summary date=%s: %w", on, err)
	}
	defer tx.Rollback()

	if err := writeSummaryTx(ctx, tx, on, scheduledHabitIDs); err != nil {
		return fmt.Errorf("refresh daily summary date=%s: %w", on, err)
	}
	return tx.Commit()
}

// writeSummaryTx recounts completions for on from committed table state
// inside tx and upserts the daily_summaries row to match. scheduledHabitIDs
// carries only the domain's scheduling decision; the completed count is
// never taken from a caller-supplied value, so the summary can never drift
// from what the transaction actually committed to completions.
func writeSummaryTx(ctx context.Context, tx *sql.Tx, on domain.Date, scheduledHabitIDs []int64) error {
	query := `SELECT COUNT(*) FROM completions WHERE date = ?`
	args := []any{on.String()}
	if len(scheduledHabitIDs) > 0 {
		placeholders := strings.Repeat("?,", len(scheduledHabitIDs))
		placeholders = placeholders[:len(placeholders)-1]
		query += fmt.Sprintf(" AND habit_id IN (%s)", placeholders)
		for _, id := range scheduledHabitIDs {
			args = append(args, id)
		}
	} else {
		query += ` AND 1 = 0`
	}

	var completed int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&completed); err != nil {
		return fmt.Errorf("recount completions: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO daily_summaries (date, scheduled, completed)
		VALUES (?, ?, ?)
		ON CONFLICT (date) DO UPDATE SET scheduled = excluded.scheduled, completed = excluded.completed`,
		on.String(), len(scheduledHabitIDs), completed,
	); err != nil {
		return fmt.Errorf("upsert daily summary: %w", err)
	}
	return nil
}
