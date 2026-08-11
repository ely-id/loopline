package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"routine-tui/internal/domain"
)

// longTitledHabitsBackend seeds count habits whose titles are long enough
// (~52 chars, matching the release-blocker report) that every one of them
// wraps to multiple physical lines at a 96-wide terminal's column width, so
// windowAround must be windowing on physical rows rather than logical
// habits for the height budget to hold.
func longTitledHabitsBackend(today domain.Date, count int) *fakeBackend {
	b := newFakeBackend(today)
	habits := make([]domain.Habit, count)
	for i := range habits {
		habits[i] = domain.Habit{Title: fmt.Sprintf("Habit %02d with a fairly descriptive long title here", i+1)}
	}
	b.withHabits(habits...)
	return b
}

// longTitledRemindersBackend is longTitledHabitsBackend's reminders
// equivalent.
func longTitledRemindersBackend(today domain.Date, count int) *fakeBackend {
	b := newFakeBackend(today)
	reminders := make([]domain.Reminder, count)
	for i := range reminders {
		reminders[i] = domain.Reminder{
			Title: fmt.Sprintf("Reminder %02d with a fairly descriptive long title here", i+1),
			Date:  today.AddDays(i),
		}
	}
	b.withReminders(reminders...)
	return b
}

// TestLongHabitTitlesNeverOverflowColumnBudget is the regression test for
// the release-blocking height-budgeting bug: todayLines used to count one
// *logical* habit as one line, but styledColumn word-wraps (and then
// height-clips) any line that doesn't fit the column's real text width. With
// enough long-titled habits, that pushed the day's progress bar out of the
// rendered frame entirely and clipped the column's own bottom border. At any
// accepted terminal size, the full rendered view must never exceed the
// terminal's own dimensions, and the progress bar must always be present.
func TestLongHabitTitlesNeverOverflowColumnBudget(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	sizes := []struct{ w, h int }{
		{96, 36},
		{MinWidth, MinHeight},
	}
	for _, sz := range sizes {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			backend := longTitledHabitsBackend(today, 15)
			m := newReadyRoot(t, backend)
			m = send(t, m, tea.WindowSizeMsg{Width: sz.w, Height: sz.h})

			content := m.View().Content
			lines := strings.Split(content, "\n")
			if len(lines) > sz.h {
				t.Fatalf("rendered view is %d lines tall, want <= %d (terminal height); the Today column's habit list must be windowed on physical, not logical, rows:\n%s", len(lines), sz.h, content)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > sz.w {
					t.Fatalf("line %d is %d cells wide (> terminal width %d): %q", i, w, sz.w, line)
				}
			}

			sum := m.snap.Today.Summary
			wantProgress := fmt.Sprintf("%d/%d", sum.Completed, sum.Scheduled)
			if !strings.Contains(stripANSI(content), wantProgress) {
				t.Fatalf("day progress bar %q is missing from the rendered view (pushed off the bottom of the Today column by unwindowed wrapped habit lines):\n%s", wantProgress, stripANSI(content))
			}
		})
	}
}

// TestLongReminderTitlesNeverOverflowColumnBudget is
// TestLongHabitTitlesNeverOverflowColumnBudget's counterpart for the
// Reminders column.
func TestLongReminderTitlesNeverOverflowColumnBudget(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	sizes := []struct{ w, h int }{
		{96, 36},
		{MinWidth, MinHeight},
	}
	for _, sz := range sizes {
		t.Run(fmt.Sprintf("%dx%d", sz.w, sz.h), func(t *testing.T) {
			backend := longTitledRemindersBackend(today, 15)
			m := newReadyRoot(t, backend)
			m = send(t, m, tea.WindowSizeMsg{Width: sz.w, Height: sz.h})

			content := m.View().Content
			lines := strings.Split(content, "\n")
			if len(lines) > sz.h {
				t.Fatalf("rendered view is %d lines tall, want <= %d (terminal height); the Reminders column's list must be windowed on physical, not logical, rows:\n%s", len(lines), sz.h, content)
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > sz.w {
					t.Fatalf("line %d is %d cells wide (> terminal width %d): %q", i, w, sz.w, line)
				}
			}
		})
	}
}

// TestTodayProgressBarIsLastNonBlankLine is the regression test for FIX 2:
// SPEC Column 2 requires the daily completion progress bar to sit "At the
// bottom" of the column. With a short habit list, renderToday must pad the
// list body out to its full height budget so the progress bar is pinned to
// the bottom of the column instead of trailing right beneath the last habit
// with a gap under it.
func TestTodayProgressBarIsLastNonBlankLine(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(
		domain.Habit{Title: "Drink water"},
		domain.Habit{Title: "Stretch"},
	)
	root := newReadyRoot(t, backend)

	const width, height = 60, 30 // a tall column budget relative to 2 habits
	content := stripANSI(root.renderToday(width, height))
	lines := strings.Split(content, "\n")
	if len(lines) != height {
		t.Fatalf("renderToday produced %d lines, want exactly %d (the height budget it was given)", len(lines), height)
	}

	lastNonBlank := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			lastNonBlank = i
		}
	}
	if lastNonBlank == -1 {
		t.Fatal("renderToday produced no non-blank lines at all")
	}

	wantProgress := strings.TrimRight(stripANSI(root.renderDayProgress(width, root.snap.Today.Summary)), " ")
	got := strings.TrimRight(lines[lastNonBlank], " ")
	if got != wantProgress {
		t.Fatalf("last non-blank line of the Today column = %q, want the progress bar %q (progress bar must be pinned to the bottom of the column, not trailing under a short habit list)", got, wantProgress)
	}
}

// TestOverviewStatsFitsBudgetAtMinColumnWidth is the regression test for
// FIX 3: at the overview column's real text width (27 cells at MinWidth=96),
// renderOverviewStats must always occupy exactly two physical lines,
// whatever the underlying values are, so a wide stats line never pushes the
// heatmap block past MinHeight's fixed 31-line budget.
func TestOverviewStatsFitsBudgetAtMinColumnWidth(t *testing.T) {
	const overviewTextWidthAtMinWidth = 27

	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newFakeBackend(today))
	root.snap.Overview.ActiveHabits = 15
	root.snap.Overview.ActiveRoutines = 0
	root.snap.Overview.OpenReminders = 123
	root.snap.Overview.CurrentStreak = 999
	root.snap.Overview.BestStreak = 999
	root.snap.Overview.LifetimePercent = 100

	stats := stripANSI(root.renderOverviewStats(overviewTextWidthAtMinWidth))
	lines := strings.Split(stats, "\n")
	if len(lines) != 2 {
		t.Fatalf("renderOverviewStats produced %d physical lines at width %d, want exactly 2: %q", len(lines), overviewTextWidthAtMinWidth, stats)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > overviewTextWidthAtMinWidth {
			t.Fatalf("stats line %d is %d cells wide (> width %d): %q", i, w, overviewTextWidthAtMinWidth, l)
		}
	}

	// Fitting the budget by truncation alone is not enough: MaxWidth cuts
	// from the right, so labels must be short enough that the numbers they
	// label survive at the minimum width. Otherwise the column renders
	// "…Best 999  Lifetime" — a label with its value clipped off, which
	// reads as a rendering bug.
	for _, want := range []string{"999", "100%", "15", "123"} {
		if !strings.Contains(stats, want) {
			t.Fatalf("stat value %q was truncated away at width %d: %q", want, overviewTextWidthAtMinWidth, stats)
		}
	}

	// End-to-end: the same six-week-month worst case
	// TestOverviewContentFitsAtMinHeight pins, but now with stat values wide
	// enough to wrap at width 27 if renderOverviewStats didn't truncate
	// instead.
	root2 := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	root2.snap.Overview.ActiveHabits = 15
	root2.snap.Overview.OpenReminders = 123
	root2.snap.Overview.LifetimePercent = 100

	overviewLines := strings.Split(root2.renderOverview(overviewTextWidthAtMinWidth), "\n")
	budget := MinHeight - 4 // bodyHeight (-2 footer) then column border (-2)
	if len(overviewLines) > budget {
		t.Fatalf("renderOverview produced %d lines at width %d, want <= %d (MinHeight=%d budget)", len(overviewLines), overviewTextWidthAtMinWidth, budget, MinHeight)
	}
}
