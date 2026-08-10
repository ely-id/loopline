package store

import (
	"context"
	"database/sql"
	"fmt"

	"routine-tui/internal/domain"
)

// PutDailySummary writes the derived snapshot for a date (insert or
// replace). daily_summaries is a derived, write-through table; callers must
// never read it back as a source of truth for analytics.
func (s *SQLite) PutDailySummary(ctx context.Context, sum domain.DailySummary) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_summaries (date, scheduled, completed)
		VALUES (?, ?, ?)
		ON CONFLICT (date) DO UPDATE SET scheduled = excluded.scheduled, completed = excluded.completed`,
		sum.Date.String(), sum.Scheduled, sum.Completed,
	)
	if err != nil {
		return fmt.Errorf("put daily summary %s: %w", sum.Date, err)
	}
	return nil
}

// DailySummaries returns stored snapshots in [from, to] inclusive.
func (s *SQLite) DailySummaries(ctx context.Context, from, to domain.Date) ([]domain.DailySummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date, scheduled, completed FROM daily_summaries
		WHERE date >= ? AND date <= ?
		ORDER BY date`,
		from.String(), to.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("query daily summaries: %w", err)
	}
	defer rows.Close()

	var out []domain.DailySummary
	for rows.Next() {
		var (
			sum  domain.DailySummary
			date string
		)
		if err := rows.Scan(&date, &sum.Scheduled, &sum.Completed); err != nil {
			return nil, fmt.Errorf("scan daily summary: %w", err)
		}
		d, err := domain.ParseDate(date)
		if err != nil {
			return nil, fmt.Errorf("parse daily summary date: %w", err)
		}
		sum.Date = d
		out = append(out, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily summaries: %w", err)
	}
	return out, nil
}

// FirstActivityDate returns the earliest date the database knows about
// (earliest habit creation or completion). ok is false when empty.
func (s *SQLite) FirstActivityDate(ctx context.Context) (domain.Date, bool, error) {
	var earliest sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT MIN(d) FROM (
			SELECT MIN(created_on) AS d FROM habits
			UNION ALL
			SELECT MIN(date) AS d FROM completions
		)`,
	).Scan(&earliest)
	if err != nil {
		return domain.Date{}, false, fmt.Errorf("query first activity date: %w", err)
	}
	if !earliest.Valid {
		return domain.Date{}, false, nil
	}
	d, err := domain.ParseDate(earliest.String)
	if err != nil {
		return domain.Date{}, false, fmt.Errorf("parse first activity date: %w", err)
	}
	return d, true, nil
}
