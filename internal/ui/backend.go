// Package ui implements the Bubble Tea v2 presentation layer for
// loopline: three keyboard-first content panels with responsive global
// navigation, driven entirely by read-only Snapshots pulled through Backend.
// The UI never touches SQLite
// and never calls time.Now(); "today" always comes from Backend.Today.
package ui

import (
	"context"

	"loopline/internal/app"
	"loopline/internal/domain"
)

// MinWidth is the narrowest terminal width the compact three-panel layout
// renders in. Below this, Model shows a centred minimum-size message instead.
const MinWidth = 96

// MinHeight is the shortest comfortable three-panel layout. At 33 rows the
// one-line footer and two border rows leave 30 content rows: a six-week month
// still leaves 15 reminder-list rows and the Overview has ten spare rows.
const MinHeight = 33

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
	EditHabit(ctx context.Context, id int64, in app.NewHabit) error
	EditRoutine(ctx context.Context, id int64, in app.NewRoutine) error
	EditReminder(ctx context.Context, id int64, in app.NewReminder) error
	ToggleReminder(ctx context.Context, id int64) error
	DeleteHabit(ctx context.Context, id int64) error
	DeleteRoutine(ctx context.Context, id int64) error
	DeleteReminder(ctx context.Context, id int64) error
}
