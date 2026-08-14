package ui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
)

// This file is QA's independent verification of the navigation contract on
// top of ui's own fakeBackend/keyMsg/pressKey/send/step helpers (defined in
// fake_test.go, model_test.go, focus_test.go). It deliberately does not
// re-test what those files already cover well (esc closes a modal, q only
// quits without a modal, help blocks other keys, enter toggles a habit or
// reminder) — see docs/QA.md for the cross-check. It focuses on: full
// multi-step wrap cycles driven through real key events, independent
// verification of block/item counts, calendar month navigation, and render safety
// requirement (no line ever exceeds the terminal width, across a range of
// sizes, without panicking).

// newRichBackend builds a fakeBackend with 2 Today groups (ungrouped=2
// items, "Morning"=3 items) and 2 reminders. Today also has its Add block;
// Reminders has action, calendar, and list blocks.
func newRichBackend(today domain.Date) *fakeBackend {
	b := newFakeBackend(today)
	b.withRoutines(domain.Routine{Title: "Morning"})
	routineID := int64(1) // first ID handed out, by withRoutines above
	b.withHabits(
		domain.Habit{Title: "Wake up"},
		domain.Habit{Title: "Journal"},
		domain.Habit{Title: "Drink water", RoutineID: &routineID},
		domain.Habit{Title: "Stretch", RoutineID: &routineID},
		domain.Habit{Title: "Meditate", RoutineID: &routineID},
	)
	b.withReminders(
		domain.Reminder{Title: "Exam", Date: today.AddDays(3)},
		domain.Reminder{Title: "Bill", Date: today.AddDays(5)},
	)
	return b
}

// newEmptyBackend has no habits and no reminders: exercises the "empty
// column/block must be a no-op, never a panic" requirement end-to-end
// through the real Update loop (complementary to focus_test.go's
// TestClampFocusHandlesEmptyColumn, which drives clampFocus directly without
// going through key events or rendering).
func newEmptyBackend(today domain.Date) *fakeBackend {
	return newFakeBackend(today)
}

// --- left/right: wrap, reset block+item ----------------------------------

func TestSpecNavigationLeftRightFullWrapCycleResetsBlockItem(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))

	// Move somewhere with a nonzero block/item so the reset is observable.
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "down")
	if root.focus.Block == 0 && root.focus.Item == 0 {
		t.Fatal("test setup: expected a nonzero block or item before switching columns")
	}
	if root.focus.Column != ColOverview {
		t.Fatalf("starting column = %d, want ColOverview", root.focus.Column)
	}

	root = pressKey(t, root, "left")
	if root.focus != (Focus{Column: ColReminders, Block: 0, Item: 0}) {
		t.Fatalf("after left from Overview: focus = %+v, want wrap to ColReminders with Block/Item reset to 0", root.focus)
	}
	root = pressKey(t, root, "left")
	if root.focus.Column != ColToday || root.focus.Block != 0 || root.focus.Item != 0 {
		t.Fatalf("after second left: focus = %+v, want ColToday/0/0", root.focus)
	}
	root = pressKey(t, root, "left")
	if root.focus.Column != ColOverview || root.focus.Block != 0 || root.focus.Item != 0 {
		t.Fatalf("after third left (full wrap): focus = %+v, want ColOverview/0/0", root.focus)
	}

	// Right must wrap the same way, in the opposite direction.
	root = pressKey(t, root, "right")
	if root.focus.Column != ColToday {
		t.Fatalf("after right from Overview: Column = %d, want ColToday", root.focus.Column)
	}
	root = pressKey(t, root, "right")
	if root.focus.Column != ColReminders {
		t.Fatalf("after second right: Column = %d, want ColReminders", root.focus.Column)
	}
	root = pressKey(t, root, "right")
	if root.focus.Column != ColOverview {
		t.Fatalf("after third right (full wrap): Column = %d, want ColOverview", root.focus.Column)
	}
}

// --- tab/shift+tab: wrap within column, reset item -------------------------

func TestSpecNavigationTabFullWrapCycleResetsItem(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))

	if bc := root.blockCount(ColOverview); bc != numOverviewBlocks {
		t.Fatalf("blockCount(Overview) = %d, want %d", bc, numOverviewBlocks)
	}

	wantBlocks := []int{1, 0} // completion, then wrap to lifetime
	for _, want := range wantBlocks {
		root = pressKey(t, root, "tab")
		if root.focus.Block != want {
			t.Fatalf("after tab: Block = %d, want %d", root.focus.Block, want)
		}
		if root.focus.Item != 0 {
			t.Fatalf("after tab: Item = %d, want 0 (reset)", root.focus.Item)
		}
	}

	// shift+tab from Block 0 must wrap to the last block.
	root = pressKey(t, root, "shift+tab")
	if root.focus.Block != numOverviewBlocks-1 {
		t.Fatalf("after shift+tab from Block 0: Block = %d, want %d (wrap to last)", root.focus.Block, numOverviewBlocks-1)
	}
}

// --- up/down: clamp, no wrap, driven through real key events ----------------

func TestSpecNavigationUpDownClampWithoutWrapping(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))

	// Move to the Today column, "Morning" block (3 items: indices 0..2).
	root = pressKey(t, root, "right")
	root = pressKey(t, root, "tab") // Add -> ungrouped
	root = pressKey(t, root, "tab") // ungrouped -> Morning
	if root.focus.Column != ColToday || root.focus.Block != 2 {
		t.Fatalf("setup: focus = %+v, want ColToday block 2 (Morning)", root.focus)
	}

	root = pressKey(t, root, "down")
	if root.focus.Item != 1 {
		t.Fatalf("after down: Item = %d, want 1", root.focus.Item)
	}
	root = pressKey(t, root, "down")
	if root.focus.Item != 2 {
		t.Fatalf("after second down: Item = %d, want 2 (last item)", root.focus.Item)
	}
	root = pressKey(t, root, "down")
	if root.focus.Item != 2 {
		t.Fatalf("after down past the end: Item = %d, want 2 (clamped, not wrapped to 0)", root.focus.Item)
	}

	root = pressKey(t, root, "up")
	root = pressKey(t, root, "up")
	if root.focus.Item != 0 {
		t.Fatalf("after two ups: Item = %d, want 0", root.focus.Item)
	}
	root = pressKey(t, root, "up")
	if root.focus.Item != 0 {
		t.Fatalf("after up past the start: Item = %d, want 0 (clamped, not wrapped to the last item)", root.focus.Item)
	}
}

// --- block/item counts match the ratified contract --------------------------

func TestSpecNavigationBlockAndItemCountsMatchSnapshot(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	snap := root.snap

	if got := root.blockCount(ColOverview); got != 2 {
		t.Fatalf("blockCount(Overview) = %d, want 2", got)
	}
	for _, block := range []int{OverviewLifetime, OverviewCompletion} {
		if got := root.itemCount(ColOverview, block); got != 1 {
			t.Fatalf("itemCount(Overview, block %d) = %d, want 1", block, got)
		}
	}

	if got := root.blockCount(ColToday); got != 1+len(snap.Today.Groups) {
		t.Fatalf("blockCount(Today) = %d, want %d (Add + len(Groups))", got, 1+len(snap.Today.Groups))
	}
	if got := root.itemCount(ColToday, TodayActions); got != 2 {
		t.Fatalf("itemCount(Today, Add) = %d, want 2", got)
	}
	if len(snap.Today.Groups) != 2 || len(snap.Today.Groups[0].Items) != 2 || len(snap.Today.Groups[1].Items) != 3 {
		t.Fatalf("fixture drifted: Today.Groups = %+v, want [2 items, 3 items]", snap.Today.Groups)
	}
	for i, g := range snap.Today.Groups {
		if got := root.itemCount(ColToday, i+1); got != len(g.Items) {
			t.Fatalf("itemCount(Today, %d) = %d, want %d", i+1, got, len(g.Items))
		}
	}

	if got := root.blockCount(ColReminders); got != 3 {
		t.Fatalf("blockCount(Reminders) = %d, want 3", got)
	}
	if got := root.itemCount(ColReminders, RemindersActions); got != 1 {
		t.Fatalf("itemCount(Reminders, Actions) = %d, want 1", got)
	}
	if got := root.itemCount(ColReminders, RemindersCalendar); got != 1 {
		t.Fatalf("itemCount(Reminders, Calendar) = %d, want 1", got)
	}
	if got := root.itemCount(ColReminders, RemindersList); got != len(snap.Reminders) {
		t.Fatalf("itemCount(Reminders, List) = %d, want %d (len(Reminders))", got, len(snap.Reminders))
	}
}

// --- empty column/block: no-op, never panics, end-to-end --------------------

func TestSpecNavigationTodayActionsRemainAvailableWithoutHabits(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newEmptyBackend(today))

	root = pressKey(t, root, "right") // Overview -> Today
	if root.focus.Column != ColToday || root.focus.Block != 0 || root.focus.Item != 0 {
		t.Fatalf("focus after entering the empty Today column = %+v, want ColToday/0/0", root.focus)
	}

	// Today still has one useful block even when no habits are scheduled.
	root = pressKey(t, root, "down")
	if root.focus.Item != ActionAddRoutine {
		t.Fatalf("down in empty Today's Add block selected item %d, want Add Routine", root.focus.Item)
	}
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "shift+tab")
	root = pressKey(t, root, "up")
	if root.focus.Column != ColToday || root.focus.Block != 0 || root.focus.Item != 0 {
		t.Fatalf("focus after navigating the Add block = %+v, want ColToday/0/0", root.focus)
	}
}

// --- focus re-clamps after a new snapshot -----------------------------------

func TestSpecNavigationFocusReclampsAfterNewSnapshot(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))

	// Focus the last item of the "Morning" block (block 1, item 2 of 3).
	root = pressKey(t, root, "right")
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "down")
	root = pressKey(t, root, "down")
	if root.focus != (Focus{Column: ColToday, Block: 2, Item: 2}) {
		t.Fatalf("setup: focus = %+v, want ColToday/2/2", root.focus)
	}

	// A fresh snapshot shrinks Today down to a single, smaller group.
	shrinkBackend := newFakeBackend(today)
	shrinkBackend.withHabits(domain.Habit{Title: "A"}, domain.Habit{Title: "B"})
	shrunk, err := shrinkBackend.Snapshot(context.Background(), app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("building the shrunk snapshot: %v", err)
	}
	if len(shrunk.Today.Groups) != 1 || len(shrunk.Today.Groups[0].Items) != 2 {
		t.Fatalf("fixture drifted: shrunk Today.Groups = %+v, want a single 2-item group", shrunk.Today.Groups)
	}
	root = send(t, root, snapshotMsg{snap: shrunk, reqID: root.snapReqID})

	if root.focus.Column != ColToday {
		t.Fatalf("Column after reclamp = %d, want unchanged ColToday", root.focus.Column)
	}
	if root.focus.Block != 1 {
		t.Fatalf("Block after reclamp = %d, want 1 (Add plus one group left)", root.focus.Block)
	}
	if root.focus.Item != 1 {
		t.Fatalf("Item after reclamp = %d, want 1 (clamped to the new last index)", root.focus.Item)
	}
}

// --- snapshot reqID guard: stale responses are discarded, current ones apply --

func TestSpecNavigationSnapshotReqIDGuardDiscardsStaleResponses(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))

	shrinkBackend := newFakeBackend(today)
	shrinkBackend.withHabits(domain.Habit{Title: "A"}, domain.Habit{Title: "B"})
	shrunk, err := shrinkBackend.Snapshot(context.Background(), app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("building the shrunk snapshot: %v", err)
	}

	staleReqID := root.snapReqID - 1 // superseded by the initial fetch from newReadyRoot
	before := root.snap
	root = send(t, root, snapshotMsg{snap: shrunk, reqID: staleReqID})
	if !reflect.DeepEqual(root.snap, before) {
		t.Fatalf("a snapshot stamped with a stale reqID (%d, current %d) was applied; snap changed from %+v to %+v",
			staleReqID, root.snapReqID, before, root.snap)
	}

	root = send(t, root, snapshotMsg{snap: shrunk, reqID: root.snapReqID})
	if !reflect.DeepEqual(root.snap, shrunk) {
		t.Fatalf("a snapshot stamped with the current reqID was not applied; snap = %+v, want %+v", root.snap, shrunk)
	}
}

// --- calendar block: up/down are item navigation only, </> shift the month --
//
// SPEC.md reserves up/down for "move between selectable items inside the
// current block" everywhere, with no calendar-specific exception. Month
// navigation instead lives on </> (with [/] as aliases) — see keys.go's
// stepCalendar. This used to assert up/down shifted the month; that binding
// was off-spec and has been moved.

func TestSpecNavigationCalendarBlockUpDownNeverShiftsMonth(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	root.focus = Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0}

	if !root.calMonth.IsZero() {
		t.Fatalf("setup: calMonth should start zero, got %s", root.calMonth)
	}

	root = pressKey(t, root, "down")
	if !root.calMonth.IsZero() {
		t.Fatalf("calMonth after down = %s, want unchanged zero (up/down is item navigation only, per SPEC)", root.calMonth)
	}
	if root.focus.Item != 0 {
		t.Fatalf("Item after down on the calendar block = %d, want unchanged 0 (the calendar block has a single selectable item)", root.focus.Item)
	}

	root = pressKey(t, root, "up")
	if !root.calMonth.IsZero() {
		t.Fatalf("calMonth after up = %s, want unchanged zero", root.calMonth)
	}
}

func TestSpecNavigationCalendarBlockAngleBracketsShiftMonth(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	root.focus = Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0}

	if !root.calMonth.IsZero() {
		t.Fatalf("setup: calMonth should start zero, got %s", root.calMonth)
	}

	root = pressKey(t, root, ">")
	want := today.StartOfMonth().AddMonths(1)
	if root.calMonth != want {
		t.Fatalf("calMonth after '>' = %s, want %s", root.calMonth, want)
	}
	if root.focus.Item != 0 {
		t.Fatalf("Item after '>' on the calendar block = %d, want unchanged 0", root.focus.Item)
	}
	if root.snap.Calendar.Month != want.Month || root.snap.Calendar.Year != want.Year {
		t.Fatalf("Calendar after '>' = %s %d, want %s %d (snapshot must be refetched for the new month)",
			root.snap.Calendar.Month, root.snap.Calendar.Year, want.Month, want.Year)
	}

	root = pressKey(t, root, "<")
	if !root.calMonth.Equal(today.StartOfMonth()) {
		t.Fatalf("calMonth after '>' then '<' = %s, want back to %s", root.calMonth, today.StartOfMonth())
	}

	// [ and ] are documented aliases for </>.
	root = pressKey(t, root, "]")
	if !root.calMonth.Equal(want) {
		t.Fatalf("calMonth after ']' = %s, want %s (] is an alias for >)", root.calMonth, want)
	}
	root = pressKey(t, root, "[")
	if !root.calMonth.Equal(today.StartOfMonth()) {
		t.Fatalf("calMonth after '[' = %s, want %s ([ is an alias for <)", root.calMonth, today.StartOfMonth())
	}
}

func TestSpecNavigationReminderCalendarSharesViewedMonth(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	root.focus = Focus{Column: ColReminders, Block: RemindersCalendar}

	root = pressKey(t, root, "]")
	want := domain.MustParseDate("2026-09-01")
	if root.calMonth != want {
		t.Fatalf("calMonth after ] on Reminders calendar = %s, want %s", root.calMonth, want)
	}
	if root.snap.Calendar.Month != want.Month || root.snap.Calendar.Year != want.Year {
		t.Fatalf("shared snapshot month = %s %d, want %s %d", root.snap.Calendar.Month, root.snap.Calendar.Year, want.Month, want.Year)
	}

	root.focus = Focus{Column: ColReminders, Block: RemindersList}
	root = pressKey(t, root, "[")
	if root.calMonth != want {
		t.Fatalf("month key on reminder list changed calMonth to %s; it should only act on calendar blocks", root.calMonth)
	}
}

// --- month navigation boundaries: Jan 31 / Mar 31 must not skip a month -----
//
// shiftMonth normalises to the 1st of the month before adding (see keys.go),
// so navigating from a 31st never lands on the wrong month the way raw
// time.AddDate arithmetic on an arbitrary day-of-month would (Jan 31 + 1
// naive month is March 3, skipping February entirely).

func TestSpecMonthNavigationJan31ForwardDoesNotSkipFebruary(t *testing.T) {
	today := domain.MustParseDate("2026-01-31")
	root := newReadyRoot(t, newRichBackend(today))
	root.focus = Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0}

	root = pressKey(t, root, ">")
	wantFeb := domain.MustParseDate("2026-02-01")
	if root.calMonth != wantFeb {
		t.Fatalf("calMonth after '>' from Jan 31 = %s, want %s (must not skip to March)", root.calMonth, wantFeb)
	}
	if root.snap.Calendar.Month != wantFeb.Month || root.snap.Calendar.Year != wantFeb.Year {
		t.Fatalf("Calendar after '>' from Jan 31 = %s %d, want %s %d", root.snap.Calendar.Month, root.snap.Calendar.Year, wantFeb.Month, wantFeb.Year)
	}

	root = pressKey(t, root, ">")
	wantMar := domain.MustParseDate("2026-03-01")
	if root.calMonth != wantMar {
		t.Fatalf("calMonth after second '>' = %s, want %s", root.calMonth, wantMar)
	}

	root = pressKey(t, root, "<")
	if root.calMonth != wantFeb {
		t.Fatalf("calMonth after '<' back from March = %s, want %s", root.calMonth, wantFeb)
	}
}

func TestSpecMonthNavigationMar31BackwardDoesNotStayInMarch(t *testing.T) {
	today := domain.MustParseDate("2026-03-31")
	root := newReadyRoot(t, newRichBackend(today))
	root.focus = Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0}

	root = pressKey(t, root, "<")
	wantFeb := domain.MustParseDate("2026-02-01")
	if root.calMonth != wantFeb {
		t.Fatalf("calMonth after '<' from Mar 31 = %s, want %s (must actually leave March)", root.calMonth, wantFeb)
	}
	if root.snap.Calendar.Month != wantFeb.Month || root.snap.Calendar.Year != wantFeb.Year {
		t.Fatalf("Calendar after '<' from Mar 31 = %s %d, want %s %d", root.snap.Calendar.Month, root.snap.Calendar.Year, wantFeb.Month, wantFeb.Year)
	}

	root = pressKey(t, root, "<")
	wantJan := domain.MustParseDate("2026-01-01")
	if root.calMonth != wantJan {
		t.Fatalf("calMonth after second '<' = %s, want %s", root.calMonth, wantJan)
	}

	root = pressKey(t, root, ">")
	if root.calMonth != wantFeb {
		t.Fatalf("calMonth after '>' back from January = %s, want %s", root.calMonth, wantFeb)
	}
}

// --- responsive rendering: the explicit render-safety requirement -----------

func TestSpecRenderNeverExceedsTerminalWidthOrPanics(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	sizes := [][2]int{{80, 20}, {96, 24}, {120, 40}, {200, 60}}
	fixtures := map[string]func() *fakeBackend{
		"rich":  func() *fakeBackend { return newRichBackend(today) },
		"empty": func() *fakeBackend { return newEmptyBackend(today) },
	}

	for name, newBackend := range fixtures {
		for _, wh := range sizes {
			w, h := wh[0], wh[1]
			t.Run(fmt.Sprintf("%s_%dx%d", name, w, h), func(t *testing.T) {
				root := newReadyRoot(t, newBackend())
				root = send(t, root, tea.WindowSizeMsg{Width: w, Height: h})

				var content string
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Fatalf("View() panicked at %dx%d: %v", w, h, r)
						}
					}()
					content = root.View().Content
				}()

				for i, line := range strings.Split(content, "\n") {
					if lw := lipgloss.Width(line); lw > w {
						t.Fatalf("line %d is %d cells wide, want <= %d terminal width:\n%q", i, lw, w, line)
					}
				}

				belowMin := w < MinWidth || h < MinHeight
				gotMinMsg := strings.Contains(content, "loopline needs a little more room")
				if belowMin != gotMinMsg {
					t.Fatalf("%dx%d: min-size message shown = %v, want %v (MinWidth=%d MinHeight=%d)", w, h, gotMinMsg, belowMin, MinWidth, MinHeight)
				}
			})
		}
	}
}
