// Package demo seeds a fresh database with representative data for previews,
// screenshots, and portfolio recordings.
package demo

import (
	"context"
	"fmt"
	"time"

	"loopline/internal/app"
	"loopline/internal/domain"
)

type seededHabit struct {
	id       int64
	schedule domain.Schedule
}

// Seed populates an empty store with a small but varied history. It refuses to
// append to an existing database so repeated demo runs cannot silently create
// duplicate routines, habits, or reminders.
func Seed(ctx context.Context, s app.Store, today domain.Date) error {
	habits, err := s.Habits(ctx)
	if err != nil {
		return fmt.Errorf("check demo habits: %w", err)
	}
	routines, err := s.Routines(ctx)
	if err != nil {
		return fmt.Errorf("check demo routines: %w", err)
	}
	reminders, err := s.Reminders(ctx)
	if err != nil {
		return fmt.Errorf("check demo reminders: %w", err)
	}
	if len(habits) != 0 || len(routines) != 0 || len(reminders) != 0 {
		return fmt.Errorf("demo data requires an empty database")
	}

	morningID, err := s.CreateRoutine(ctx, domain.Routine{
		Title:     "Morning ritual",
		Position:  0,
		CreatedOn: today.AddDays(-45),
	})
	if err != nil {
		return fmt.Errorf("create morning routine: %w", err)
	}
	focusID, err := s.CreateRoutine(ctx, domain.Routine{
		Title:     "Deep work",
		Position:  1,
		CreatedOn: today.AddDays(-30),
	})
	if err != nil {
		return fmt.Errorf("create focus routine: %w", err)
	}
	wellbeingID, err := s.CreateRoutine(ctx, domain.Routine{
		Title:     "Wellbeing",
		Position:  2,
		CreatedOn: today.AddDays(-24),
	})
	if err != nil {
		return fmt.Errorf("create wellbeing routine: %w", err)
	}

	seeded := make([]seededHabit, 0, 5)
	for _, spec := range []struct {
		title    string
		routine  int64
		schedule domain.Schedule
		created  domain.Date
		notes    string
	}{
		{"Drink water", morningID, domain.DailySchedule(), today.AddDays(-45), "Keep a glass nearby."},
		{"Read 20 pages", morningID, domain.DailySchedule(), today.AddDays(-42), "A little progress every day."},
		{"Plan the day", focusID, domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)}, today.AddDays(-30), "Choose the three most important tasks."},
		{"Stretch", wellbeingID, domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Monday, time.Wednesday, time.Friday)}, today.AddDays(-24), "Five minutes is enough."},
		{"Walk after lunch", wellbeingID, domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Monday, time.Tuesday, time.Thursday, time.Friday)}, today.AddDays(-21), "Take the long way around."},
	} {
		routineID := spec.routine
		id, err := s.CreateHabit(ctx, domain.Habit{
			Title:     spec.title,
			RoutineID: &routineID,
			Schedule:  spec.schedule,
			Position:  len(seeded),
			CreatedOn: spec.created,
			Notes:     spec.notes,
		})
		if err != nil {
			return fmt.Errorf("create demo habit %q: %w", spec.title, err)
		}
		seeded = append(seeded, seededHabit{id: id, schedule: spec.schedule})
	}

	if err := seedCompletions(ctx, s, today, seeded); err != nil {
		return err
	}

	for _, reminder := range []domain.Reminder{
		{Title: "Review quarterly goals", Date: today.AddDays(-6), Notes: "Capture what worked and what needs attention.", Completed: true},
		{Title: "Prepare weekly plan", Date: today, Notes: "Set up the next focused work block."},
		{Title: "Call mom", Date: today.AddDays(3), Notes: "Sunday afternoon."},
		{Title: "Book dentist appointment", Date: today.AddDays(8), Notes: "Ask for an early morning slot."},
		{Title: "Publish portfolio update", Date: today.AddDays(14), Notes: "Include the loopline walkthrough."},
	} {
		if _, err := s.CreateReminder(ctx, reminder); err != nil {
			return fmt.Errorf("create demo reminder %q: %w", reminder.Title, err)
		}
	}

	return nil
}

func seedCompletions(ctx context.Context, s app.Store, today domain.Date, habits []seededHabit) error {
	start := today.AddDays(-45)
	for d := start; !d.After(today); d = d.AddDays(1) {
		scheduled := make([]int64, 0, len(habits))
		for i, habit := range habits {
			if !habit.schedule.AppliesOn(d) {
				continue
			}
			scheduled = append(scheduled, habit.id)
			if !completedOn(i, d, today) {
				continue
			}
			at := d.Time().Add(time.Duration(7+i*2) * time.Hour)
			if err := s.SetCompletion(ctx, habit.id, d, true, at); err != nil {
				return fmt.Errorf("seed completion habit=%d date=%s: %w", habit.id, d, err)
			}
		}
		if err := s.RefreshDailySummary(ctx, d, scheduled); err != nil {
			return fmt.Errorf("refresh demo summary date=%s: %w", d, err)
		}
	}
	return nil
}

func completedOn(habit int, d, today domain.Date) bool {
	if d.Equal(today) {
		return habit != 4
	}
	day := d.Day
	switch habit {
	case 0:
		return day%10 != 3
	case 1:
		return day%4 != 0
	case 2:
		return d.Weekday() != time.Friday || day%3 != 0
	case 3:
		return day%5 != 0
	case 4:
		return day%3 != 0
	default:
		return false
	}
}
