package store

import (
	"context"
	"testing"
	"time"

	"loopline/internal/domain"
)

func TestSetCompletionToggle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Read",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC)

	if err := s.SetCompletion(ctx, habitID, on, true, at); err != nil {
		t.Fatalf("SetCompletion(done=true): %v", err)
	}
	comps, err := s.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 1 || comps[0].HabitID != habitID || comps[0].Date != on {
		t.Fatalf("CompletionsBetween = %+v", comps)
	}
	if !comps[0].CompletedAt.Equal(at) {
		t.Errorf("CompletedAt = %v, want %v", comps[0].CompletedAt, at)
	}

	// Re-marking done should update in place, not duplicate.
	at2 := at.Add(time.Hour)
	if err := s.SetCompletion(ctx, habitID, on, true, at2); err != nil {
		t.Fatalf("SetCompletion(done=true) second time: %v", err)
	}
	comps, err = s.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("expected exactly 1 completion after re-marking done, got %d", len(comps))
	}
	if !comps[0].CompletedAt.Equal(at2) {
		t.Errorf("CompletedAt = %v, want %v", comps[0].CompletedAt, at2)
	}

	// Un-marking removes the row; un-marking again is a harmless no-op.
	if err := s.SetCompletion(ctx, habitID, on, false, at); err != nil {
		t.Fatalf("SetCompletion(done=false): %v", err)
	}
	if err := s.SetCompletion(ctx, habitID, on, false, at); err != nil {
		t.Fatalf("SetCompletion(done=false) again: %v", err)
	}
	comps, err = s.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected 0 completions after unmarking, got %d", len(comps))
	}
}

func TestCompletionsBetweenRange(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Meditate",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	at := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	for _, ds := range []string{"2026-08-01", "2026-08-05", "2026-08-10"} {
		if err := s.SetCompletion(ctx, habitID, domain.MustParseDate(ds), true, at); err != nil {
			t.Fatalf("SetCompletion(%s): %v", ds, err)
		}
	}

	got, err := s.CompletionsBetween(ctx, domain.MustParseDate("2026-08-02"), domain.MustParseDate("2026-08-10"))
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 completions in range, got %d: %+v", len(got), got)
	}
	if got[0].Date != domain.MustParseDate("2026-08-05") || got[1].Date != domain.MustParseDate("2026-08-10") {
		t.Errorf("unexpected dates in range result: %+v", got)
	}
}

func TestToggleCompletion(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Stretch",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	scheduled := []int64{habitID}

	done, err := s.ToggleCompletion(ctx, habitID, on, at, scheduled)
	if err != nil {
		t.Fatalf("ToggleCompletion (first): %v", err)
	}
	if !done {
		t.Fatalf("first toggle on a bare date: done = %v, want true", done)
	}
	comps, err := s.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 1 || !comps[0].CompletedAt.Equal(at) {
		t.Fatalf("expected exactly 1 completion at %v, got %+v", at, comps)
	}

	done, err = s.ToggleCompletion(ctx, habitID, on, at, scheduled)
	if err != nil {
		t.Fatalf("ToggleCompletion (second): %v", err)
	}
	if done {
		t.Fatalf("second toggle: done = %v, want false", done)
	}
	comps, err = s.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 0 {
		t.Fatalf("expected no completion row left behind, got %+v", comps)
	}
}

// summaryFor is a test helper that fetches the single daily_summaries row for
// a date, failing the test if it is missing.
func summaryFor(t *testing.T, s *SQLite, on domain.Date) domain.DailySummary {
	t.Helper()
	got, err := s.DailySummaries(context.Background(), on, on)
	if err != nil {
		t.Fatalf("DailySummaries(%s): %v", on, err)
	}
	if len(got) != 1 {
		t.Fatalf("DailySummaries(%s) = %+v, want exactly one row", on, got)
	}
	return got[0]
}

// TestToggleCompletionWritesSummaryAtomically proves the RELEASE BLOCKER fix:
// toggling a completion and rewriting daily_summaries happen in the same
// transaction, so the two are never observed out of sync.
func TestToggleCompletionWritesSummaryAtomically(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Stretch",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	scheduled := []int64{habitID}

	// Toggle on: completion row and summary row must land together.
	done, err := s.ToggleCompletion(ctx, habitID, on, at, scheduled)
	if err != nil {
		t.Fatalf("ToggleCompletion (on): %v", err)
	}
	if !done {
		t.Fatalf("ToggleCompletion (on): done = %v, want true", done)
	}
	sum := summaryFor(t, s, on)
	if sum.Scheduled != 1 || sum.Completed != 1 {
		t.Fatalf("after toggle-on summary = %+v, want {Scheduled:1 Completed:1}", sum)
	}

	// Toggle off: the summary must be rewritten back down in the same
	// transaction as the completion row's removal.
	done, err = s.ToggleCompletion(ctx, habitID, on, at, scheduled)
	if err != nil {
		t.Fatalf("ToggleCompletion (off): %v", err)
	}
	if done {
		t.Fatalf("ToggleCompletion (off): done = %v, want false", done)
	}
	sum = summaryFor(t, s, on)
	if sum.Scheduled != 1 || sum.Completed != 0 {
		t.Fatalf("after toggle-off summary = %+v, want {Scheduled:1 Completed:0}", sum)
	}
}

// TestToggleCompletionRecountsFromCommittedState proves the completed count
// is recomputed from the completions table inside the transaction, not
// derived from any value the caller might pass in — the point of the fix.
// One habit's toggle must correctly reflect a sibling habit's already
// completed state on the same date.
func TestToggleCompletionRecountsFromCommittedState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitA, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Drink water",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(A): %v", err)
	}
	habitB, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Brush teeth",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(B): %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	scheduled := []int64{habitA, habitB}

	if _, err := s.ToggleCompletion(ctx, habitA, on, at, scheduled); err != nil {
		t.Fatalf("ToggleCompletion(A): %v", err)
	}
	sum := summaryFor(t, s, on)
	if sum.Scheduled != 2 || sum.Completed != 1 {
		t.Fatalf("after A on: summary = %+v, want {Scheduled:2 Completed:1}", sum)
	}

	// Toggling B on must recount and see A's already-committed completion,
	// not just B's own change.
	if _, err := s.ToggleCompletion(ctx, habitB, on, at, scheduled); err != nil {
		t.Fatalf("ToggleCompletion(B): %v", err)
	}
	sum = summaryFor(t, s, on)
	if sum.Scheduled != 2 || sum.Completed != 2 {
		t.Fatalf("after A+B on: summary = %+v, want {Scheduled:2 Completed:2}", sum)
	}

	// Toggling A back off must recount down to just B, not silently subtract.
	if _, err := s.ToggleCompletion(ctx, habitA, on, at, scheduled); err != nil {
		t.Fatalf("ToggleCompletion(A off): %v", err)
	}
	sum = summaryFor(t, s, on)
	if sum.Scheduled != 2 || sum.Completed != 1 {
		t.Fatalf("after A off: summary = %+v, want {Scheduled:2 Completed:1}", sum)
	}
}

// TestToggleCompletionCountsOnlyScheduledHabits proves the recount is scoped
// to scheduledHabitIDs: a completion row left behind by a habit that is no
// longer in the scheduled set (e.g. archived) must not inflate the count.
func TestToggleCompletionCountsOnlyScheduledHabits(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	scheduledHabit, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Meditate",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(scheduled): %v", err)
	}
	unscheduledHabit, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "Archived habit",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(unscheduled): %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

	// Complete the habit that will NOT be in the scheduled set for this date.
	if err := s.SetCompletion(ctx, unscheduledHabit, on, true, at); err != nil {
		t.Fatalf("SetCompletion(unscheduled): %v", err)
	}

	// Only scheduledHabit is passed as scheduled for this date.
	if _, err := s.ToggleCompletion(ctx, scheduledHabit, on, at, []int64{scheduledHabit}); err != nil {
		t.Fatalf("ToggleCompletion: %v", err)
	}
	sum := summaryFor(t, s, on)
	if sum.Scheduled != 1 || sum.Completed != 1 {
		t.Fatalf("summary = %+v, want {Scheduled:1 Completed:1} (unscheduled habit's completion must not count)", sum)
	}
}

// TestToggleCompletionEmptyScheduledHabitIDs proves an empty scheduled set is
// valid and produces {Scheduled:0 Completed:0} without emitting invalid SQL
// for an empty IN (...) clause.
func TestToggleCompletionEmptyScheduledHabitIDs(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitID, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "One-off",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)

	done, err := s.ToggleCompletion(ctx, habitID, on, at, nil)
	if err != nil {
		t.Fatalf("ToggleCompletion with nil scheduledHabitIDs: %v", err)
	}
	if !done {
		t.Fatalf("ToggleCompletion: done = %v, want true", done)
	}
	sum := summaryFor(t, s, on)
	if sum.Scheduled != 0 || sum.Completed != 0 {
		t.Fatalf("summary = %+v, want {Scheduled:0 Completed:0} for an empty scheduled set", sum)
	}
}

func TestRefreshDailySummaryRecountsFromCompletions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	habitA, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "A",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(A): %v", err)
	}
	habitB, err := s.CreateHabit(ctx, domain.Habit{
		Title:     "B",
		Schedule:  domain.DailySchedule(),
		CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit(B): %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	at := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	if err := s.SetCompletion(ctx, habitA, on, true, at); err != nil {
		t.Fatalf("SetCompletion(A): %v", err)
	}

	// A stale, manually-planted summary must be fully overwritten, not merged.
	if err := s.PutDailySummary(ctx, domain.DailySummary{Date: on, Scheduled: 99, Completed: 99}); err != nil {
		t.Fatalf("PutDailySummary (seed stale): %v", err)
	}

	if err := s.RefreshDailySummary(ctx, on, []int64{habitA, habitB}); err != nil {
		t.Fatalf("RefreshDailySummary: %v", err)
	}
	sum := summaryFor(t, s, on)
	if sum.Scheduled != 2 || sum.Completed != 1 {
		t.Fatalf("summary = %+v, want {Scheduled:2 Completed:1}", sum)
	}

	if err := s.RefreshDailySummary(ctx, on, nil); err != nil {
		t.Fatalf("RefreshDailySummary (empty): %v", err)
	}
	sum = summaryFor(t, s, on)
	if sum.Scheduled != 0 || sum.Completed != 0 {
		t.Fatalf("summary = %+v, want {Scheduled:0 Completed:0} after refreshing with an empty scheduled set", sum)
	}
}
