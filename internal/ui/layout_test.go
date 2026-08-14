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
// rows (Monday-first): the worst case renderOverview must still fit inside
// MinHeight without being clipped. August 2026 starts on a Saturday and ends
// on a Monday, so its grid spans Jul 27 - Sep 6 (6 rows).
var sixWeekMonthToday = domain.MustParseDate("2026-08-10")

func TestFixedNerdFontIconsHaveStableSingleCellWidth(t *testing.T) {
	icons := []string{
		theme.IconLoop, theme.IconToday, theme.IconReminders,
		theme.IconLifetime, theme.IconCompletion, theme.IconYear, theme.IconCalendar,
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

func TestColumnWidthsPrioritizeTodayAndSumExactly(t *testing.T) {
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

// TestOverviewContentFitsAtMinHeight pins the redesigned six-week completion
// calendar against the real 30-row content budget at 96x33.
func TestOverviewContentFitsAtMinHeight(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	if weeks := len(root.snap.Calendar.Weeks); weeks != 6 {
		t.Fatalf("fixture drifted: calendar has %d weeks, want 6 (the worst case this test exists to cover)", weeks)
	}

	lines := strings.Split(root.renderOverview(27), "\n")

	budget := MinHeight - 3 // one footer row + two column-border rows
	if len(lines) > budget {
		t.Fatalf("renderOverview produced %d lines, want <= %d (MinHeight=%d budget); raise MinHeight to match", len(lines), budget, MinHeight)
	}
}

// TestViewAtMinHeightRendersBothCalendarsUnclipped is the end-to-end 96x33
// guard for the two new month views.
func TestViewAtMinHeightRendersBothCalendarsUnclipped(t *testing.T) {
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
	root = pressKey(t, root, "right") // overview -> today
	root = pressKey(t, root, "tab")   // Add actions -> ungrouped habits

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
