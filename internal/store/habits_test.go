package store

import (
	"context"
	"testing"
	"time"

	"loopline/internal/domain"
)

func TestCreateAndFetchHabit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	h := domain.Habit{
		Title:     "Drink water",
		Schedule:  domain.DailySchedule(),
		Position:  0,
		CreatedOn: domain.MustParseDate("2026-08-01"),
		Notes:     "8 glasses",
	}
	id, err := s.CreateHabit(ctx, h)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero habit ID")
	}

	habits, err := s.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	got := habits[0]
	if got.ID != id || got.Title != h.Title || got.Notes != h.Notes {
		t.Errorf("Habits()[0] = %+v, want id=%d title=%q notes=%q", got, id, h.Title, h.Notes)
	}
	if got.CreatedOn != h.CreatedOn {
		t.Errorf("CreatedOn = %v, want %v", got.CreatedOn, h.CreatedOn)
	}
	if got.ArchivedOn != nil {
		t.Errorf("expected ArchivedOn nil, got %v", got.ArchivedOn)
	}
	if got.RoutineID != nil {
		t.Errorf("expected RoutineID nil, got %v", *got.RoutineID)
	}
	if got.Schedule != domain.DailySchedule() {
		t.Errorf("Schedule = %+v, want daily", got.Schedule)
	}
}

func TestCreateHabitWithRoutine(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	routineID, err := s.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	h := domain.Habit{
		Title:     "Stretch",
		RoutineID: &routineID,
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	}
	id, err := s.CreateHabit(ctx, h)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	habits, err := s.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	var got *domain.Habit
	for i := range habits {
		if habits[i].ID == id {
			got = &habits[i]
		}
	}
	if got == nil {
		t.Fatalf("created habit %d not found in %+v", id, habits)
	}
	if got.RoutineID == nil || *got.RoutineID != routineID {
		t.Errorf("expected RoutineID %d, got %v", routineID, got.RoutineID)
	}
}

func TestUpdateHabit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Journal",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	updated := domain.Habit{
		ID:       id,
		Title:    "Journal nightly",
		Schedule: domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday)},
		Position: 2,
		Notes:    "before bed",
	}
	if err := s.UpdateHabit(ctx, updated); err != nil {
		t.Fatalf("UpdateHabit: %v", err)
	}

	habits, err := s.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("expected 1 habit, got %d", len(habits))
	}
	got := habits[0]
	if got.Title != "Journal nightly" || got.Position != 2 || got.Notes != "before bed" {
		t.Errorf("Habits()[0] = %+v after update", got)
	}
	if got.Schedule.Kind != domain.ScheduleWeekdays || got.Schedule.Weekdays != updated.Schedule.Weekdays {
		t.Errorf("Schedule = %+v, want %+v", got.Schedule, updated.Schedule)
	}
	// UpdateHabit must not touch created_on.
	if got.CreatedOn != domain.MustParseDate("2026-08-01") {
		t.Errorf("CreatedOn changed by UpdateHabit: got %v", got.CreatedOn)
	}
}

func TestArchiveHabitPreservesRow(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Old habit",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	archivedOn := domain.MustParseDate("2026-08-05")
	if err := s.ArchiveHabit(ctx, id, archivedOn); err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}

	habits, err := s.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("expected archived habit to still be returned (soft delete only), got %d habits", len(habits))
	}
	if habits[0].ArchivedOn == nil || *habits[0].ArchivedOn != archivedOn {
		t.Errorf("ArchivedOn = %v, want %v", habits[0].ArchivedOn, archivedOn)
	}
}
