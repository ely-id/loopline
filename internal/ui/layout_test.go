package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
	"loopline/internal/theme"
)

// sixWeekMonthToday is a date whose month renders as a full six calendar
// rows (Sunday-first): the worst case renderOverview must still fit inside
// MinHeight without being clipped. August 2026 starts on a Saturday and ends
// on a Monday, so its grid spans Jul 26 - Sep 5 (6 rows).
var sixWeekMonthToday = domain.MustParseDate("2026-08-10")

func TestFixedNerdFontIconsHaveStableSingleCellWidth(t *testing.T) {
	icons := []string{
		theme.IconLoop, theme.IconOverview, theme.IconHabits, theme.IconReminders,
		theme.IconLifetime, theme.IconWeekly, theme.IconCompletion, theme.IconYear, theme.IconCalendar,
	}
	for _, icon := range icons {
		if width := lipgloss.Width(icon); width != 1 {
			t.Fatalf("fixed heading icon %q has measured width %d, want 1", icon, width)
		}
	}
}

func TestLifetimeUsesExactLeafHeading(t *testing.T) {
	if theme.IconLifetime != "" {
		t.Fatalf("IconLifetime = %q, want exact U+F06C leaf glyph", theme.IconLifetime)
	}
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	first := strings.Split(stripANSI(root.renderLifetime(27)), "\n")[0]
	if first != " Lifetime" {
		t.Fatalf("lifetime heading = %q, want %q", first, " Lifetime")
	}
	if prefixWidth := lipgloss.Width(strings.TrimSuffix(first, "Lifetime")); prefixWidth != 2 {
		t.Fatalf("lifetime icon + spacing width = %d, want the fixed-heading two-cell prefix", prefixWidth)
	}
}

func TestCompactColumnWidthsPrioritizeTodayAndSumExactly(t *testing.T) {
	for _, width := range []int{MinWidth, 120, 160} {
		cols := columnWidths(width)
		if got := cols[0] + cols[1] + cols[2] + 2; got != width {
			t.Fatalf("columnWidths(%d) sums to %d including gaps", width, got)
		}
		if cols[1] <= cols[0] || cols[1] <= cols[2] {
			t.Fatalf("columnWidths(%d) = %v, want Today wider than both supporting columns", width, cols)
		}
	}
	if got := columnWidths(MinWidth); got != [3]int{31, 34, 29} {
		t.Fatalf("minimum column widths = %v, want [31 34 29]", got)
	}
}

func TestWideColumnWidthsKeepIdentityCompactAndHabitsNarrow(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	wantIdentity := lipgloss.Width(looplineASCII) + root.theme.Column.GetHorizontalFrameSize() + identityRailExtraWidth

	for _, width := range []int{wideLayoutMinWidth, 140, 200} {
		root.width = width
		cols := root.wideColumnWidths()
		if got := cols[0] + cols[1] + cols[2] + cols[3] + wideLayoutGaps; got != width {
			t.Fatalf("wideColumnWidths(%d) sums to %d including gaps", width, got)
		}
		if cols[0] != wantIdentity {
			t.Fatalf("wideColumnWidths(%d) identity width = %d, want configured width %d", width, cols[0], wantIdentity)
		}
		if cols[1] >= cols[2] || cols[1] >= cols[3] {
			t.Fatalf("wideColumnWidths(%d) = %v, want Habits narrower than Overview and Reminders", width, cols)
		}
	}

	root.width = wideLayoutMinWidth
	if got := root.wideColumnWidths(); got != [4]int{20, 28, 35, 34} {
		t.Fatalf("minimum wide widths = %v, want identity/Habits/Reminders/Overview [20 28 35 34]", got)
	}
	innerIdentity := root.wideColumnWidths()[0] - root.theme.Column.GetHorizontalFrameSize()
	navLines := strings.Split(stripANSI(root.renderIdentityNavigation(innerIdentity)), "\n")
	if len(navLines) != numColumns {
		t.Fatalf("identity navigation has %d rows, want %d", len(navLines), numColumns)
	}
	for i, want := range []string{"HABITS", "REMINDERS", "OVERVIEW"} {
		if !strings.Contains(navLines[i], want) || lipgloss.Width(navLines[i]) != innerIdentity {
			t.Fatalf("identity navigation row %d = %q, want padded vertical %s row at width %d", i, navLines[i], want, innerIdentity)
		}
	}
}

func TestWideLayoutBreakpointPreservesCompactThreePanelFallback(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	root.height = MinHeight

	root.width = wideLayoutMinWidth - 1
	if root.usesIdentityRail() {
		t.Fatal("compact width unexpectedly enabled the identity rail")
	}
	if got := strings.Count(stripANSI(root.View().Content), "└"); got != 3 {
		t.Fatalf("compact layout closed %d columns, want 3", got)
	}

	root.width = wideLayoutMinWidth
	if !root.usesIdentityRail() {
		t.Fatal("wide breakpoint did not enable the identity rail")
	}
	content := stripANSI(root.View().Content)
	if got := strings.Count(content, "└"); got != 4 {
		t.Fatalf("wide layout closed %d columns, want identity plus 3 content columns", got)
	}
	if strings.Count(content, "┓      ┓•") != 1 {
		t.Fatalf("wide layout did not render exactly one ASCII identity:\n%s", content)
	}
	for _, want := range []string{"Overview", "Habits", "Reminders", "OVERVIEW", "HABITS", "REMINDERS"} {
		if !strings.Contains(content, want) {
			t.Fatalf("wide layout is missing %q:\n%s", want, content)
		}
	}
	titleLine := strings.Split(content, "\n")[1]
	habitsAt := strings.Index(titleLine, "Habits")
	remindersAt := strings.Index(titleLine, "Reminders")
	overviewAt := strings.Index(titleLine, "Overview")
	if habitsAt < 0 || !(habitsAt < remindersAt && remindersAt < overviewAt) {
		t.Fatalf("wide content order is not Habits → Reminders → Overview: %q", titleLine)
	}
}

func TestCompactPanelsFollowGlobalVisualOrder(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	root.width, root.height = wideLayoutMinWidth-1, MinHeight
	content := strings.Split(stripANSI(root.renderColumns(MinHeight-2)), "\n")
	if len(content) < 2 {
		t.Fatalf("compact columns did not render an interior row: %q", content)
	}
	firstInterior := content[1]
	habitsAt := strings.Index(firstInterior, "Add")
	remindersAt := strings.Index(firstInterior, "+ Add reminder")
	overviewAt := strings.Index(firstInterior, "Lifetime")
	if habitsAt < 0 || !(habitsAt < remindersAt && remindersAt < overviewAt) {
		t.Fatalf("compact content order is not Habits → Reminders → Overview: %q", firstInterior)
	}
}

// TestOverviewContentFitsAtMinHeight pins the continuous Completion timeline
// against the real 29-row content budget at 96x33.
func TestOverviewContentFitsAtMinHeight(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	if weeks := len(root.snap.Calendar.Weeks); weeks != 6 {
		t.Fatalf("fixture drifted: calendar has %d weeks, want 6 (the worst case this test exists to cover)", weeks)
	}

	lines := strings.Split(root.renderOverview(27, MinHeight-4), "\n")

	budget := MinHeight - 4 // one header, one footer, two column-border rows
	if len(lines) > budget {
		t.Fatalf("renderOverview produced %d lines, want <= %d (MinHeight=%d budget); raise MinHeight to match", len(lines), budget, MinHeight)
	}
}

// TestViewAtMinHeightRendersCompletionAndCalendarUnclipped is the end-to-end
// 96x33 guard for the timeline and conventional calendar.
func TestViewAtMinHeightRendersCompletionAndCalendarUnclipped(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	root = send(t, root, tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})

	content := root.View().Content
	if !strings.Contains(content, "Completion") || !strings.Contains(content, "Calendar") {
		t.Fatalf("View at MinWidth x MinHeight is missing a calendar block title:\n%s", content)
	}
}

// TestTodayLongListKeepsFocusedItemVisibleAndProgressPinned exercises the
// windowing renderToday/windowAround do to keep a long habit list from
// pushing the day's progress bar off the bottom of a fixed-height column,
// and to keep the focused habit on screen however far down the list it is.
func TestTodayLongListKeepsFocusedItemVisibleAndProgressPinned(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	habits := make([]domain.Habit, 30)
	for i := range habits {
		habits[i] = domain.Habit{Title: fmt.Sprintf("Habit %02d", i)}
	}
	backend.withHabits(habits...)

	root := newReadyRoot(t, backend)
	root = send(t, root, tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
	root = enterPanel(t, root, ColToday)
	root = pressKey(t, root, "down") // Add Habit -> Add Routine
	root = pressKey(t, root, "down") // Add Routine -> ungrouped habits

	for i := 0; i < len(habits)-1; i++ {
		root = pressKey(t, root, "down")
	}
	if root.focus.Item != len(habits)-1 {
		t.Fatalf("setup: focus.Item = %d, want %d", root.focus.Item, len(habits)-1)
	}

	content := root.View().Content
	lastTitle := habits[len(habits)-1].Title
	if !strings.Contains(content, lastTitle) {
		t.Fatalf("focused last habit %q not visible in the rendered view at height %d:\n%s", lastTitle, MinHeight, content)
	}

	wantProgress := fmt.Sprintf("0/%d", len(habits))
	if !strings.Contains(content, wantProgress) {
		t.Fatalf("day progress %q not present (pinned to the bottom of the column) in:\n%s", wantProgress, content)
	}
}
