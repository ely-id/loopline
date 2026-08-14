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

func newRichBackend(today domain.Date) *fakeBackend {
	b := newFakeBackend(today)
	b.withRoutines(domain.Routine{Title: "Morning"})
	routineID := int64(1)
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

func newEmptyBackend(today domain.Date) *fakeBackend { return newFakeBackend(today) }

func TestHeaderIsInitialNavigationLevel(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	if root.navigation != NavigationHeader {
		t.Fatalf("initial navigation = %d, want header", root.navigation)
	}
	if root.focus != (Focus{Column: ColOverview, Block: OverviewLifetime}) {
		t.Fatalf("initial focus = %+v, want Overview header entry/local default", root.focus)
	}
	if !strings.Contains(stripANSI(root.renderHeader()), "OVERVIEW") {
		t.Fatal("global header does not expose Overview navigation")
	}
}

func TestHeaderTabCyclesPanelsAndReverseTabCyclesBack(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	for _, want := range []int{ColToday, ColReminders, ColOverview} {
		root = pressKey(t, root, "tab")
		if root.focus.Column != want || root.navigation != NavigationHeader {
			t.Fatalf("after Tab focus=%+v mode=%d, want column %d in header", root.focus, root.navigation, want)
		}
	}
	root = pressKey(t, root, "shift+tab")
	if root.focus.Column != ColReminders {
		t.Fatalf("reverse Tab column = %d, want Reminders", root.focus.Column)
	}
}

func TestEnterAndEscapeMoveBetweenHeaderAndSelectedPanel(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root = pressKey(t, root, "tab") // Habits
	root = pressKey(t, root, "enter")
	if root.navigation != NavigationPanel || root.focus.Column != ColToday {
		t.Fatalf("after Enter mode=%d focus=%+v, want Habits panel", root.navigation, root.focus)
	}
	root = pressKey(t, root, "esc")
	if root.navigation != NavigationHeader || root.focus.Column != ColToday {
		t.Fatalf("after Esc mode=%d focus=%+v, want Habits header entry", root.navigation, root.focus)
	}
}

func TestPanelUpDownCrossesBlocksAndTabDoesNotCreateSecondLocalNavigation(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "enter")
	root = pressKey(t, root, "down") // Add Routine
	root = pressKey(t, root, "down") // first ungrouped habit
	if root.focus != (Focus{Column: ColToday, Block: 1, Item: 0}) {
		t.Fatalf("continuous down focus = %+v, want first ungrouped habit", root.focus)
	}
	root = pressKey(t, root, "down") // second ungrouped
	root = pressKey(t, root, "down") // routine header
	if root.focus != (Focus{Column: ColToday, Block: 2, Item: 0}) {
		t.Fatalf("focus after crossing into routine = %+v, want routine header", root.focus)
	}
	before := root.focus
	root = pressKey(t, root, "tab")
	if root.focus != before {
		t.Fatalf("Tab moved local focus from %+v to %+v", before, root.focus)
	}
	root = pressKey(t, root, "down")
	if root.focus != (Focus{Column: ColToday, Block: 2, Item: 1}) {
		t.Fatalf("routine child focus = %+v, want first child after header", root.focus)
	}
}

func TestOverviewLocalNavigationCrossesWeeklyRows(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root = pressKey(t, root, "enter")
	root = pressKey(t, root, "down")
	if root.focus.Block != OverviewWeekly || root.focus.Item != 0 {
		t.Fatalf("down from Lifetime = %+v, want first Weekly row", root.focus)
	}
	for range root.snap.Weekly.Rows {
		root = pressKey(t, root, "down")
	}
	if root.focus.Block != OverviewCompletion {
		t.Fatalf("down through Weekly rows = %+v, want Completion", root.focus)
	}
	root = pressKey(t, root, "down")
	if root.focus.Block != OverviewCompletion {
		t.Fatalf("down past panel end wrapped: %+v", root.focus)
	}
}

func TestEmptyWeeklyAndReminderListsDoNotTrapLocalNavigation(t *testing.T) {
	root := newReadyRoot(t, newEmptyBackend(domain.MustParseDate("2026-08-10")))
	root = pressKey(t, root, "enter")
	root = pressKey(t, root, "down")
	if root.focus.Block != OverviewCompletion {
		t.Fatalf("empty Weekly block trapped focus: %+v", root.focus)
	}
	root = pressKey(t, root, "esc")
	root = pressKey(t, root, "shift+tab") // Reminders
	root = pressKey(t, root, "enter")
	root = pressKey(t, root, "down")
	if root.focus.Block != RemindersCalendar {
		t.Fatalf("down from Add in empty Reminders = %+v, want Calendar", root.focus)
	}
	root = pressKey(t, root, "down")
	if root.focus.Block != RemindersCalendar {
		t.Fatalf("empty reminder list should clamp at Calendar: %+v", root.focus)
	}
}

func TestSnapshotReclampsLocalFocusAfterDeletion(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	root.navigation = NavigationPanel
	root.focus = Focus{Column: ColToday, Block: 2, Item: 3}

	shrinkBackend := newFakeBackend(today).withHabits(domain.Habit{Title: "A"}, domain.Habit{Title: "B"})
	shrunk, err := shrinkBackend.Snapshot(context.Background(), app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("build shrunk snapshot: %v", err)
	}
	root = send(t, root, snapshotMsg{snap: shrunk, reqID: root.snapReqID})
	if root.focus != (Focus{Column: ColToday, Block: 1, Item: 1}) {
		t.Fatalf("reclamped focus = %+v, want last surviving habit", root.focus)
	}
}

func TestSnapshotRequestGuardStillDiscardsStaleResponses(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newRichBackend(today))
	shrunk, err := newFakeBackend(today).withHabits(domain.Habit{Title: "A"}).Snapshot(context.Background(), app.SnapshotRequest{})
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	before := root.snap
	root = send(t, root, snapshotMsg{snap: shrunk, reqID: root.snapReqID - 1})
	if !reflect.DeepEqual(root.snap, before) {
		t.Fatal("stale snapshot was applied")
	}
	root = send(t, root, snapshotMsg{snap: shrunk, reqID: root.snapReqID})
	if !reflect.DeepEqual(root.snap, shrunk) {
		t.Fatal("current snapshot was not applied")
	}
}

func TestMonthNavigationOnlyActsInsideFocusedCalendar(t *testing.T) {
	today := domain.MustParseDate("2026-01-31")
	root := newReadyRoot(t, newRichBackend(today))
	root = pressKey(t, root, "]")
	if !root.calMonth.IsZero() {
		t.Fatal("month changed while header had focus")
	}
	root.navigation = NavigationPanel
	root.focus = Focus{Column: ColOverview, Block: OverviewCompletion}
	root = pressKey(t, root, ">")
	if !root.calMonth.IsZero() {
		t.Fatalf("month key changed the conventional Calendar while Completion was focused: %s", root.calMonth)
	}

	root.focus = Focus{Column: ColReminders, Block: RemindersCalendar}
	root = pressKey(t, root, "]")
	want := domain.MustParseDate("2026-02-01")
	if root.calMonth != want {
		t.Fatalf("Reminder Calendar month = %s, want February", root.calMonth)
	}
	root.focus = Focus{Column: ColReminders, Block: RemindersList}
	root = pressKey(t, root, "[")
	if root.calMonth != want {
		t.Fatal("month key acted outside a calendar target")
	}
	root.focus = Focus{Column: ColReminders, Block: RemindersCalendar}
	root = pressKey(t, root, "[")
	if root.calMonth != today.StartOfMonth() {
		t.Fatalf("month after return = %s, want January", root.calMonth)
	}
}

func TestSpecRenderNeverExceedsTerminalWidthOrPanics(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	sizes := [][2]int{{80, 20}, {96, 33}, {120, 40}, {200, 60}}
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
				content := root.View().Content
				for i, line := range strings.Split(content, "\n") {
					if got := lipgloss.Width(line); got > w {
						t.Fatalf("line %d is %d cells wide, want <= %d", i, got, w)
					}
				}
				belowMin := w < MinWidth || h < MinHeight
				if got := strings.Contains(content, "loopline needs a little more room"); got != belowMin {
					t.Fatalf("minimum-size message=%v, want %v", got, belowMin)
				}
			})
		}
	}
}
