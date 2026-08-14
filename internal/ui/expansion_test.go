package ui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

func TestRoutineExpandCollapseCorrectsFocusAndHidesChildren(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newRichBackend(today)
	root := newReadyRoot(t, backend)
	root = pressKey(t, root, "right")
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "tab") // Morning routine
	root = pressKey(t, root, "down")
	root = pressKey(t, root, "down")

	group := root.snap.Today.Groups[1]
	if group.RoutineID == nil || root.routineCollapsed(group) {
		t.Fatal("routine must exist and start expanded")
	}
	root = pressKey(t, root, "t")
	if !root.routineCollapsed(group) {
		t.Fatal("t did not collapse the focused routine")
	}
	if root.focus.Item != 0 || root.itemCount(ColToday, root.focus.Block) != 1 {
		t.Fatalf("collapsed focus = %+v count=%d, want header item 0 only", root.focus, root.itemCount(ColToday, root.focus.Block))
	}

	lines, focusLine := root.todayLines(30)
	visible := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(visible, "Morning") || strings.Contains(visible, "Drink water") {
		t.Fatalf("collapsed routine should show its summary but not children:\n%s", visible)
	}
	if focusLine < 0 || !strings.Contains(stripANSI(lines[focusLine]), "Morning") {
		t.Fatalf("collapsed routine focus is not anchored on its header: line=%d\n%s", focusLine, visible)
	}

	calls := len(backend.calls)
	root = pressKey(t, root, "enter")
	if len(backend.calls) != calls {
		t.Fatal("Enter on a collapsed routine header must not toggle a hidden habit")
	}
	root = pressKey(t, root, "t")
	if root.routineCollapsed(root.snap.Today.Groups[1]) || root.itemCount(ColToday, root.focus.Block) != 3 {
		t.Fatal("second t did not restore the routine's children")
	}
}

func TestExpandDoesNotCollapseUngroupedHabits(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root = pressKey(t, root, "right")
	root = pressKey(t, root, "tab") // ungrouped
	before := root.itemCount(ColToday, root.focus.Block)
	root = pressKey(t, root, "t")
	if got := root.itemCount(ColToday, root.focus.Block); got != before {
		t.Fatalf("t changed ungrouped item count from %d to %d", before, got)
	}
}

func TestFooterShowsExpandHintOnlyForStructuredSelection(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	footer := stripANSI(root.renderFooter())
	if strings.Contains(footer, "t expand") || strings.Contains(footer, "t collapse") {
		t.Fatal("Overview lifetime footer should not advertise an irrelevant expand action")
	}
	root.focus = Focus{Column: ColToday, Block: 2, Item: 0}
	if !strings.Contains(stripANSI(root.renderFooter()), "t collapse") {
		t.Fatal("expanded routine footer is missing the collapse hint")
	}
	root.toggleExpansion()
	if !strings.Contains(stripANSI(root.renderFooter()), "t expand") {
		t.Fatal("collapsed routine footer is missing the expand hint")
	}

	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}
	if !strings.Contains(stripANSI(root.renderFooter()), "t expand") {
		t.Fatal("collapsed reminder footer is missing the expand hint")
	}
	root.toggleExpansion()
	if !strings.Contains(stripANSI(root.renderFooter()), "t collapse") {
		t.Fatal("expanded reminder footer is missing the collapse hint")
	}
}

func TestFooterShowsEditToggleAndDeleteBindingsAtMinimumWidth(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.width = MinWidth
	root.focus = Focus{Column: ColToday, Block: 2, Item: 0}
	footer := stripANSI(root.renderFooter())
	for _, want := range []string{"enter act", "e edit", "t collapse", "Delete delete", "? help", "q quit"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("minimum-width footer is missing %q: %q", want, footer)
		}
	}
	if width := lipgloss.Width(root.renderFooter()); width > MinWidth {
		t.Fatalf("footer width = %d, want <= %d: %q", width, MinWidth, footer)
	}
}

func TestHelpDocumentsFinalEntityBindings(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	help := flattenWhitespace(stripANSI(root.renderHelp()))
	for _, want := range []string{
		"e edit selected habit, routine or reminder",
		"t expand / collapse routine or reminder",
		"Delete delete selected entity with confirmation",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help is missing %q: %q", want, help)
		}
	}
}

func TestReminderExpandCollapseShowsExistingMetadata(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withReminders(domain.Reminder{
		Title: "Submit the complete release candidate review",
		Date:  today,
		Notes: "first line\nsecond line with additional context",
	})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}

	compact, _, _ := root.reminderLines(25, root.sortedReminders(), true)
	if len(compact) != 1 {
		t.Fatalf("collapsed reminder uses %d lines, want one compact row", len(compact))
	}

	root = pressKey(t, root, "t")
	if root.expandedReminder != root.sortedReminders()[0].ID {
		t.Fatal("t did not expand the selected reminder")
	}
	lines, _, _ := root.reminderLines(25, root.sortedReminders(), true)
	rendered := stripANSI(strings.Join(lines, "\n"))
	flat := flattenWhitespace(rendered)
	for _, want := range []string{
		"Submit the complete release candidate review",
		"Mon, Aug 10, 2026",
		"Due today",
		"first line second line with additional context",
	} {
		if !strings.Contains(flat, want) {
			t.Fatalf("expanded reminder missing %q:\n%s", want, rendered)
		}
	}
	for i, line := range lines {
		if width := lipgloss.Width(line); width > 25 {
			t.Fatalf("expanded reminder row %d is %d cells wide (>25): %q", i, width, stripANSI(line))
		}
	}

	root = pressKey(t, root, "t")
	if root.expandedReminder != 0 {
		t.Fatal("second t did not collapse the selected reminder")
	}
}

func TestReminderExpansionHandlesMissingNotesAndCompletedStatus(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withReminders(domain.Reminder{
		Title: "Archived task", Date: today.AddDays(-1), Completed: true,
	})
	root := newReadyRoot(t, backend)
	root.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}
	root = pressKey(t, root, "t")

	lines, _, _ := root.reminderLines(27, root.sortedReminders(), true)
	rendered := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(rendered, "Completed") || !strings.Contains(rendered, "Notes   —") {
		t.Fatalf("completed reminder with no notes rendered poorly:\n%s", rendered)
	}
}
