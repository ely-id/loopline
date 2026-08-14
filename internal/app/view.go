package app

import (
	"time"

	"loopline/internal/domain"
)

// HabitItem is one actionable habit in today's list.
type HabitItem struct {
	Habit  domain.Habit
	Done   bool
	Streak int // consecutive scheduled days completed up to and including today
}

// HabitGroup is a visual block in the today column: either the ungrouped
// habits (RoutineID nil, Title "") or one routine.
type HabitGroup struct {
	RoutineID *int64
	Title     string
	Items     []HabitItem
	Scheduled int
	Completed int
}

// TodayView is the today column read model.
type TodayView struct {
	Date    domain.Date
	Groups  []HabitGroup
	Summary domain.DailySummary
}

// WeeklyHabit is one habit's schedule and completion state across the current
// Sunday-through-Saturday display week. Scheduled and Completed are deliberately separate so the
// UI can distinguish an inactive day from a scheduled-but-incomplete day
// without inventing a persistent "partial" completion state.
type WeeklyHabit struct {
	Habit domain.Habit
	Days  [7]WeeklyDay
}

// WeeklyDay is one Sunday-first cell in Weekly Overview.
type WeeklyDay struct {
	Date      domain.Date
	Scheduled bool
	Completed bool
	IsToday   bool
	IsFuture  bool
}

// WeeklyOverview is the current Sunday-through-Saturday display week,
// independent of the month selected in either calendar.
type WeeklyOverview struct {
	Start domain.Date
	End   domain.Date
	Rows  []WeeklyHabit
}

// CalendarDay is one cell of the month calendar.
type CalendarDay struct {
	Date        domain.Date
	InMonth     bool
	IsToday     bool
	IsFuture    bool
	Summary     domain.DailySummary
	HasReminder bool
}

// CalendarMonth is a Sunday-first month grid; Weeks always holds full rows of
// seven days, padded with adjacent-month days flagged InMonth false.
type CalendarMonth struct {
	Year  int
	Month time.Month
	Weeks [][]CalendarDay
}

// Overview is the lifetime summary block.
type Overview struct {
	CurrentStreak    int
	BestStreak       int
	TotalCompletions int
	DaysTracked      int
	LifetimePercent  int
}

// Snapshot is everything the UI needs for one render pass. The UI never queries
// storage itself; it asks for a Snapshot and replaces its state wholesale.
type Snapshot struct {
	Today     TodayView
	Weekly    WeeklyOverview
	Overview  Overview
	Calendar  CalendarMonth
	Reminders []domain.Reminder
	Routines  []domain.Routine
}

// SnapshotRequest selects the view parameters that the user can change.
type SnapshotRequest struct {
	// Month is any date inside the month the calendar should display.
	// Zero means the month containing today.
	Month domain.Date
}

// NewHabit contains the editable fields used when creating or updating a habit.
type NewHabit struct {
	Title     string
	RoutineID *int64
	Schedule  domain.Schedule
	Notes     string
}

// NewRoutine contains the editable fields used when creating or updating a routine.
type NewRoutine struct {
	Title string
}

// NewReminder contains the editable fields used when creating or updating a reminder.
type NewReminder struct {
	Title string
	Date  domain.Date
	Notes string
}
