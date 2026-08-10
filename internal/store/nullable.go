package store

import (
	"database/sql"

	"routine-tui/internal/domain"
)

// nullableDate converts an optional domain.Date to its SQL representation.
func nullableDate(d *domain.Date) sql.NullString {
	if d == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: d.String(), Valid: true}
}

// parseNullableDate is the inverse of nullableDate.
func parseNullableDate(ns sql.NullString) (*domain.Date, error) {
	if !ns.Valid {
		return nil, nil
	}
	d, err := domain.ParseDate(ns.String)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// nullableInt64 converts an optional int64 (e.g. Habit.RoutineID) to its SQL
// representation.
func nullableInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
