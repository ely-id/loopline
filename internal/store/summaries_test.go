package store

import (
	"context"
	"testing"
	"time"

	"loopline/internal/domain"
)

func TestPutDailySummaryUpsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	d := domain.MustParseDate("2026-08-10")
	if err := s.PutDailySummary(ctx, domain.DailySummary{Date: d, Scheduled: 3, Completed: 1}); err != nil {
		t.Fatalf("PutDailySummary: %v", err)
	}
	if err := s.PutDailySummary(ctx, domain.DailySummary{Date: d, Scheduled: 3, Completed: 3}); err != nil {
		t.Fatalf("PutDailySummary (overwrite): %v", err)
	}

	got, err := s.DailySummaries(ctx, d, d)
	if err != nil {
		t.Fatalf("DailySummaries: %v", err)
	}
	if len(got) != 1 || got[0].Completed != 3 {
		t.Fatalf("DailySummaries = %+v, want single row with Completed=3", got)
	}
}

func TestDailySummariesRange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, ds := range []string{"2026-08-01", "2026-08-05", "2026-08-10"} {
		if err := s.PutDailySummary(ctx, domain.DailySummary{Date: domain.MustParseDate(ds), Scheduled: 1, Completed: 1}); err != nil {
			t.Fatalf("PutDailySummary(%s): %v", ds, err)
		}
	}

	got, err := s.DailySummaries(ctx, domain.MustParseDate("2026-08-02"), domain.MustParseDate("2026-08-10"))
	if err != nil {
		t.Fatalf("DailySummaries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 summaries in range, got %d", len(got))
	}
}

func TestFirstActivityDateEmpty(t *testing.T) {
	s := newTestStore(t)
	_, ok, err := s.FirstActivityDate(context.Background())
	if err != nil {
		t.Fatalf("FirstActivityDate: %v", err)
	}
	if ok {
		t.Error("expected ok=false for an empty database")
	}
}

func TestFirstActivityDateUsesEarliestOfHabitsAndCompletions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Old habit",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-06-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	// A completion earlier than the habit's own creation date (e.g. imported
	// history) should still be picked up as the earliest activity.
	earlier := domain.MustParseDate("2026-05-15")
	if err := s.SetCompletion(ctx, habitID, earlier, true, time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("SetCompletion: %v", err)
	}

	got, ok, err := s.FirstActivityDate(ctx)
	if err != nil {
		t.Fatalf("FirstActivityDate: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != earlier {
		t.Errorf("FirstActivityDate = %v, want %v", got, earlier)
	}
}
