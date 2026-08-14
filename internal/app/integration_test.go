package app_test

// This file binds app.Service to the REAL internal/store SQLite
// implementation instead of a fake, per Codex review feedback: the
// failure-free fakeStore in service_test.go cannot catch transaction
// behaviour (real BEGIN/COMMIT/ROLLBACK, real foreign keys, the atomic
// ToggleCompletion read-modify-write) that only the actual driver exercises.
// It lives in package app_test (not package app) since it needs no
// unexported access and mirrors how cmd/routine wires app+store together.

import (
	"context"
	"testing"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/store"
)

func TestServiceIntegrationWithRealSQLiteStore(t *testing.T) {
	ctx := context.Background()
	db, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer db.Close()

	today := domain.MustParseDate("2026-08-10")
	svc := app.New(db, domain.NewFixedClock(today))

	if err := svc.AddRoutine(ctx, app.NewRoutine{Title: "Morning"}); err != nil {
		t.Fatalf("AddRoutine: %v", err)
	}
	routines, err := db.Routines(ctx)
	if err != nil || len(routines) != 1 {
		t.Fatalf("Routines = %+v, %v, want exactly one", routines, err)
	}
	routineID := routines[0].ID

	if err := svc.AddHabit(ctx, app.NewHabit{Title: "Stretch", RoutineID: &routineID}); err != nil {
		t.Fatalf("AddHabit (grouped): %v", err)
	}
	if err := svc.AddHabit(ctx, app.NewHabit{Title: "Wake up"}); err != nil {
		t.Fatalf("AddHabit (ungrouped): %v", err)
	}

	habits, err := db.Habits(ctx)
	if err != nil || len(habits) != 2 {
		t.Fatalf("Habits = %+v, %v, want exactly two", habits, err)
	}
	var stretchID int64
	for _, h := range habits {
		if h.Title == "Stretch" {
			stretchID = h.ID
		}
	}
	if stretchID == 0 {
		t.Fatal("could not find the Stretch habit after AddHabit")
	}

	if err := svc.ToggleHabit(ctx, stretchID); err != nil {
		t.Fatalf("ToggleHabit: %v", err)
	}

	snap, err := svc.Snapshot(ctx, app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Today.Summary.Scheduled != 2 || snap.Today.Summary.Completed != 1 {
		t.Fatalf("Today.Summary = %+v, want Scheduled=2 Completed=1", snap.Today.Summary)
	}
	if len(snap.Today.Groups) != 2 {
		t.Fatalf("Today.Groups = %+v, want 2 groups (ungrouped + Morning)", snap.Today.Groups)
	}

	// In-place edit use cases must preserve identity, relationships and the
	// completion row through the real SQLite repositories.
	if err := svc.EditRoutine(ctx, routineID, app.NewRoutine{Title: "First light"}); err != nil {
		t.Fatalf("EditRoutine: %v", err)
	}
	if err := svc.EditHabit(ctx, stretchID, app.NewHabit{
		Title: "Mobility", RoutineID: &routineID, Schedule: domain.DailySchedule(), Notes: "five minutes",
	}); err != nil {
		t.Fatalf("EditHabit: %v", err)
	}
	completions, err := db.CompletionsBetween(ctx, today, today)
	if err != nil || len(completions) != 1 || completions[0].HabitID != stretchID {
		t.Fatalf("completion history after EditHabit = %+v, %v", completions, err)
	}
	editedHabits, err := db.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits after EditHabit: %v", err)
	}
	for _, h := range editedHabits {
		if h.ID == stretchID && (h.Title != "Mobility" || h.Notes != "five minutes" || h.RoutineID == nil || *h.RoutineID != routineID) {
			t.Fatalf("edited habit did not retain identity/membership: %+v", h)
		}
	}

	if err := svc.AddReminder(ctx, app.NewReminder{Title: "Exam", Date: today, Notes: "room 1"}); err != nil {
		t.Fatalf("AddReminder: %v", err)
	}
	reminders, err := db.Reminders(ctx)
	if err != nil || len(reminders) != 1 {
		t.Fatalf("Reminders after AddReminder = %+v, %v", reminders, err)
	}
	reminderID := reminders[0].ID
	if err := svc.ToggleReminder(ctx, reminderID); err != nil {
		t.Fatalf("ToggleReminder: %v", err)
	}
	if err := svc.EditReminder(ctx, reminderID, app.NewReminder{Title: "Algorithms exam", Date: today.AddDays(1), Notes: "room 2"}); err != nil {
		t.Fatalf("EditReminder: %v", err)
	}
	reminders, err = db.Reminders(ctx)
	if err != nil || len(reminders) != 1 || reminders[0].ID != reminderID || !reminders[0].Completed || reminders[0].Title != "Algorithms exam" {
		t.Fatalf("reminder edit did not preserve identity/status: %+v, %v", reminders, err)
	}
	if err := svc.DeleteReminder(ctx, reminderID); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	reminders, err = db.Reminders(ctx)
	if err != nil || len(reminders) != 0 {
		t.Fatalf("Reminders after DeleteReminder = %+v, %v", reminders, err)
	}

	// Exercise the real store's atomic ToggleCompletion transaction on the
	// same habit again: it must flip cleanly back off, not double-fire.
	if err := svc.ToggleHabit(ctx, stretchID); err != nil {
		t.Fatalf("ToggleHabit (off): %v", err)
	}
	snap2, err := svc.Snapshot(ctx, app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot 2: %v", err)
	}
	if snap2.Today.Summary.Completed != 0 {
		t.Fatalf("Completed after second toggle = %d, want 0", snap2.Today.Summary.Completed)
	}

	// DeleteRoutine must detach, not destroy, going through the real store's
	// transaction (UPDATE habits SET routine_id=NULL + DELETE routine, backed
	// by ON DELETE SET NULL) rather than the fake's in-memory approximation.
	if err := svc.DeleteRoutine(ctx, routineID); err != nil {
		t.Fatalf("DeleteRoutine: %v", err)
	}
	habitsAfter, err := db.Habits(ctx)
	if err != nil || len(habitsAfter) != 2 {
		t.Fatalf("Habits after DeleteRoutine = %+v, %v, want both habits to survive", habitsAfter, err)
	}
	for _, h := range habitsAfter {
		if h.ID == stretchID && h.RoutineID != nil {
			t.Fatalf("edited habit still has RoutineID %v after its routine was deleted", h.RoutineID)
		}
	}

	// daily_summaries, written through the real store, must stay consistent
	// with what CompletionsBetween reports.
	stored, err := db.DailySummaries(ctx, today, today)
	if err != nil || len(stored) != 1 {
		t.Fatalf("DailySummaries = %+v, %v, want exactly one row", stored, err)
	}
	if stored[0].Scheduled != 2 || stored[0].Completed != 0 {
		t.Fatalf("persisted summary = %+v, want Scheduled=2 Completed=0", stored[0])
	}

	// ToggleHabit rejecting an unscheduled habit must also hold through the
	// real store, not just the fake.
	if err := db.ArchiveHabit(ctx, stretchID, today); err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}
	if err := svc.ToggleHabit(ctx, stretchID); err == nil {
		t.Fatal("ToggleHabit on a habit archived as of today should be rejected through the real store too")
	}
}
