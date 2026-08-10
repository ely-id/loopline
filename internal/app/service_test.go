package app

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"routine-tui/internal/domain"
)

// errInjectedToggleFailure is returned by fakeStore.ToggleCompletion when
// toggleCompletionFailNext is set, for failure-injection tests. ToggleCompletion
// is one atomic transaction in the real store (ports.go), so a failure must
// never leave a partial write (a completion with no matching summary, or vice
// versa) behind — the fake models that by writing nothing at all on failure.
var errInjectedToggleFailure = errors.New("injected ToggleCompletion failure")

// fakeStore is an in-memory implementation of Store for driving Service in
// tests without touching internal/store or SQLite. It mirrors the documented
// behaviour of the real store closely enough to exercise Service logic:
// DeleteRoutine detaches habits instead of destroying them, ArchiveHabit sets
// archived_on rather than deleting the row, and SetCompletion is idempotent.
type fakeStore struct {
	habits        map[int64]domain.Habit
	nextHabitID   int64
	routines      map[int64]domain.Routine
	nextRoutineID int64
	completions   map[int64]map[domain.Date]domain.Completion // habitID -> date -> completion
	reminders     map[int64]domain.Reminder
	nextReminder  int64
	summaries     map[domain.Date]domain.DailySummary

	// toggleCompletionFailNext, when true, makes the next ToggleCompletion
	// call fail and then clears itself, for failure-injection tests.
	toggleCompletionFailNext bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		habits:      map[int64]domain.Habit{},
		routines:    map[int64]domain.Routine{},
		completions: map[int64]map[domain.Date]domain.Completion{},
		reminders:   map[int64]domain.Reminder{},
		summaries:   map[domain.Date]domain.DailySummary{},
	}
}

func (f *fakeStore) Habits(ctx context.Context) ([]domain.Habit, error) {
	out := make([]domain.Habit, 0, len(f.habits))
	for _, h := range f.habits {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) CreateHabit(ctx context.Context, h domain.Habit) (int64, error) {
	f.nextHabitID++
	h.ID = f.nextHabitID
	f.habits[h.ID] = h
	return h.ID, nil
}

func (f *fakeStore) UpdateHabit(ctx context.Context, h domain.Habit) error {
	f.habits[h.ID] = h
	return nil
}

func (f *fakeStore) ArchiveHabit(ctx context.Context, id int64, on domain.Date) error {
	h := f.habits[id]
	archived := on
	h.ArchivedOn = &archived
	f.habits[id] = h
	return nil
}

func (f *fakeStore) Routines(ctx context.Context) ([]domain.Routine, error) {
	out := make([]domain.Routine, 0, len(f.routines))
	for _, r := range f.routines {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) CreateRoutine(ctx context.Context, r domain.Routine) (int64, error) {
	f.nextRoutineID++
	r.ID = f.nextRoutineID
	f.routines[r.ID] = r
	return r.ID, nil
}

func (f *fakeStore) UpdateRoutine(ctx context.Context, r domain.Routine) error {
	f.routines[r.ID] = r
	return nil
}

// DeleteRoutine mirrors the real store's contract: hard-delete the routine
// row, detach (never destroy) its habits.
func (f *fakeStore) DeleteRoutine(ctx context.Context, id int64) error {
	delete(f.routines, id)
	for hid, h := range f.habits {
		if h.RoutineID != nil && *h.RoutineID == id {
			h.RoutineID = nil
			f.habits[hid] = h
		}
	}
	return nil
}

func (f *fakeStore) CompletionsBetween(ctx context.Context, from, to domain.Date) ([]domain.Completion, error) {
	var out []domain.Completion
	for _, byDate := range f.completions {
		for d, c := range byDate {
			if d.Before(from) || d.After(to) {
				continue
			}
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date.Before(out[j].Date)
		}
		return out[i].HabitID < out[j].HabitID
	})
	return out, nil
}

func (f *fakeStore) SetCompletion(ctx context.Context, habitID int64, on domain.Date, done bool, at time.Time) error {
	byDate, ok := f.completions[habitID]
	if !ok {
		byDate = map[domain.Date]domain.Completion{}
		f.completions[habitID] = byDate
	}
	if done {
		byDate[on] = domain.Completion{HabitID: habitID, Date: on, CompletedAt: at}
	} else {
		delete(byDate, on)
	}
	return nil
}

// ToggleCompletion flips the completion for a habit on a date and rewrites
// the derived daily summary for that date, mirroring the real store's atomic
// read-modify-write-then-recount contract (ports.go). On an injected
// failure, nothing is written at all — matching the real store's transaction
// rollback — rather than leaving a completion behind with no summary update.
func (f *fakeStore) ToggleCompletion(ctx context.Context, habitID int64, on domain.Date, at time.Time, scheduledHabitIDs []int64) (bool, error) {
	if f.toggleCompletionFailNext {
		f.toggleCompletionFailNext = false
		return false, errInjectedToggleFailure
	}

	byDate, ok := f.completions[habitID]
	if !ok {
		byDate = map[domain.Date]domain.Completion{}
		f.completions[habitID] = byDate
	}
	_, wasDone := byDate[on]
	done := !wasDone
	if done {
		byDate[on] = domain.Completion{HabitID: habitID, Date: on, CompletedAt: at}
	} else {
		delete(byDate, on)
	}
	f.recomputeSummary(on, scheduledHabitIDs)
	return done, nil
}

// RefreshDailySummary recomputes and stores the derived summary for a date,
// recounting from committed completions. Used after habit-set changes that
// alter the scheduled count without touching completions (ports.go).
func (f *fakeStore) RefreshDailySummary(ctx context.Context, on domain.Date, scheduledHabitIDs []int64) error {
	f.recomputeSummary(on, scheduledHabitIDs)
	return nil
}

// recomputeSummary rewrites f.summaries[on] from committed completions for
// exactly the given scheduled habit set, mirroring the real store's
// writeSummaryTx: the completed count is always recounted, never taken from
// a caller-supplied value, so the fake can't drift from completions any more
// than the real store can.
func (f *fakeStore) recomputeSummary(on domain.Date, scheduledHabitIDs []int64) {
	completed := 0
	for _, id := range scheduledHabitIDs {
		if byDate, ok := f.completions[id]; ok {
			if _, done := byDate[on]; done {
				completed++
			}
		}
	}
	f.summaries[on] = domain.DailySummary{Date: on, Scheduled: len(scheduledHabitIDs), Completed: completed}
}

func (f *fakeStore) Reminders(ctx context.Context) ([]domain.Reminder, error) {
	out := make([]domain.Reminder, 0, len(f.reminders))
	for _, r := range f.reminders {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}

func (f *fakeStore) CreateReminder(ctx context.Context, r domain.Reminder) (int64, error) {
	f.nextReminder++
	r.ID = f.nextReminder
	f.reminders[r.ID] = r
	return r.ID, nil
}

func (f *fakeStore) UpdateReminder(ctx context.Context, r domain.Reminder) error {
	f.reminders[r.ID] = r
	return nil
}

func (f *fakeStore) DeleteReminder(ctx context.Context, id int64) error {
	delete(f.reminders, id)
	return nil
}

func (f *fakeStore) PutDailySummary(ctx context.Context, s domain.DailySummary) error {
	f.summaries[s.Date] = s
	return nil
}

func (f *fakeStore) DailySummaries(ctx context.Context, from, to domain.Date) ([]domain.DailySummary, error) {
	var out []domain.DailySummary
	for d, s := range f.summaries {
		if d.Before(from) || d.After(to) {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}

func (f *fakeStore) FirstActivityDate(ctx context.Context) (domain.Date, bool, error) {
	var first domain.Date
	has := false
	consider := func(d domain.Date) {
		if !has || d.Before(first) {
			first = d
			has = true
		}
	}
	for _, h := range f.habits {
		consider(h.CreatedOn)
	}
	for _, byDate := range f.completions {
		for d := range byDate {
			consider(d)
		}
	}
	return first, has, nil
}

func (f *fakeStore) Close() error { return nil }

var _ Store = (*fakeStore)(nil)

// --- test helpers -----------------------------------------------------------

func findCalendarDay(cal CalendarMonth, d domain.Date) (CalendarDay, bool) {
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.Date.Equal(d) {
				return day, true
			}
		}
	}
	return CalendarDay{}, false
}

func findHeatmapCell(hm Heatmap, d domain.Date) (HeatmapCell, bool) {
	for _, col := range hm.Columns {
		for _, cell := range col {
			if cell.Date.Equal(d) {
				return cell, true
			}
		}
	}
	return HeatmapCell{}, false
}

func findGroup(groups []HabitGroup, title string) (HabitGroup, bool) {
	for _, g := range groups {
		if g.Title == title {
			return g, true
		}
	}
	return HabitGroup{}, false
}

// --- Snapshot: today grouping -----------------------------------------------

func TestServiceSnapshotGroupsUngroupedFirstThenRoutinesInPosition(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	clock := domain.NewFixedClock(domain.MustParseDate("2026-08-10"))
	svc := New(store, clock)

	if err := svc.AddRoutine(ctx, NewRoutine{Title: "Morning"}); err != nil {
		t.Fatalf("AddRoutine(Morning): %v", err)
	}
	if err := svc.AddRoutine(ctx, NewRoutine{Title: "Evening"}); err != nil {
		t.Fatalf("AddRoutine(Evening): %v", err)
	}
	routines, err := store.Routines(ctx)
	if err != nil || len(routines) != 2 {
		t.Fatalf("Routines() = %v, %v", routines, err)
	}
	var morningID, eveningID int64
	for _, r := range routines {
		switch r.Title {
		case "Morning":
			morningID = r.ID
		case "Evening":
			eveningID = r.ID
		}
	}

	if err := svc.AddHabit(ctx, NewHabit{Title: "Wake up before 8"}); err != nil {
		t.Fatalf("AddHabit(ungrouped): %v", err)
	}
	if err := svc.AddHabit(ctx, NewHabit{Title: "Drink water", RoutineID: &morningID}); err != nil {
		t.Fatalf("AddHabit(morning): %v", err)
	}
	if err := svc.AddHabit(ctx, NewHabit{Title: "Meditate", RoutineID: &eveningID}); err != nil {
		t.Fatalf("AddHabit(evening): %v", err)
	}

	snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if len(snap.Today.Groups) != 3 {
		t.Fatalf("len(Groups) = %d, want 3: %+v", len(snap.Today.Groups), snap.Today.Groups)
	}
	if snap.Today.Groups[0].Title != "" || snap.Today.Groups[0].RoutineID != nil {
		t.Fatalf("Groups[0] = %+v, want the ungrouped block first", snap.Today.Groups[0])
	}
	if len(snap.Today.Groups[0].Items) != 1 || snap.Today.Groups[0].Items[0].Habit.Title != "Wake up before 8" {
		t.Fatalf("ungrouped items = %+v, want [Wake up before 8]", snap.Today.Groups[0].Items)
	}
	if snap.Today.Groups[1].Title != "Morning" {
		t.Fatalf("Groups[1].Title = %q, want Morning (routine position order)", snap.Today.Groups[1].Title)
	}
	if snap.Today.Groups[2].Title != "Evening" {
		t.Fatalf("Groups[2].Title = %q, want Evening", snap.Today.Groups[2].Title)
	}

	if snap.Today.Summary.Scheduled != 3 {
		t.Fatalf("Summary.Scheduled = %d, want 3", snap.Today.Summary.Scheduled)
	}
	if snap.Today.Summary.Completed != 0 {
		t.Fatalf("Summary.Completed = %d, want 0", snap.Today.Summary.Completed)
	}
}

func TestServiceSnapshotOmitsEmptyRoutineGroups(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	clock := domain.NewFixedClock(domain.MustParseDate("2026-08-10"))
	svc := New(store, clock)

	if err := svc.AddRoutine(ctx, NewRoutine{Title: "Empty"}); err != nil {
		t.Fatalf("AddRoutine(Empty): %v", err)
	}
	if err := svc.AddRoutine(ctx, NewRoutine{Title: "Morning"}); err != nil {
		t.Fatalf("AddRoutine(Morning): %v", err)
	}
	routines, _ := store.Routines(ctx)
	var morningID int64
	for _, r := range routines {
		if r.Title == "Morning" {
			morningID = r.ID
		}
	}
	if err := svc.AddHabit(ctx, NewHabit{Title: "Stretch", RoutineID: &morningID}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}

	snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if _, ok := findGroup(snap.Today.Groups, "Empty"); ok {
		t.Fatalf("empty routine group must be omitted from TodayView, got groups: %+v", snap.Today.Groups)
	}
	if _, ok := findGroup(snap.Today.Groups, "Morning"); !ok {
		t.Fatalf("Morning group with a habit must be present, got groups: %+v", snap.Today.Groups)
	}
	if len(snap.Today.Groups) != 1 {
		t.Fatalf("len(Groups) = %d, want 1 (only Morning)", len(snap.Today.Groups))
	}
}

// --- ToggleHabit propagation -------------------------------------------------

func TestServiceToggleHabitUpdatesTodaySummaryCalendarAndHeatmap(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	clock := domain.NewFixedClock(today)
	svc := New(store, clock)

	if err := svc.AddHabit(ctx, NewHabit{Title: "Read"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	habitID := habits[0].ID

	before, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot (before): %v", err)
	}
	if before.Today.Summary.Completed != 0 {
		t.Fatalf("before toggle: Completed = %d, want 0", before.Today.Summary.Completed)
	}
	beforeDay, ok := findCalendarDay(before.Calendar, today)
	if !ok || beforeDay.Summary.Completed != 0 {
		t.Fatalf("before toggle: calendar day = %+v, want Completed 0", beforeDay)
	}
	beforeCell, ok := findHeatmapCell(before.Heatmap, today)
	if !ok || beforeCell.Level != 0 {
		t.Fatalf("before toggle: heatmap cell = %+v, want Level 0", beforeCell)
	}

	if err := svc.ToggleHabit(ctx, habitID); err != nil {
		t.Fatalf("ToggleHabit: %v", err)
	}

	after, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot (after): %v", err)
	}
	if after.Today.Summary.Completed != 1 || after.Today.Summary.Scheduled != 1 {
		t.Fatalf("after toggle: Summary = %+v, want Scheduled=1 Completed=1", after.Today.Summary)
	}
	afterDay, ok := findCalendarDay(after.Calendar, today)
	if !ok || afterDay.Summary.Completed != 1 || !afterDay.Summary.IsComplete() {
		t.Fatalf("after toggle: calendar day = %+v, want Completed=1 and IsComplete", afterDay)
	}
	afterCell, ok := findHeatmapCell(after.Heatmap, today)
	if !ok || afterCell.Level != 4 {
		t.Fatalf("after toggle: heatmap cell = %+v, want Level 4 (100%% complete)", afterCell)
	}

	// Toggling again must be idempotent-reversible: back to incomplete.
	if err := svc.ToggleHabit(ctx, habitID); err != nil {
		t.Fatalf("ToggleHabit (second time): %v", err)
	}
	final, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot (final): %v", err)
	}
	if final.Today.Summary.Completed != 0 {
		t.Fatalf("after second toggle: Completed = %d, want 0", final.Today.Summary.Completed)
	}
}

func TestServiceToggleHabitUnknownIDFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	svc := New(store, domain.NewFixedClock(domain.MustParseDate("2026-08-10")))

	if err := svc.ToggleHabit(ctx, 999); err == nil {
		t.Fatal("ToggleHabit(unknown id) should return an error")
	}
}

// --- Midnight rollover --------------------------------------------------------

func TestServiceMidnightRolloverPreservesYesterdayAndStartsFreshToday(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	day1 := domain.MustParseDate("2026-08-10")
	clock := domain.NewFixedClock(day1)
	svc := New(store, clock)

	if err := svc.AddHabit(ctx, NewHabit{Title: "Journal"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	habitID := habits[0].ID

	if err := svc.ToggleHabit(ctx, habitID); err != nil {
		t.Fatalf("ToggleHabit on day1: %v", err)
	}

	day1Snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot day1: %v", err)
	}
	if day1Snap.Today.Summary.Completed != 1 {
		t.Fatalf("day1 Completed = %d, want 1", day1Snap.Today.Summary.Completed)
	}

	clock.Advance(1)
	day2 := day1.AddDays(1)
	if svc.Today() != day2 {
		t.Fatalf("Service.Today() = %s after Advance(1), want %s", svc.Today(), day2)
	}

	day2Snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot day2: %v", err)
	}
	if day2Snap.Today.Date != day2 {
		t.Fatalf("Today.Date = %s, want %s", day2Snap.Today.Date, day2)
	}
	if day2Snap.Today.Summary.Completed != 0 {
		t.Fatalf("day2 Completed = %d, want 0 (fresh day, habit not yet toggled)", day2Snap.Today.Summary.Completed)
	}
	if day2Snap.Today.Summary.Scheduled != 1 {
		t.Fatalf("day2 Scheduled = %d, want 1 (daily habit still scheduled)", day2Snap.Today.Summary.Scheduled)
	}

	// Yesterday's completion must still be visible in the calendar/store.
	yesterdayCell, ok := findCalendarDay(day2Snap.Calendar, day1)
	if !ok {
		t.Fatalf("day1 (%s) missing from day2's calendar", day1)
	}
	if yesterdayCell.Summary.Completed != 1 {
		t.Fatalf("yesterday's calendar cell Completed = %d, want 1 (history must survive rollover)", yesterdayCell.Summary.Completed)
	}

	stored, err := store.DailySummaries(ctx, day1, day1)
	if err != nil || len(stored) != 1 || stored[0].Completed != 1 {
		t.Fatalf("store.DailySummaries(day1) = %+v, %v, want one row with Completed=1", stored, err)
	}
}

// --- daily_summaries consistency ---------------------------------------------

func TestServiceDailySummariesStayConsistentWithCompletions(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	clock := domain.NewFixedClock(today)
	svc := New(store, clock)

	if err := svc.AddHabit(ctx, NewHabit{Title: "A"}); err != nil {
		t.Fatalf("AddHabit(A): %v", err)
	}
	if err := svc.AddHabit(ctx, NewHabit{Title: "B"}); err != nil {
		t.Fatalf("AddHabit(B): %v", err)
	}
	habits, _ := store.Habits(ctx)

	assertConsistent := func(label string) {
		t.Helper()
		stored, err := store.DailySummaries(ctx, today, today)
		if err != nil {
			t.Fatalf("%s: DailySummaries: %v", label, err)
		}
		if len(stored) != 1 {
			t.Fatalf("%s: DailySummaries = %+v, want exactly one row for today", label, stored)
		}
		comps, err := store.CompletionsBetween(ctx, today, today)
		if err != nil {
			t.Fatalf("%s: CompletionsBetween: %v", label, err)
		}
		done := map[int64]bool{}
		for _, c := range comps {
			done[c.HabitID] = true
		}
		want := domain.SummarizeDay(habits, done, today)
		if stored[0] != want {
			t.Fatalf("%s: stored summary = %+v, want %+v (recomputed from habits+completions)", label, stored[0], want)
		}
	}

	if err := svc.ToggleHabit(ctx, habits[0].ID); err != nil {
		t.Fatalf("ToggleHabit(A): %v", err)
	}
	assertConsistent("after toggling A on")

	if err := svc.ToggleHabit(ctx, habits[1].ID); err != nil {
		t.Fatalf("ToggleHabit(B): %v", err)
	}
	assertConsistent("after toggling B on")

	if err := svc.ToggleHabit(ctx, habits[0].ID); err != nil {
		t.Fatalf("ToggleHabit(A off): %v", err)
	}
	assertConsistent("after toggling A back off")

	habits, _ = store.Habits(ctx) // refresh in case DeleteHabit changed archived_on
	if err := svc.DeleteHabit(ctx, habits[0].ID); err != nil {
		t.Fatalf("DeleteHabit(A): %v", err)
	}
	habits, _ = store.Habits(ctx)
	assertConsistent("after archiving A")
}

// --- Heatmap padding and range selection -------------------------------------

func TestServiceHeatmapPaddingCellsHaveZeroDate(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10") // a Monday
	clock := domain.NewFixedClock(today)
	svc := New(store, clock)

	if err := svc.AddHabit(ctx, NewHabit{Title: "Anything"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}

	snap, err := svc.Snapshot(ctx, SnapshotRequest{Range: RangeMonth})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	hm := snap.Heatmap
	if hm.Start.IsZero() || hm.End.IsZero() {
		t.Fatalf("Heatmap.Start/End must not be zero: %+v", hm)
	}

	wantDays := hm.End.DaysSince(hm.Start) + 1
	gotDays := 0
	for _, col := range hm.Columns {
		if len(col) != 7 {
			t.Fatalf("heatmap column has %d cells, want 7 (every column is a full week)", len(col))
		}
		for _, cell := range col {
			if cell.Date.IsZero() {
				if cell.Level != 0 {
					t.Fatalf("padding cell (zero Date) has Level %d, want 0: %+v", cell.Level, cell)
				}
				continue
			}
			gotDays++
			if cell.Date.Before(hm.Start) || cell.Date.After(hm.End) {
				t.Fatalf("non-zero cell Date %s falls outside [%s, %s]", cell.Date, hm.Start, hm.End)
			}
		}
	}
	if gotDays != wantDays {
		t.Fatalf("heatmap has %d dated cells, want %d (End.DaysSince(Start)+1)", gotDays, wantDays)
	}

	// The first column must be padded up to the weekday offset of Start,
	// computed independently via Date.StartOfWeek/Weekday rather than a
	// hardcoded calendar assumption.
	offset := int(hm.Start.Weekday()+6) % 7 // Monday == 0, matches Date.StartOfWeek
	firstCol := hm.Columns[0]
	for i := 0; i < offset; i++ {
		if !firstCol[i].Date.IsZero() {
			t.Fatalf("firstCol[%d] = %+v, want zero-date padding before Start %s", i, firstCol[i], hm.Start)
		}
	}
	if firstCol[offset].Date != hm.Start {
		t.Fatalf("firstCol[%d] = %s, want Start %s", offset, firstCol[offset].Date, hm.Start)
	}
}

func TestServiceHeatmapRangeAllStartsAtFirstActivityDate(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	firstActivity := domain.MustParseDate("2026-05-15")

	// Insert a habit created well before "today" directly through the store,
	// bypassing Service.AddHabit (which always stamps CreatedOn = today).
	if _, err := store.CreateHabit(ctx, domain.Habit{
		Title:     "Old habit",
		Schedule:  domain.DailySchedule(),
		CreatedOn: firstActivity,
	}); err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}

	svc := New(store, domain.NewFixedClock(today))
	snap, err := svc.Snapshot(ctx, SnapshotRequest{Range: RangeAll})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.Heatmap.Start != firstActivity {
		t.Fatalf("Heatmap.Start = %s, want %s (FirstActivityDate)", snap.Heatmap.Start, firstActivity)
	}
	if snap.Heatmap.End != today {
		t.Fatalf("Heatmap.End = %s, want %s (today; the heatmap never shows the future)", snap.Heatmap.End, today)
	}
}

// --- Calendar grid invariants -------------------------------------------------

func TestServiceCalendarWeeksAreFullSevenDayRows(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	svc := New(store, domain.NewFixedClock(today))

	snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	cal := snap.Calendar
	if len(cal.Weeks) == 0 {
		t.Fatal("Calendar.Weeks is empty")
	}
	for i, week := range cal.Weeks {
		if len(week) != 7 {
			t.Fatalf("week %d has %d days, want 7", i, len(week))
		}
		if week[0].Date.Weekday() != time.Monday {
			t.Fatalf("week %d starts on %s, want Monday", i, week[0].Date.Weekday())
		}
		if week[6].Date.Weekday() != time.Sunday {
			t.Fatalf("week %d ends on %s, want Sunday", i, week[6].Date.Weekday())
		}
	}
	firstWeek := cal.Weeks[0]
	lastWeek := cal.Weeks[len(cal.Weeks)-1]
	monthStart := today.StartOfMonth()
	monthEnd := today.EndOfMonth()
	for _, day := range firstWeek {
		wantInMonth := !day.Date.Before(monthStart) && !day.Date.After(monthEnd)
		if day.InMonth != wantInMonth {
			t.Fatalf("day %s InMonth = %v, want %v", day.Date, day.InMonth, wantInMonth)
		}
	}
	for _, day := range lastWeek {
		wantInMonth := !day.Date.Before(monthStart) && !day.Date.After(monthEnd)
		if day.InMonth != wantInMonth {
			t.Fatalf("day %s InMonth = %v, want %v", day.Date, day.InMonth, wantInMonth)
		}
	}
}

// --- DeleteHabit archives, never destroys -------------------------------------

func TestServiceDeleteHabitArchivesRatherThanDestroys(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	createdOn := domain.MustParseDate("2026-08-01")
	deleteOn := domain.MustParseDate("2026-08-10")
	clock := domain.NewFixedClock(createdOn)
	svc := New(store, clock)

	if err := svc.AddHabit(ctx, NewHabit{Title: "Old habit"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	habitID := habits[0].ID

	clock.Date = deleteOn // jump straight to the delete date
	clock.At = clock.At.AddDate(0, 0, deleteOn.DaysSince(createdOn))

	if err := svc.DeleteHabit(ctx, habitID); err != nil {
		t.Fatalf("DeleteHabit: %v", err)
	}

	stored, err := store.Habits(ctx)
	if err != nil || len(stored) != 1 {
		t.Fatalf("Habits() after delete = %+v, %v, want the habit to still exist", stored, err)
	}
	if stored[0].ArchivedOn == nil || *stored[0].ArchivedOn != deleteOn {
		t.Fatalf("ArchivedOn = %v, want %s (archived, not destroyed)", stored[0].ArchivedOn, deleteOn)
	}

	snap, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Today.Groups) != 0 {
		t.Fatalf("archived habit must not appear in today's groups on/after archive day: %+v", snap.Today.Groups)
	}

	dayBefore := deleteOn.AddDays(-1)
	beforeDay, ok := findCalendarDay(snap.Calendar, dayBefore)
	if !ok {
		t.Fatalf("day before delete (%s) missing from calendar", dayBefore)
	}
	if beforeDay.Summary.Scheduled != 1 {
		t.Fatalf("day before archive: Scheduled = %d, want 1 (history must still show it scheduled)", beforeDay.Summary.Scheduled)
	}
}

// --- ToggleHabit rejects habits that are not scheduled today ----------------

func TestServiceToggleHabitRejectsArchivedHabit(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	svc := New(store, domain.NewFixedClock(today))

	if err := svc.AddHabit(ctx, NewHabit{Title: "Old habit"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	id := habits[0].ID

	if err := store.ArchiveHabit(ctx, id, today); err != nil {
		t.Fatalf("ArchiveHabit: %v", err)
	}

	err := svc.ToggleHabit(ctx, id)
	if err == nil {
		t.Fatal("ToggleHabit on an archived (as of today) habit should be rejected")
	}
	if !errors.Is(err, ErrNotScheduled) {
		t.Fatalf("ToggleHabit error = %v, want it to wrap ErrNotScheduled", err)
	}

	comps, err := store.CompletionsBetween(ctx, today, today)
	if err != nil || len(comps) != 0 {
		t.Fatalf("CompletionsBetween after a rejected toggle = %+v, %v, want no completion written", comps, err)
	}
}

func TestServiceToggleHabitRejectsUnscheduledWeekday(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10") // a Monday
	svc := New(store, domain.NewFixedClock(today))

	// Scheduled Tuesday only: not scheduled on Monday.
	tuesdayOnly := domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(time.Tuesday)}
	if _, err := store.CreateHabit(ctx, domain.Habit{
		Title: "Tuesday thing", Schedule: tuesdayOnly, CreatedOn: today.AddDays(-30),
	}); err != nil {
		t.Fatalf("CreateHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	id := habits[0].ID

	err := svc.ToggleHabit(ctx, id)
	if err == nil {
		t.Fatal("ToggleHabit on a day the habit's schedule doesn't apply should be rejected")
	}
	if !errors.Is(err, ErrNotScheduled) {
		t.Fatalf("ToggleHabit error = %v, want it to wrap ErrNotScheduled", err)
	}
}

// --- completion + same-day archive: totals must agree with the daily summary

func TestServiceCompletingThenArchivingSameDayKeepsTotalsConsistent(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	svc := New(store, domain.NewFixedClock(today))

	if err := svc.AddHabit(ctx, NewHabit{Title: "One-off"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	id := habits[0].ID

	if err := svc.ToggleHabit(ctx, id); err != nil {
		t.Fatalf("ToggleHabit: %v", err)
	}
	before, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot (before archive): %v", err)
	}
	if before.Today.Summary.Completed != 1 || before.Overview.TotalCompletions != 1 {
		t.Fatalf("before archive: Today.Summary=%+v Overview.TotalCompletions=%d, want Completed=1 and TotalCompletions=1",
			before.Today.Summary, before.Overview.TotalCompletions)
	}

	// Archive the habit on the very day it was completed. Per the ActiveOn
	// contract (architecture doc: created_on <= D && D < archived_on), the
	// habit stops being "scheduled" on archived_on itself, even though the
	// completion row for that date still physically exists.
	if err := svc.DeleteHabit(ctx, id); err != nil {
		t.Fatalf("DeleteHabit: %v", err)
	}

	after, err := svc.Snapshot(ctx, SnapshotRequest{})
	if err != nil {
		t.Fatalf("Snapshot (after archive): %v", err)
	}
	if after.Today.Summary.Scheduled != 0 || after.Today.Summary.Completed != 0 {
		t.Fatalf("after archive: Today.Summary = %+v, want Scheduled=0 Completed=0 (no longer scheduled that day)", after.Today.Summary)
	}
	if after.Overview.TotalCompletions != 0 {
		t.Fatalf("after archive: Overview.TotalCompletions = %d, want 0 (must agree with the daily summary, not the raw completion row)", after.Overview.TotalCompletions)
	}

	stored, err := store.DailySummaries(ctx, today, today)
	if err != nil || len(stored) != 1 {
		t.Fatalf("DailySummaries = %+v, %v, want exactly one row", stored, err)
	}
	if stored[0].Scheduled != 0 || stored[0].Completed != 0 {
		t.Fatalf("persisted daily summary = %+v, want Scheduled=0 Completed=0, matching Overview.TotalCompletions", stored[0])
	}

	// The raw completion row itself must survive archiving: completions are
	// the source of truth for history and are never deleted by archiving.
	comps, err := store.CompletionsBetween(ctx, today, today)
	if err != nil || len(comps) != 1 {
		t.Fatalf("CompletionsBetween = %+v, %v, want the completion row to survive archiving", comps, err)
	}
}

// --- failure injection: ToggleCompletion fails atomically, nothing is written
//
// This replaces an older test that assumed Service.ToggleHabit wrote the
// completion and the derived summary as two separate steps (a completion
// write, then an independent PutDailySummary call that could fail on its
// own leaving the completion behind). ports.go's ToggleCompletion doc
// comment retired that in favour of one atomic store-side transaction
// specifically so a failure can never land between the two writes — so the
// only failure this fake can still meaningfully model is "the whole toggle
// failed, nothing changed", mirroring the real store's tx.Rollback().

func TestServiceSurfacesToggleCompletionFailureAtomically(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	today := domain.MustParseDate("2026-08-10")
	svc := New(store, domain.NewFixedClock(today))

	if err := svc.AddHabit(ctx, NewHabit{Title: "Flaky"}); err != nil {
		t.Fatalf("AddHabit: %v", err)
	}
	habits, _ := store.Habits(ctx)
	id := habits[0].ID

	store.toggleCompletionFailNext = true
	err := svc.ToggleHabit(ctx, id)
	if err == nil {
		t.Fatal("ToggleHabit should surface the injected ToggleCompletion failure")
	}
	if !errors.Is(err, errInjectedToggleFailure) {
		t.Fatalf("ToggleHabit error = %v, want it to wrap the injected failure", err)
	}

	// Atomic means atomic: the failed toggle must not record a completion,
	// nor disturb the summary AddHabit already wrote (Scheduled=1,
	// Completed=0) when it created the habit.
	comps, err := store.CompletionsBetween(ctx, today, today)
	if err != nil || len(comps) != 0 {
		t.Fatalf("CompletionsBetween after a failed atomic toggle = %+v, %v, want nothing recorded (the transaction must roll back)", comps, err)
	}
	stored, err := store.DailySummaries(ctx, today, today)
	if err != nil || len(stored) != 1 || stored[0].Completed != 0 || stored[0].Scheduled != 1 {
		t.Fatalf("DailySummaries after a failed atomic toggle = %+v, %v, want the pre-existing Scheduled=1 Completed=0 row untouched", stored, err)
	}

	// The failure was injected for exactly one call: a retry must succeed
	// normally and leave a fully consistent completion + summary pair.
	if err := svc.ToggleHabit(ctx, id); err != nil {
		t.Fatalf("ToggleHabit after the injected failure cleared: %v", err)
	}
	stored, err = store.DailySummaries(ctx, today, today)
	if err != nil || len(stored) != 1 || stored[0].Completed != 1 || stored[0].Scheduled != 1 {
		t.Fatalf("DailySummaries after a successful retry = %+v, %v, want one row with Scheduled=1 Completed=1", stored, err)
	}
}
