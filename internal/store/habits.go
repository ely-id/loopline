package store

import (
	"context"
	"database/sql"
	"fmt"

	"routine-tui/internal/domain"
)

const habitColumns = `id, title, routine_id, schedule_kind, schedule_weekdays, position, created_on, archived_on, notes`

// Habits returns every habit, archived ones included, ordered by position.
func (s *SQLite) Habits(ctx context.Context) ([]domain.Habit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+habitColumns+` FROM habits ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("query habits: %w", err)
	}
	defer rows.Close()

	var out []domain.Habit
	for rows.Next() {
		h, err := scanHabit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate habits: %w", err)
	}
	return out, nil
}

// CreateHabit inserts a habit and returns its new ID.
func (s *SQLite) CreateHabit(ctx context.Context, h domain.Habit) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO habits (title, routine_id, schedule_kind, schedule_weekdays, position, created_on, archived_on, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		h.Title, nullableInt64(h.RoutineID), string(h.Schedule.Kind), int(h.Schedule.Weekdays), h.Position,
		h.CreatedOn.String(), nullableDate(h.ArchivedOn), h.Notes,
	)
	if err != nil {
		return 0, fmt.Errorf("insert habit: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("habit last insert id: %w", err)
	}
	return id, nil
}

// UpdateHabit persists title, routine membership, schedule, position and
// notes. CreatedOn and ArchivedOn are untouched; use ArchiveHabit for those.
func (s *SQLite) UpdateHabit(ctx context.Context, h domain.Habit) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE habits
		SET title = ?, routine_id = ?, schedule_kind = ?, schedule_weekdays = ?, position = ?, notes = ?
		WHERE id = ?`,
		h.Title, nullableInt64(h.RoutineID), string(h.Schedule.Kind), int(h.Schedule.Weekdays), h.Position, h.Notes, h.ID,
	)
	if err != nil {
		return fmt.Errorf("update habit %d: %w", h.ID, err)
	}
	return nil
}

// ArchiveHabit soft-deletes a habit from the given date onward, preserving
// history. Habits are never hard-deleted.
func (s *SQLite) ArchiveHabit(ctx context.Context, id int64, on domain.Date) error {
	_, err := s.db.ExecContext(ctx, `UPDATE habits SET archived_on = ? WHERE id = ?`, on.String(), id)
	if err != nil {
		return fmt.Errorf("archive habit %d: %w", id, err)
	}
	return nil
}

func scanHabit(rows *sql.Rows) (domain.Habit, error) {
	var (
		h            domain.Habit
		routineID    sql.NullInt64
		scheduleKind string
		weekdays     int
		createdOn    string
		archivedOn   sql.NullString
	)
	if err := rows.Scan(&h.ID, &h.Title, &routineID, &scheduleKind, &weekdays, &h.Position, &createdOn, &archivedOn, &h.Notes); err != nil {
		return domain.Habit{}, fmt.Errorf("scan habit: %w", err)
	}
	if routineID.Valid {
		v := routineID.Int64
		h.RoutineID = &v
	}
	h.Schedule = domain.Schedule{Kind: domain.ScheduleKind(scheduleKind), Weekdays: domain.WeekdaySet(weekdays)}

	created, err := domain.ParseDate(createdOn)
	if err != nil {
		return domain.Habit{}, fmt.Errorf("parse habit created_on: %w", err)
	}
	h.CreatedOn = created

	archived, err := parseNullableDate(archivedOn)
	if err != nil {
		return domain.Habit{}, fmt.Errorf("parse habit archived_on: %w", err)
	}
	h.ArchivedOn = archived

	return h, nil
}
