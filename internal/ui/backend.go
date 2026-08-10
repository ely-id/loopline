// Package ui implements the Bubble Tea v2 presentation layer for
// routine-tui: a three-column, keyboard-first layout driven entirely by
// read-only Snapshots pulled through Backend. The UI never touches SQLite
// and never calls time.Now(); "today" always comes from Backend.Today.
package ui

import (
	"context"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
)

// MinWidth is the narrowest terminal width the three-column layout renders
// in. Below this, Model shows a centred minimum-size message instead.
const MinWidth = 96

// MinHeight is the shortest terminal height the layout renders in. The
// overview column's content (actions, a six-week calendar, the week strip
// and the heatmap) is a fixed 31 lines regardless of terminal size; 35 = 31
// + 2 (column border) + 2 (footer) is the smallest height that renders it
// without MaxHeight silently clipping the bottom.
const MinHeight = 35

// Backend is everything the UI needs from the application layer. It is
// declared here, not in internal/app, so tests can supply a fake without
// pulling in storage.
type Backend interface {
	Today() domain.Date
	Snapshot(ctx context.Context, req app.SnapshotRequest) (app.Snapshot, error)
	ToggleHabit(ctx context.Context, habitID int64) error
	AddHabit(ctx context.Context, in app.NewHabit) error
	AddRoutine(ctx context.Context, in app.NewRoutine) error
	AddReminder(ctx context.Context, in app.NewReminder) error
	ToggleReminder(ctx context.Context, id int64) error
	DeleteHabit(ctx context.Context, id int64) error
	DeleteRoutine(ctx context.Context, id int64) error
	DeleteReminder(ctx context.Context, id int64) error
}
