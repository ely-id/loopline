package ui

import (
	"strings"
	"testing"

	"loopline/internal/domain"
)

func replaceFocusedText(t *testing.T, root *Root, old, replacement string) *Root {
	t.Helper()
	for range []rune(old) {
		root = pressKey(t, root, "backspace")
	}
	for _, r := range replacement {
		root = send(t, root, keyMsg(string(r)))
	}
	return root
}

func hasCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func confirmDeletion(t *testing.T, root *Root) *Root {
	t.Helper()
	root = pressKey(t, root, "y")
	if root.modal != nil {
		t.Fatal("accepting deletion did not close the confirmation modal")
	}
	return root
}

func TestEditHabitUsesSelectedExpandedRoutineChildAndPreservesIdentity(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newRichBackend(today)
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 2, Item: 1}
	original := root.snap.Today.Groups[1].Items[1].Habit

	root = pressKey(t, root, "e")
	if root.modal == nil {
		t.Fatal("e on a habit did not open an edit form")
	}
	view := stripANSI(root.modal.form.View())
	if !strings.Contains(view, "Edit habit") || !strings.Contains(view, original.Title) {
		t.Fatalf("habit edit form is not pre-populated:\n%s", view)
	}
	root = replaceFocusedText(t, root, original.Title, "Mobility")
	for range 5 {
		root = pressKey(t, root, "enter")
	}

	if !hasCall(backend.calls, "EditHabit") {
		t.Fatalf("habit edit did not reach Backend.EditHabit: %v", backend.calls)
	}
	var edited domain.Habit
	for _, habit := range backend.habits {
		if habit.ID == original.ID {
			edited = habit
		}
	}
	if edited.ID != original.ID || edited.Title != "Mobility" || edited.RoutineID == nil || *edited.RoutineID != *original.RoutineID {
		t.Fatalf("habit was recreated or edited incorrectly: original=%+v edited=%+v", original, edited)
	}
	if backend.routines[0].Title != "Morning" {
		t.Fatal("e on an expanded routine child edited the routine instead of the habit")
	}
}

func TestEditRoutineUsesCollapsedHeaderAndPreservesChildren(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newRichBackend(today)
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 2, Item: 2}
	routineID := *root.snap.Today.Groups[1].RoutineID
	root = pressKey(t, root, "t")
	root = pressKey(t, root, "e")
	if root.modal == nil || !strings.Contains(stripANSI(root.modal.form.View()), "Edit routine") {
		t.Fatal("e on a collapsed routine header did not open its edit form")
	}
	root = replaceFocusedText(t, root, "Morning", "First light")
	root = pressKey(t, root, "enter")

	if backend.routines[0].ID != routineID || backend.routines[0].Title != "First light" {
		t.Fatalf("routine was recreated or not renamed: %+v", backend.routines)
	}
	children := 0
	for _, habit := range backend.habits {
		if habit.RoutineID != nil && *habit.RoutineID == routineID {
			children++
		}
	}
	if children != 3 {
		t.Fatalf("routine edit changed membership: got %d children, want 3", children)
	}
}

func TestEditReminderWorksWhileExpandedAndPreservesStatus(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withReminders(domain.Reminder{
		Title: "Exam", Date: today.AddDays(2), Notes: "room 1", Completed: true,
	})
	originalID := backend.reminders[0].ID
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}
	root = pressKey(t, root, "t")
	root = pressKey(t, root, "e")
	if root.modal == nil {
		t.Fatal("e on an expanded reminder did not open an edit form")
	}
	view := stripANSI(root.modal.form.View())
	if !strings.Contains(view, "Edit reminder") || !strings.Contains(view, "Exam") || !strings.Contains(view, "2026-08-12") {
		t.Fatalf("reminder edit form is not pre-populated:\n%s", view)
	}
	root = replaceFocusedText(t, root, "Exam", "Algorithms exam")
	for range 3 {
		root = pressKey(t, root, "enter")
	}

	got := backend.reminders[0]
	if got.ID != originalID || got.Title != "Algorithms exam" || !got.Completed || got.Notes != "room 1" {
		t.Fatalf("reminder edit lost identity/status/metadata: %+v", got)
	}
	if root.expandedReminder != originalID {
		t.Fatal("editing an expanded reminder unnecessarily collapsed it")
	}
}

func TestCancellingEditLeavesEntityUnchanged(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Read"})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 1, Item: 0}
	root = pressKey(t, root, "e")
	root = replaceFocusedText(t, root, "Read", "Changed")
	root = pressKey(t, root, "esc")

	if root.modal != nil || backend.habits[0].Title != "Read" || hasCall(backend.calls, "EditHabit") {
		t.Fatalf("cancelled edit mutated data: habit=%+v calls=%v", backend.habits[0], backend.calls)
	}
}

func TestDeleteHabitRequiresConfirmationAndClampsSelection(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(
		domain.Habit{Title: "One"}, domain.Habit{Title: "Two"},
	)
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 1, Item: 1}
	root = pressKey(t, root, "delete")

	if root.modal == nil || hasCall(backend.calls, "DeleteHabit") || len(backend.habits) != 2 {
		t.Fatal("Delete key must open confirmation without deleting immediately")
	}
	if !strings.Contains(stripANSI(root.modal.form.View()), `Delete habit "Two"?`) {
		t.Fatalf("delete confirmation does not identify the selected habit:\n%s", stripANSI(root.modal.form.View()))
	}
	root = confirmDeletion(t, root)
	if len(backend.habits) != 1 || backend.habits[0].Title != "One" || root.focus.Item != 0 {
		t.Fatalf("habit deletion did not preserve nearest selection: habits=%+v focus=%+v", backend.habits, root.focus)
	}
}

func TestDeleteFinalHabitMovesFocusToAddBlock(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Only"})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 1, Item: 0}
	root = pressKey(t, root, "delete")
	root = confirmDeletion(t, root)

	if len(root.snap.Today.Groups) != 0 || root.focus != (Focus{Column: ColToday, Block: TodayActions, Item: 0}) {
		t.Fatalf("deleting final habit left invalid focus: groups=%+v focus=%+v", root.snap.Today.Groups, root.focus)
	}
}

func TestCancellingDeleteIsNonMutating(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withReminders(domain.Reminder{Title: "Keep", Date: today})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}
	root = pressKey(t, root, "delete")
	root = pressKey(t, root, "n")

	if root.modal != nil || len(backend.reminders) != 1 || hasCall(backend.calls, "DeleteReminder") || root.pending {
		t.Fatalf("cancelled deletion mutated or left pending state: reminders=%+v calls=%v pending=%v", backend.reminders, backend.calls, root.pending)
	}
}

func TestDeleteConfirmationEnterKeepsDefaultCancel(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Keep"})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 1, Item: 0}
	root = pressKey(t, root, "delete")
	root = pressKey(t, root, "enter")

	if root.modal != nil || len(backend.habits) != 1 || hasCall(backend.calls, "DeleteHabit") || root.pending {
		t.Fatalf("Enter on default Cancel mutated or left pending state: habits=%+v calls=%v pending=%v", backend.habits, backend.calls, root.pending)
	}
}

func TestDeleteRoutineDetachesChildrenAndCleansCollapsedState(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newRichBackend(today)
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColToday, Block: 2, Item: 2}
	routineID := *root.snap.Today.Groups[1].RoutineID
	root = pressKey(t, root, "t")
	if !root.collapsedRoutines[routineID] {
		t.Fatal("test setup: routine was not collapsed")
	}
	root = pressKey(t, root, "delete")
	if !strings.Contains(stripANSI(root.modal.form.View()), "habits will remain") {
		t.Fatal("routine confirmation does not communicate child detachment")
	}
	root = confirmDeletion(t, root)

	if len(backend.routines) != 0 || root.collapsedRoutines[routineID] {
		t.Fatalf("routine deletion left routine or stale expansion state: routines=%+v collapsed=%v", backend.routines, root.collapsedRoutines)
	}
	for _, habit := range backend.habits {
		if habit.RoutineID != nil && *habit.RoutineID == routineID {
			t.Fatalf("routine deletion left child attached: %+v", habit)
		}
	}
	if len(root.snap.Today.Groups) != 1 || root.snap.Today.Groups[0].RoutineID != nil {
		t.Fatalf("detached habits did not remain as ungrouped habits: %+v", root.snap.Today.Groups)
	}
}

func TestDeleteReminderCleansExpansionAndHandlesFinalItem(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withReminders(
		domain.Reminder{Title: "One", Date: today},
		domain.Reminder{Title: "Two", Date: today.AddDays(1)},
	)
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 1}
	root = pressKey(t, root, "t")
	root = pressKey(t, root, "delete")
	root = confirmDeletion(t, root)
	if len(backend.reminders) != 1 || root.expandedReminder != 0 || root.focus.Item != 0 {
		t.Fatalf("reminder deletion left stale expansion/focus: reminders=%+v expanded=%d focus=%+v", backend.reminders, root.expandedReminder, root.focus)
	}

	root = pressKey(t, root, "delete")
	root = confirmDeletion(t, root)
	if len(backend.reminders) != 0 || root.focus.Block != RemindersList || root.focus.Item != 0 || root.editableSelection().valid() {
		t.Fatalf("deleting final reminder left invalid selection: reminders=%+v focus=%+v", backend.reminders, root.focus)
	}
}

func TestEditAndToggleKeysAreContextual(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.focus = Focus{Column: ColOverview, Block: OverviewLifetime, Item: 0}
	root = pressKey(t, root, "e")
	if root.modal != nil {
		t.Fatal("e should not edit a non-entity Overview block")
	}
	before := len(root.collapsedRoutines)
	root = pressKey(t, root, "t")
	if len(root.collapsedRoutines) != before {
		t.Fatal("t should not toggle an unrelated block")
	}
}
