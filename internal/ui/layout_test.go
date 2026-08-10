package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/domain"
)

// sixWeekMonthToday is a date whose month renders as a full six calendar
// rows (Monday-first): the worst case renderOverview must still fit inside
// MinHeight without being clipped. August 2026 starts on a Saturday and ends
// on a Monday, so its grid spans Jul 27 - Sep 6 (6 rows).
var sixWeekMonthToday = domain.MustParseDate("2026-08-10")

// TestOverviewContentFitsAtMinHeight pins down the height budget MinHeight's
// doc comment claims: the overview column has no viewport of its own, so if
// its worst-case content (a six-week month) ever exceeds what MinHeight
// affords, styledColumn's MaxHeight silently crops the bottom of the column
// (typically the heatmap) instead of the app falling back to the
// minimum-size message SPEC requires. If this test starts failing, raise
// MinHeight to match — don't shrink the budget check.
func TestOverviewContentFitsAtMinHeight(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	if weeks := len(root.snap.Calendar.Weeks); weeks != 6 {
		t.Fatalf("fixture drifted: calendar has %d weeks, want 6 (the worst case this test exists to cover)", weeks)
	}

	lines := strings.Split(root.renderOverview(40), "\n")

	budget := MinHeight - 4 // bodyHeight (-2 footer) then column border (-2)
	if len(lines) > budget {
		t.Fatalf("renderOverview produced %d lines, want <= %d (MinHeight=%d budget); raise MinHeight to match", len(lines), budget, MinHeight)
	}
}

// TestViewAtMinHeightRendersHeatmapUnclipped is the integration-level
// counterpart of TestOverviewContentFitsAtMinHeight: at exactly MinWidth x
// MinHeight, the heatmap block (the last, and tallest, block in the overview
// column) must still be visible in the rendered frame rather than clipped
// off the bottom.
func TestViewAtMinHeightRendersHeatmapUnclipped(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	root = send(t, root, tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})

	content := root.View().Content
	if !strings.Contains(content, "Heatmap") {
		t.Fatalf("View at MinWidth x MinHeight is missing the Heatmap block title:\n%s", content)
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
