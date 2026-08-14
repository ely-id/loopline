// Package app coordinates the application: it owns the use cases, the storage
// contract and the read models handed to the UI.
//
// Dependency direction is cmd/loopline -> ui -> app -> domain, with
// internal/store implementing the Store interface declared here. app never
// imports ui or store.
package app

import (
	"context"
	"time"

	"loopline/internal/domain"
)

// Store is the persistence contract. It is declared by its consumer (app) and
// implemented by internal/store. Implementations must be safe for sequential
// use from the Bubble Tea command goroutines.
type Store interface {
	// Habits returns every habit, archived ones included, ordered by position.
	Habits(ctx context.Context) ([]domain.Habit, error)
	// CreateHabit inserts a habit and returns its new ID.
	CreateHabit(ctx context.Context, h domain.Habit) (int64, error)
	// UpdateHabit persists title, routine membership, schedule, position and notes.
	UpdateHabit(ctx context.Context, h domain.Habit) error
	// ArchiveHabit soft-deletes a habit from the given date onward, preserving history.
	ArchiveHabit(ctx context.Context, id int64, on domain.Date) error

	// Routines returns every routine, archived ones included, ordered by position.
	Routines(ctx context.Context) ([]domain.Routine, error)
	// CreateRoutine inserts a routine and returns its new ID.
	CreateRoutine(ctx context.Context, r domain.Routine) (int64, error)
	// UpdateRoutine persists title and position.
	UpdateRoutine(ctx context.Context, r domain.Routine) error
	// DeleteRoutine removes a routine and detaches its habits (routine_id -> NULL)
	// in one transaction. Habits are never destroyed.
	DeleteRoutine(ctx context.Context, id int64) error

	// CompletionsBetween returns completions in [from, to] inclusive.
	CompletionsBetween(ctx context.Context, from, to domain.Date) ([]domain.Completion, error)
	// SetCompletion marks a habit complete or incomplete on a date, idempotently.
	SetCompletion(ctx context.Context, habitID int64, on domain.Date, done bool, at time.Time) error
	// ToggleCompletion flips the completion for a habit on a date AND rewrites
	// the derived daily summary for that date, both inside a single
	// transaction, then reports the resulting state. Doing the read, the write
	// and the summary together is what stops two racing toggles from losing an
	// update and what stops daily_summaries from drifting from completions.
	//
	// scheduledHabitIDs is the set of habits the domain considers scheduled on
	// that date. The summary's scheduled count is its length; its completed
	// count is recounted from the completions table inside the transaction, so
	// it reflects the committed state rather than a stale pre-read.
	ToggleCompletion(ctx context.Context, habitID int64, on domain.Date, at time.Time, scheduledHabitIDs []int64) (done bool, err error)
	// RefreshDailySummary recomputes and stores the derived summary for a date
	// in one transaction, recounting completions inside it. Used after habit
	// set changes, which alter the scheduled count without touching completions.
	RefreshDailySummary(ctx context.Context, on domain.Date, scheduledHabitIDs []int64) error

	// Reminders returns every reminder ordered by date then title.
	Reminders(ctx context.Context) ([]domain.Reminder, error)
	// CreateReminder inserts a reminder and returns its new ID.
	CreateReminder(ctx context.Context, r domain.Reminder) (int64, error)
	// UpdateReminder persists title, date, notes and completed state.
	UpdateReminder(ctx context.Context, r domain.Reminder) error
	// DeleteReminder removes a reminder permanently.
	DeleteReminder(ctx context.Context, id int64) error

	// PutDailySummary writes the derived snapshot for a date (insert or replace).
	PutDailySummary(ctx context.Context, s domain.DailySummary) error
	// DailySummaries returns stored snapshots in [from, to] inclusive.
	DailySummaries(ctx context.Context, from, to domain.Date) ([]domain.DailySummary, error)

	// FirstActivityDate returns the earliest date the database knows about
	// (earliest habit creation or completion). ok is false when empty.
	FirstActivityDate(ctx context.Context) (d domain.Date, ok bool, err error)

	// Close releases the underlying database handle.
	Close() error
}
