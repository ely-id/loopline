package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"routine-tui/internal/domain"
)

func mustOpenMemory(t *testing.T) *SQLite {
	t.Helper()
	db, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- migrations -------------------------------------------------------------

func TestSpecOpenMemoryAppliesMigrationsExactlyOnce(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	tables := []string{"routines", "habits", "completions", "reminders", "daily_summaries", "schema_migrations"}
	for _, tbl := range tables {
		var name string
		err := db.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, tbl).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing after migrate: %v", tbl, err)
		}
	}

	var version int
	if err := db.db.QueryRowContext(ctx, `SELECT version FROM schema_migrations`).Scan(&version); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	if version != 1 {
		t.Fatalf("schema_migrations version = %d, want 1", version)
	}
	var n int
	if err := db.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if n != 1 {
		t.Fatalf("schema_migrations has %d rows, want exactly 1", n)
	}
}

func TestSpecMigrationsAreIdempotentAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routine.db")

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	ctx := context.Background()
	if _, err := db1.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")}); err != nil {
		t.Fatalf("CreateRoutine before reopen: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("close db1: %v", err)
	}

	// Re-running migrations against the same file must not error (no
	// "table already exists") and must not duplicate the migration record.
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open (same path) must succeed: %v", err)
	}
	defer db2.Close()

	var n int
	if err := db2.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count schema_migrations after reopen: %v", err)
	}
	if n != 1 {
		t.Fatalf("schema_migrations has %d rows after reopen, want exactly 1 (migration must not reapply)", n)
	}

	routines, err := db2.Routines(ctx)
	if err != nil {
		t.Fatalf("Routines after reopen: %v", err)
	}
	if len(routines) != 1 || routines[0].Title != "Morning" {
		t.Fatalf("Routines after reopen = %+v, want the one created before close", routines)
	}
}

// --- round trips --------------------------------------------------------------

func TestSpecRoundTripRoutine(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	created := domain.MustParseDate("2026-08-01")
	id, err := db.CreateRoutine(ctx, domain.Routine{Title: "Morning", Position: 2, CreatedOn: created})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	routines, err := db.Routines(ctx)
	if err != nil {
		t.Fatalf("Routines: %v", err)
	}
	if len(routines) != 1 {
		t.Fatalf("Routines() = %+v, want exactly one", routines)
	}
	got := routines[0]
	if got.ID != id || got.Title != "Morning" || got.Position != 2 || got.CreatedOn != created || got.ArchivedOn != nil {
		t.Fatalf("round-tripped routine = %+v, want ID=%d Title=Morning Position=2 CreatedOn=%s ArchivedOn=nil", got, id, created)
	}

	if err := db.UpdateRoutine(ctx, domain.Routine{ID: id, Title: "Morning routine", Position: 5}); err != nil {
		t.Fatalf("UpdateRoutine: %v", err)
	}
	routines, _ = db.Routines(ctx)
	got = routines[0]
	if got.Title != "Morning routine" || got.Position != 5 {
		t.Fatalf("after UpdateRoutine: got %+v, want Title='Morning routine' Position=5", got)
	}
	if got.CreatedOn != created {
		t.Fatalf("UpdateRoutine must not touch CreatedOn: got %s, want %s", got.CreatedOn, created)
	}
}

func TestSpecRoundTripHabitWithScheduleAndArchive(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	routineID, err := db.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}

	archived := domain.MustParseDate("2026-09-01")
	sched := domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Monday, time.Wednesday, time.Friday)}
	h := domain.Habit{
		Title:      "Drink water",
		RoutineID:  &routineID,
		Schedule:   sched,
		Position:   3,
		CreatedOn:  domain.MustParseDate("2026-08-02"),
		ArchivedOn: &archived,
		Notes:      "8 glasses",
	}
	id, err := db.CreateHabit(ctx, h)
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	habits, err := db.Habits(ctx)
	if err != nil {
		t.Fatalf("Habits: %v", err)
	}
	if len(habits) != 1 {
		t.Fatalf("Habits() = %+v, want exactly one", habits)
	}
	got := habits[0]
	if got.ID != id {
		t.Fatalf("ID = %d, want %d", got.ID, id)
	}
	if got.Title != h.Title {
		t.Fatalf("Title = %q, want %q", got.Title, h.Title)
	}
	if got.RoutineID == nil || *got.RoutineID != routineID {
		t.Fatalf("RoutineID = %v, want %d", got.RoutineID, routineID)
	}
	if got.Schedule != sched {
		t.Fatalf("Schedule = %+v, want %+v", got.Schedule, sched)
	}
	if got.Position != h.Position {
		t.Fatalf("Position = %d, want %d", got.Position, h.Position)
	}
	if got.CreatedOn != h.CreatedOn {
		t.Fatalf("CreatedOn = %s, want %s", got.CreatedOn, h.CreatedOn)
	}
	if got.ArchivedOn == nil || *got.ArchivedOn != archived {
		t.Fatalf("ArchivedOn = %v, want %s", got.ArchivedOn, archived)
	}
	if got.Notes != h.Notes {
		t.Fatalf("Notes = %q, want %q", got.Notes, h.Notes)
	}

	// UpdateHabit must not touch CreatedOn or ArchivedOn.
	newSched := domain.DailySchedule()
	if err := db.UpdateHabit(ctx, domain.Habit{
		ID: id, Title: "Drink more water", RoutineID: nil, Schedule: newSched, Position: 7, Notes: "10 glasses",
	}); err != nil {
		t.Fatalf("UpdateHabit: %v", err)
	}
	habits, _ = db.Habits(ctx)
	got = habits[0]
	if got.Title != "Drink more water" || got.Position != 7 || got.Notes != "10 glasses" {
		t.Fatalf("after UpdateHabit: got %+v", got)
	}
	if got.RoutineID != nil {
		t.Fatalf("after UpdateHabit: RoutineID = %v, want nil (habit detached)", got.RoutineID)
	}
	if got.Schedule != newSched {
		t.Fatalf("after UpdateHabit: Schedule = %+v, want %+v", got.Schedule, newSched)
	}
	if got.CreatedOn != h.CreatedOn {
		t.Fatalf("UpdateHabit must not change CreatedOn: got %s, want %s", got.CreatedOn, h.CreatedOn)
	}
	if got.ArchivedOn == nil || *got.ArchivedOn != archived {
		t.Fatalf("UpdateHabit must not change ArchivedOn: got %v, want %s", got.ArchivedOn, archived)
	}
}

func TestSpecArchiveHabitSetsArchivedOnOnly(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	id, err := db.CreateHabit(ctx, domain.Habit{
		Title: "Stretch", Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	on := domain.MustParseDate("2026-08-10")
	if err := db.ArchiveHabit(ctx, id, on); err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}

	habits, err := db.Habits(ctx)
	if err != nil || len(habits) != 1 {
		t.Fatalf("Habits() after archive = %+v, %v, want the habit to still exist", habits, err)
	}
	if habits[0].ArchivedOn == nil || *habits[0].ArchivedOn != on {
		t.Fatalf("ArchivedOn = %v, want %s (ArchiveHabit must never delete the row)", habits[0].ArchivedOn, on)
	}
	if habits[0].Title != "Stretch" {
		t.Fatalf("Title = %q, want unchanged 'Stretch'", habits[0].Title)
	}
}

func TestSpecRoundTripReminder(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	r := domain.Reminder{Title: "Exam", Date: domain.MustParseDate("2026-09-01"), Notes: "bring calculator"}
	id, err := db.CreateReminder(ctx, r)
	if err != nil {
		t.Fatalf("CreateReminder: %v", err)
	}

	reminders, err := db.Reminders(ctx)
	if err != nil || len(reminders) != 1 {
		t.Fatalf("Reminders() = %+v, %v, want exactly one", reminders, err)
	}
	got := reminders[0]
	if got.ID != id || got.Title != r.Title || got.Date != r.Date || got.Notes != r.Notes || got.Completed {
		t.Fatalf("round-tripped reminder = %+v, want %+v (ID=%d, Completed=false)", got, r, id)
	}

	got.Completed = true
	got.Notes = "bring calculator and ID"
	if err := db.UpdateReminder(ctx, got); err != nil {
		t.Fatalf("UpdateReminder: %v", err)
	}
	reminders, _ = db.Reminders(ctx)
	if !reminders[0].Completed || reminders[0].Notes != "bring calculator and ID" {
		t.Fatalf("after UpdateReminder: got %+v", reminders[0])
	}

	if err := db.DeleteReminder(ctx, id); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	reminders, err = db.Reminders(ctx)
	if err != nil || len(reminders) != 0 {
		t.Fatalf("Reminders() after delete = %+v, %v, want empty", reminders, err)
	}
}

func TestSpecCompletionToggleIsIdempotent(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	habitID, err := db.CreateHabit(ctx, domain.Habit{
		Title: "Read", Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	on := domain.MustParseDate("2026-08-10")

	// Clearing a completion that does not exist must be a harmless no-op.
	if err := db.SetCompletion(ctx, habitID, on, false, time.Now()); err != nil {
		t.Fatalf("SetCompletion(false) on empty state: %v", err)
	}
	comps, err := db.CompletionsBetween(ctx, on, on)
	if err != nil || len(comps) != 0 {
		t.Fatalf("CompletionsBetween after no-op clear = %+v, %v, want empty", comps, err)
	}

	at1 := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	if err := db.SetCompletion(ctx, habitID, on, true, at1); err != nil {
		t.Fatalf("SetCompletion(true) #1: %v", err)
	}
	at2 := time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC)
	if err := db.SetCompletion(ctx, habitID, on, true, at2); err != nil {
		t.Fatalf("SetCompletion(true) #2 (marking complete again): %v", err)
	}

	comps, err = db.CompletionsBetween(ctx, on, on)
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(comps) != 1 {
		t.Fatalf("CompletionsBetween = %+v, want exactly one row (habit_id, date) is a primary key)", comps)
	}
	if !comps[0].CompletedAt.Equal(at2) {
		t.Fatalf("CompletedAt = %s, want the latest timestamp %s", comps[0].CompletedAt, at2)
	}

	if err := db.SetCompletion(ctx, habitID, on, false, time.Now()); err != nil {
		t.Fatalf("SetCompletion(false): %v", err)
	}
	comps, err = db.CompletionsBetween(ctx, on, on)
	if err != nil || len(comps) != 0 {
		t.Fatalf("CompletionsBetween after clear = %+v, %v, want empty", comps, err)
	}

	// Clearing again (already cleared) must still be a harmless no-op.
	if err := db.SetCompletion(ctx, habitID, on, false, time.Now()); err != nil {
		t.Fatalf("SetCompletion(false) again: %v", err)
	}
}

func TestSpecCompletionsBetweenIsInclusiveAndOrdered(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()
	h1, _ := db.CreateHabit(ctx, domain.Habit{Title: "A", Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01")})
	h2, _ := db.CreateHabit(ctx, domain.Habit{Title: "B", Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01")})

	dates := []string{"2026-08-08", "2026-08-09", "2026-08-10", "2026-08-11"}
	for _, ds := range dates {
		if err := db.SetCompletion(ctx, h1, domain.MustParseDate(ds), true, time.Now()); err != nil {
			t.Fatalf("SetCompletion h1 %s: %v", ds, err)
		}
	}
	if err := db.SetCompletion(ctx, h2, domain.MustParseDate("2026-08-09"), true, time.Now()); err != nil {
		t.Fatalf("SetCompletion h2: %v", err)
	}

	got, err := db.CompletionsBetween(ctx, domain.MustParseDate("2026-08-09"), domain.MustParseDate("2026-08-10"))
	if err != nil {
		t.Fatalf("CompletionsBetween: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("CompletionsBetween(08-09, 08-10) = %+v, want 3 rows (08-08 and 08-11 excluded)", got)
	}
	for _, c := range got {
		if c.Date.Before(domain.MustParseDate("2026-08-09")) || c.Date.After(domain.MustParseDate("2026-08-10")) {
			t.Fatalf("completion %+v falls outside the requested inclusive range", c)
		}
	}
}

func TestSpecRoundTripDailySummary(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()
	d := domain.MustParseDate("2026-08-10")

	if err := db.PutDailySummary(ctx, domain.DailySummary{Date: d, Scheduled: 3, Completed: 1}); err != nil {
		t.Fatalf("PutDailySummary: %v", err)
	}
	got, err := db.DailySummaries(ctx, d, d)
	if err != nil || len(got) != 1 {
		t.Fatalf("DailySummaries = %+v, %v, want exactly one", got, err)
	}
	if got[0].Scheduled != 3 || got[0].Completed != 1 {
		t.Fatalf("stored summary = %+v, want Scheduled=3 Completed=1", got[0])
	}

	// Upsert: writing the same date again must replace, not duplicate.
	if err := db.PutDailySummary(ctx, domain.DailySummary{Date: d, Scheduled: 3, Completed: 3}); err != nil {
		t.Fatalf("PutDailySummary (update): %v", err)
	}
	got, err = db.DailySummaries(ctx, d, d)
	if err != nil || len(got) != 1 {
		t.Fatalf("DailySummaries after upsert = %+v, %v, want exactly one row", got, err)
	}
	if got[0].Completed != 3 {
		t.Fatalf("Completed after upsert = %d, want 3 (last write wins)", got[0].Completed)
	}
}

func TestSpecFirstActivityDate(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	if _, ok, err := db.FirstActivityDate(ctx); err != nil || ok {
		t.Fatalf("FirstActivityDate on empty db = ok=%v err=%v, want ok=false", ok, err)
	}

	habitCreated := domain.MustParseDate("2026-08-05")
	habitID, err := db.CreateHabit(ctx, domain.Habit{Title: "A", Schedule: domain.DailySchedule(), CreatedOn: habitCreated})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	d, ok, err := db.FirstActivityDate(ctx)
	if err != nil || !ok || d != habitCreated {
		t.Fatalf("FirstActivityDate = %s, ok=%v, err=%v, want %s, true, nil", d, ok, err, habitCreated)
	}

	// A completion earlier than the habit's created_on (a data anomaly, but
	// the query must still take the true minimum across both tables).
	earlierCompletion := domain.MustParseDate("2026-07-20")
	if err := db.SetCompletion(ctx, habitID, earlierCompletion, true, time.Now()); err != nil {
		t.Fatalf("SetCompletion: %v", err)
	}
	d, ok, err = db.FirstActivityDate(ctx)
	if err != nil || !ok || d != earlierCompletion {
		t.Fatalf("FirstActivityDate = %s, ok=%v, err=%v, want %s (earliest of habit/completion)", d, ok, err, earlierCompletion)
	}
}

// --- routine deletion ---------------------------------------------------------

func TestSpecDeleteRoutineDetachesHabitsWithoutDestroyingThem(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	routineID, err := db.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}
	habitID, err := db.CreateHabit(ctx, domain.Habit{
		Title: "Drink water", RoutineID: &routineID, Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	if err := db.DeleteRoutine(ctx, routineID); err != nil {
		t.Fatalf("DeleteRoutine: %v", err)
	}

	routines, err := db.Routines(ctx)
	if err != nil || len(routines) != 0 {
		t.Fatalf("Routines() after delete = %+v, %v, want empty", routines, err)
	}
	habits, err := db.Habits(ctx)
	if err != nil || len(habits) != 1 {
		t.Fatalf("Habits() after routine delete = %+v, %v, want the habit to survive", habits, err)
	}
	if habits[0].ID != habitID {
		t.Fatalf("surviving habit ID = %d, want %d", habits[0].ID, habitID)
	}
	if habits[0].RoutineID != nil {
		t.Fatalf("RoutineID = %v, want nil (habit must be detached, not destroyed)", habits[0].RoutineID)
	}
}

// --- foreign keys ---------------------------------------------------------

func TestSpecForeignKeysAreEnforced(t *testing.T) {
	db := mustOpenMemory(t)
	ctx := context.Background()

	var enabled int
	if err := db.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&enabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if enabled != 1 {
		t.Fatal("PRAGMA foreign_keys must be ON")
	}

	bogusRoutine := int64(99999)
	if _, err := db.CreateHabit(ctx, domain.Habit{
		Title: "Orphan", RoutineID: &bogusRoutine, Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01"),
	}); err == nil {
		t.Fatal("CreateHabit with a nonexistent routine_id should fail the foreign key constraint")
	}

	bogusHabit := int64(99999)
	if err := db.SetCompletion(ctx, bogusHabit, domain.MustParseDate("2026-08-01"), true, time.Now()); err == nil {
		t.Fatal("SetCompletion for a nonexistent habit_id should fail the foreign key constraint")
	}
}

// --- on-disk persistence ---------------------------------------------------

func TestSpecDataSurvivesCloseAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routine.db")
	ctx := context.Background()

	db1, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	routineID, err := db1.CreateRoutine(ctx, domain.Routine{Title: "Morning", CreatedOn: domain.MustParseDate("2026-08-01")})
	if err != nil {
		t.Fatalf("CreateRoutine: %v", err)
	}
	habitID, err := db1.CreateHabit(ctx, domain.Habit{
		Title: "Drink water", RoutineID: &routineID, Schedule: domain.DailySchedule(), CreatedOn: domain.MustParseDate("2026-08-01"),
	})
	if err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	if err := db1.SetCompletion(ctx, habitID, domain.MustParseDate("2026-08-10"), true, time.Now()); err != nil {
		t.Fatalf("SetCompletion: %v", err)
	}
	if _, err := db1.CreateReminder(ctx, domain.Reminder{Title: "Exam", Date: domain.MustParseDate("2026-09-01")}); err != nil {
		t.Fatalf("CreateReminder: %v", err)
	}
	if err := db1.PutDailySummary(ctx, domain.DailySummary{Date: domain.MustParseDate("2026-08-10"), Scheduled: 1, Completed: 1}); err != nil {
		t.Fatalf("PutDailySummary: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	routines, err := db2.Routines(ctx)
	if err != nil || len(routines) != 1 || routines[0].Title != "Morning" {
		t.Fatalf("Routines after reopen = %+v, %v", routines, err)
	}
	habits, err := db2.Habits(ctx)
	if err != nil || len(habits) != 1 || habits[0].Title != "Drink water" {
		t.Fatalf("Habits after reopen = %+v, %v", habits, err)
	}
	comps, err := db2.CompletionsBetween(ctx, domain.MustParseDate("2026-08-10"), domain.MustParseDate("2026-08-10"))
	if err != nil || len(comps) != 1 {
		t.Fatalf("CompletionsBetween after reopen = %+v, %v", comps, err)
	}
	reminders, err := db2.Reminders(ctx)
	if err != nil || len(reminders) != 1 || reminders[0].Title != "Exam" {
		t.Fatalf("Reminders after reopen = %+v, %v", reminders, err)
	}
	summaries, err := db2.DailySummaries(ctx, domain.MustParseDate("2026-08-10"), domain.MustParseDate("2026-08-10"))
	if err != nil || len(summaries) != 1 || summaries[0].Completed != 1 {
		t.Fatalf("DailySummaries after reopen = %+v, %v", summaries, err)
	}
}

// --- XDG path resolution ----------------------------------------------------

func TestSpecDefaultPathUsesXDGDataHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(tmp, "routine-tui", "routine.db")
	if path != want {
		t.Fatalf("DefaultPath() = %s, want %s", path, want)
	}

	info, err := os.Stat(filepath.Join(tmp, "routine-tui"))
	if err != nil {
		t.Fatalf("stat data dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("routine-tui path must be a directory")
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Fatalf("data dir permissions = %o, want 0700", perm)
	}
}

func TestSpecDefaultPathFallsBackToHomeLocalShare(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", tmp)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(tmp, ".local", "share", "routine-tui", "routine.db")
	if path != want {
		t.Fatalf("DefaultPath() = %s, want %s", path, want)
	}
}
