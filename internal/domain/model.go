package domain

import "time"

// Routine is an optional grouping of habits. Deleting a routine never deletes
// its habits; they fall back to the ungrouped list.
type Routine struct {
	ID         int64
	Title      string
	Position   int
	CreatedOn  Date
	ArchivedOn *Date // nil while active
}

// Habit is a recurring action the user wants to perform.
type Habit struct {
	ID         int64
	Title      string
	RoutineID  *int64 // nil means the habit has no routine
	Schedule   Schedule
	Position   int
	CreatedOn  Date
	ArchivedOn *Date // nil while active; set instead of deleting so history stays correct
	Notes      string
}

// Completion records that a habit was completed on a given calendar date. It is
// the source of truth for all history and statistics.
type Completion struct {
	HabitID     int64
	Date        Date
	CompletedAt time.Time
}

// Reminder is a dated thing the user must not forget.
type Reminder struct {
	ID        int64
	Title     string
	Date      Date
	Notes     string
	Completed bool
}

// ScheduleKind selects how a Schedule decides which dates it applies to.
type ScheduleKind string

const (
	// ScheduleDaily applies to every calendar date.
	ScheduleDaily ScheduleKind = "daily"
	// ScheduleWeekdays applies to the weekdays listed in Schedule.Weekdays.
	ScheduleWeekdays ScheduleKind = "weekdays"
)

// WeekdaySet is a bitmask of weekdays, bit 0 being Sunday through bit 6 being
// Saturday, matching time.Weekday ordering.
type WeekdaySet uint8

// AllWeekdays is every day of the week.
const AllWeekdays WeekdaySet = 0x7F

// Schedule decides whether a habit applies on a given calendar date.
type Schedule struct {
	Kind     ScheduleKind
	Weekdays WeekdaySet // only meaningful when Kind is ScheduleWeekdays
}

// DailySchedule is the default schedule for a new habit.
func DailySchedule() Schedule { return Schedule{Kind: ScheduleDaily, Weekdays: AllWeekdays} }

// DailySummary is the completion state of a single calendar date.
type DailySummary struct {
	Date      Date
	Scheduled int
	Completed int
}
