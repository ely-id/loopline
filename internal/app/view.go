package app

import (
	"time"

	"routine-tui/internal/domain"
)

// StatsRange is the selectable window of the heatmap / analytics block.
type StatsRange string

// Selectable analytics ranges, in the order the UI cycles through them.
const (
	RangeWeek  StatsRange = "week"
	RangeMonth StatsRange = "month"
	Range90d   StatsRange = "90d"
	RangeYear  StatsRange = "year"
	RangeAll   StatsRange = "all"
)

// StatsRanges is the ordered list of selectable ranges.
var StatsRanges = []StatsRange{RangeWeek, RangeMonth, Range90d, RangeYear, RangeAll}

// Label is the short human label shown in the UI.
func (r StatsRange) Label() string {
	switch r {
	case RangeWeek:
		return "week"
	case RangeMonth:
		return "month"
	case Range90d:
		return "90 days"
	case RangeYear:
		return "year"
	case RangeAll:
		return "all"
	}
	return string(r)
}

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

// CalendarDay is one cell of the month calendar.
type CalendarDay struct {
	Date        domain.Date
	InMonth     bool
	IsToday     bool
	IsFuture    bool
	Summary     domain.DailySummary
	HasReminder bool
}

// CalendarMonth is a Monday-first month grid; Weeks always holds full rows of
// seven days, padded with adjacent-month days flagged InMonth false.
type CalendarMonth struct {
	Year  int
	Month time.Month
	Weeks [][]CalendarDay
}

// WeekView is the current ISO week block.
type WeekView struct {
	ISOYear int
	ISOWeek int
	Start   domain.Date
	Days    []CalendarDay // exactly 7, Monday first
}

// HeatmapCell is one contribution square.
type HeatmapCell struct {
	Date    domain.Date
	Summary domain.DailySummary
	// Level buckets completion for colouring: 0 none, 1..4 increasing.
	Level int
}

// Heatmap is the contribution view for the selected range. Columns are weeks
// (Monday first); each column holds seven cells, oldest week first. Cells
// outside the range are zero-valued with a zero Date.
type Heatmap struct {
	Range   StatsRange
	Start   domain.Date
	End     domain.Date
	Columns [][]HeatmapCell
}

// Overview is the lifetime summary block.
type Overview struct {
	Today            domain.Date
	CurrentStreak    int
	BestStreak       int
	TotalCompletions int
	ActiveHabits     int
	ActiveRoutines   int
	DaysTracked      int
	LifetimePercent  int
	OpenReminders    int
}

// Snapshot is everything the UI needs for one render pass. The UI never queries
// storage itself; it asks for a Snapshot and replaces its state wholesale.
type Snapshot struct {
	Today     TodayView
	Overview  Overview
	Calendar  CalendarMonth
	Week      WeekView
	Heatmap   Heatmap
	Reminders []domain.Reminder
	Routines  []domain.Routine
}

// SnapshotRequest selects the view parameters that the user can change.
type SnapshotRequest struct {
	// Month is any date inside the month the calendar should display.
	// Zero means the month containing today.
	Month domain.Date
	// Range selects the heatmap window. Empty means RangeMonth.
	Range StatsRange
}

// NewHabit is the input for creating a habit.
type NewHabit struct {
	Title     string
	RoutineID *int64
	Schedule  domain.Schedule
	Notes     string
}

// NewRoutine is the input for creating a routine.
type NewRoutine struct {
	Title string
}

// NewReminder is the input for creating a reminder.
type NewReminder struct {
	Title string
	Date  domain.Date
	Notes string
}
