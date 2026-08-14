package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

func TestWeeklyOverviewWindowsRowsAndKeepsCompletionAnchored(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	habits := make([]domain.Habit, 30)
	for i := range habits {
		habits[i] = domain.Habit{Title: fmt.Sprintf("Weekly habit %02d", i+1)}
	}
	backend.withHabits(habits...)
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewWeekly, Item: len(habits) - 1})

	const width, height = 27, MinHeight - 4
	rendered := stripANSI(root.renderOverview(width, height))
	lines := strings.Split(rendered, "\n")
	if len(lines) != height {
		t.Fatalf("Overview height = %d, want exact budget %d", len(lines), height)
	}
	if !strings.Contains(rendered, habits[len(habits)-1].Title[:12]) {
		t.Fatalf("selected final Weekly row is not visible:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Weekly Overview") || !strings.Contains(rendered, "↕") {
		t.Fatalf("overflowing Weekly viewport lacks title/scroll affordance:\n%s", rendered)
	}
	completionLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Completion") {
			completionLine = i
		}
		if lipgloss.Width(line) > width {
			t.Fatalf("Weekly Overview row %d exceeds width %d: %q", i, width, line)
		}
	}
	wantCompletionLine := height - lipgloss.Height(root.renderCompletionTimeline(width))
	if completionLine != wantCompletionLine {
		t.Fatalf("Completion begins on row %d, want bottom-anchored row %d", completionLine, wantCompletionLine)
	}
}

func TestWeeklyOverviewUsesOnlyScheduledAndCompletedStates(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Read"})
	habitID := backend.habits[0].ID
	backend.done[habitID] = map[domain.Date]bool{today: true}
	root := newReadyRoot(t, backend)
	rendered := stripANSI(root.renderWeeklyOverview(30, 6))
	if !strings.Contains(rendered, "✓") || !strings.Contains(rendered, "·") {
		t.Fatalf("Weekly row must distinguish completed and scheduled-incomplete cells:\n%s", rendered)
	}
	if strings.Contains(strings.ToLower(rendered), "partial") {
		t.Fatalf("Weekly Overview invented a partial state:\n%s", rendered)
	}
}

func TestWeeklyOverviewHeaderStartsOnSunday(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-12")))
	if root.snap.Weekly.Start.Weekday() != time.Sunday || root.snap.Weekly.End.Weekday() != time.Saturday {
		t.Fatalf("Weekly range = %s..%s, want Sunday through Saturday", root.snap.Weekly.Start, root.snap.Weekly.End)
	}
	header := stripANSI(root.renderWeeklyHeader(35))
	if !strings.Contains(header, "Su Mo Tu We Th Fr Sa") {
		t.Fatalf("Weekly header is not Sunday-first: %q", header)
	}
}

func TestSwitchingPanelsAfterWeeklyScrollLeavesValidVisibleSelection(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	for i := 0; i < 20; i++ {
		backend.withHabits(domain.Habit{Title: fmt.Sprintf("Habit %02d", i)})
	}
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewWeekly, Item: 19})
	_ = root.View().Content
	root = pressKey(t, root, "esc")
	root = pressKey(t, root, "tab")
	root = pressKey(t, root, "enter")
	if root.navigation != NavigationPanel || root.focus.Column != ColToday {
		t.Fatalf("switch after Weekly scroll produced invalid state: mode=%d focus=%+v", root.navigation, root.focus)
	}
	if content := root.View().Content; !strings.Contains(content, "Habit 00") {
		t.Fatalf("newly selected Habits panel did not render its selected region:\n%s", stripANSI(content))
	}
}
