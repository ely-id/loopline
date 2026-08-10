package store

import (
	"context"
	"testing"

	"routine-tui/internal/domain"
)

func TestCreateAndFetchRoutine(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateRoutine(ctx, domain.Routine{Title: "Morning", Position: 1, CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	routines, err := s.Routines(ctx)
	if err != nil {
		t.Fatalf("Routines: %v", err)
	}
	if len(routines) != 1 || routines[0].ID != id || routines[0].Title != "Morning" {
		t.Fatalf("Routines() = %+v", routines)
	}
	if routines[0].ArchivedOn != nil {
		t.Errorf("expected ArchivedOn nil, got %v", routines[0].ArchivedOn)
	}
}

func TestUpdateRoutine(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	if err := s.UpdateRoutine(ctx, domain.Routine{ID: id, Title: "Morning routine", Position: 3}); err != nil {
		t.Fatalf("UpdateRoutine: %v", err)
	}

	routines, err := s.Routines(ctx)
	if err != nil {
		t.Fatalf("Routines: %v", err)
	}
	if routines[0].Title != "Morning routine" || routines[0].Position != 3 {
		t.Errorf("Routines()[0] = %+v after update", routines[0])
	}
}

// TestDeleteRoutineDetachesHabitsWithoutDestroyingThem verifies the CRITICAL
// rule from ARCHITECTURE.md §4: deleting a routine hard-deletes the routine
// row but must never destroy its habits.
func TestDeleteRoutineDetachesHabitsWithoutDestroyingThem(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	routineID, err := s.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}
	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Stretch",
		RoutineID: &routineID,
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	if err := s.DeleteRoutine(ctx, routineID); err != nil {
		t.Fatalf("DeleteRoutine: %v", err)
	}

	routines, err := s.Routines(ctx)
	if err != nil {
		t.Fatalf("Routines: %v", err)
	}
	if len(routines) != 0 {
		t.Fatalf("expected routine to be hard-deleted, got %+v", routines)
	}

	habits, err := s.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("expected habit to survive routine deletion, got %d habits", len(habits))
	}
	if habits[0].ID != habitID {
		t.Fatalf("unexpected habit after routine deletion: %+v", habits[0])
	}
	if habits[0].RoutineID != nil {
		t.Errorf("expected habit RoutineID to be nil after routine deletion, got %v", *habits[0].RoutineID)
	}
}
