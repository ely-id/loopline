package store

import (
	"context"
	"fmt"

	"routine-tui/internal/domain"
)

const reminderColumns = `id, title, date, notes, completed`

// Reminders returns every reminder ordered by date then title.
func (s *SQLite) Reminders(ctx context.Context) ([]domain.Reminder, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+reminderColumns+` FROM reminders ORDER BY date, title`)
	if err != nil {
		return nil, fmt.Errorf("query reminders: %w", err)
	}
	defer rows.Close()

	var out []domain.Reminder
	for rows.Next() {
		var (
			r         domain.Reminder
			date      string
			completed int
		)
		if err := rows.Scan(&r.ID, &r.Title, &date, &r.Notes, &completed); err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		d, err := domain.ParseDate(date)
		if err != nil {
			return nil, fmt.Errorf("parse reminder date: %w", err)
		}
		r.Date = d
		r.Completed = completed != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}
	return out, nil
}

// CreateReminder inserts a reminder and returns its new ID.
func (s *SQLite) CreateReminder(ctx context.Context, r domain.Reminder) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO reminders (title, date, notes, completed)
		VALUES (?, ?, ?, ?)`,
		r.Title, r.Date.String(), r.Notes, boolToInt(r.Completed),
	)
	if err != nil {
		return 0, fmt.Errorf("insert reminder: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reminder last insert id: %w", err)
	}
	return id, nil
}

// UpdateReminder persists title, date, notes and completed state.
func (s *SQLite) UpdateReminder(ctx context.Context, r domain.Reminder) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE reminders SET title = ?, date = ?, notes = ?, completed = ? WHERE id = ?`,
		r.Title, r.Date.String(), r.Notes, boolToInt(r.Completed), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update reminder %d: %w", r.ID, err)
	}
	return nil
}

// DeleteReminder removes a reminder permanently.
func (s *SQLite) DeleteReminder(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM reminders WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete reminder %d: %w", id, err)
	}
	return nil
}
