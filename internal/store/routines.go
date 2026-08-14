package store

import (
	"context"
	"database/sql"
	"fmt"

	"loopline/internal/domain"
)

const routineColumns = `id, title, position, created_on, archived_on`

// Routines returns every routine, archived ones included, ordered by
// position.
func (s *SQLite) Routines(ctx context.Context) ([]domain.Routine, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+routineColumns+` FROM routines ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("query routines: %w", err)
	}
	defer rows.Close()

	var out []domain.Routine
	for rows.Next() {
		r, err := scanRoutine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routines: %w", err)
	}
	return out, nil
}

// CreateRoutine inserts a routine and returns its new ID.
func (s *SQLite) CreateRoutine(ctx context.Context, r domain.Routine) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO routines (title, position, created_on, archived_on)
		VALUES (?, ?, ?, ?)`,
		r.Title, r.Position, r.CreatedOn.String(), nullableDate(r.ArchivedOn),
	)
	if err != nil {
		return 0, fmt.Errorf("insert routine: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("routine last insert id: %w", err)
	}
	return id, nil
}

// UpdateRoutine persists title and position.
func (s *SQLite) UpdateRoutine(ctx context.Context, r domain.Routine) error {
	_, err := s.db.ExecContext(ctx, `UPDATE routines SET title = ?, position = ? WHERE id = ?`, r.Title, r.Position, r.ID)
	if err != nil {
		return fmt.Errorf("update routine %d: %w", r.ID, err)
	}
	return nil
}

// DeleteRoutine removes a routine and detaches its habits (routine_id ->
// NULL) in one transaction. Habits are never destroyed. The schema's
// ON DELETE SET NULL foreign key backs this up as a second line of defence.
func (s *SQLite) DeleteRoutine(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete routine %d: %w", id, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE habits SET routine_id = NULL WHERE routine_id = ?`, id); err != nil {
		return fmt.Errorf("detach habits from routine %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM routines WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete routine %d: %w", id, err)
	}
	return tx.Commit()
}

func scanRoutine(rows *sql.Rows) (domain.Routine, error) {
	var (
		r          domain.Routine
		createdOn  string
		archivedOn sql.NullString
	)
	if err := rows.Scan(&r.ID, &r.Title, &r.Position, &createdOn, &archivedOn); err != nil {
		return domain.Routine{}, fmt.Errorf("scan routine: %w", err)
	}

	created, err := domain.ParseDate(createdOn)
	if err != nil {
		return domain.Routine{}, fmt.Errorf("parse routine created_on: %w", err)
	}
	r.CreatedOn = created

	archived, err := parseNullableDate(archivedOn)
	if err != nil {
		return domain.Routine{}, fmt.Errorf("parse routine archived_on: %w", err)
	}
	r.ArchivedOn = archived

	return r, nil
}
