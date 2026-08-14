package ui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

func TestCompletionStageUsesSharedSemanticBoundaries(t *testing.T) {
	cases := []struct {
		completed, scheduled int
		want                 theme.CompletionStage
	}{
		{0, 0, theme.CompletionStageNone},
		{0, 4, theme.CompletionStageZero},
		{1, 4, theme.CompletionStageLow},
		{33, 100, theme.CompletionStageLow},
		{34, 100, theme.CompletionStageMedium},
		{66, 100, theme.CompletionStageMedium},
		{67, 100, theme.CompletionStageHigh},
		{199, 200, theme.CompletionStageHigh}, // rounded 100% is not complete
		{4, 4, theme.CompletionStageComplete},
	}
	for _, tc := range cases {
		sum := domain.DailySummary{Completed: tc.completed, Scheduled: tc.scheduled}
		if got := completionStage(sum); got != tc.want {
			t.Errorf("completionStage(%d/%d=%d%%) = %d, want %d", tc.completed, tc.scheduled, sum.Percent(), got, tc.want)
		}
	}
}

func TestCompletionStagesUseDistinctSemanticANSIColors(t *testing.T) {
	styles := theme.New().CompletionStyles
	for stage := theme.CompletionStageNone; stage < theme.CompletionStageCount; stage++ {
		for other := stage + 1; other < theme.CompletionStageCount; other++ {
			if styles[stage].GetForeground() == styles[other].GetForeground() {
				t.Fatalf("completion stages %d and %d share foreground %v", stage, other, styles[stage].GetForeground())
			}
		}
	}
}

func TestCompletionCalendarDistinguishesNoneMissedAndFuture(t *testing.T) {
	root := rootWithGroups()
	date := domain.MustParseDate("2026-08-10")

	none := stripANSI(root.completionDayCell(app.CalendarDay{Date: date, InMonth: true}))
	missed := stripANSI(root.completionDayCell(app.CalendarDay{
		Date: date, InMonth: true,
		Summary: domain.DailySummary{Date: date, Scheduled: 1},
	}))
	future := stripANSI(root.completionDayCell(app.CalendarDay{
		Date: date, InMonth: true, IsFuture: true,
		Summary: domain.DailySummary{Date: date, Scheduled: 1, Completed: 1},
	}))

	if !strings.HasSuffix(none, string(theme.CompletionGlyphs[theme.CompletionStageNone])) {
		t.Fatalf("no-schedule cell = %q, want neutral suffix", none)
	}
	if !strings.HasSuffix(missed, string(theme.CompletionGlyphs[theme.CompletionStageZero])) {
		t.Fatalf("missed scheduled cell = %q, want zero-stage suffix", missed)
	}
	if !strings.HasSuffix(future, string(theme.CompletionGlyphs[theme.CompletionStageNone])) {
		t.Fatalf("future cell = %q, want neutral suffix rather than completed intensity", future)
	}
}

func TestCompletionCalendarTodayMarkerIsIndependentOfStageColor(t *testing.T) {
	root := rootWithGroups()
	day := app.CalendarDay{
		Date:    domain.MustParseDate("2026-08-10"),
		InMonth: true,
		IsToday: true,
		Summary: domain.DailySummary{Scheduled: 4, Completed: 2},
	}
	stage := completionStage(day.Summary)
	style := root.completionDayNumberStyle(day, stage)
	if !style.GetUnderline() || !style.GetBold() {
		t.Fatal("today must retain a non-color marker on the completion calendar")
	}
	if style.GetForeground() != root.theme.CompletionStyles[stage].GetForeground() {
		t.Fatal("today marker replaced, rather than preserved, the completion-stage color")
	}
}

func TestCompletionCalendarHasNoPermanentLegend(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	rendered := stripANSI(root.renderCompletionCalendar(27))
	for _, unwanted := range []string{"none", "progress", "░▒▓▆█"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("completion calendar retained legend text %q:\n%s", unwanted, rendered)
		}
	}
}

func TestTodayProgressUsesSameCompletionStageStyle(t *testing.T) {
	root := rootWithGroups()
	zero := root.renderDayProgress(30, domain.DailySummary{Scheduled: 4})
	complete := root.renderDayProgress(30, domain.DailySummary{Scheduled: 4, Completed: 4})

	zeroColor := root.theme.CompletionStyles[theme.CompletionStageZero].GetForeground()
	completeColor := root.theme.CompletionStyles[theme.CompletionStageComplete].GetForeground()
	if root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageZero]).GetForeground() != zeroColor {
		t.Fatal("zero-progress label does not inherit the shared zero-stage color")
	}
	if root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageComplete]).GetForeground() != completeColor {
		t.Fatal("complete-progress label does not inherit the shared complete-stage color")
	}
	if zero == complete {
		t.Fatal("0% and 100% Today progress must not render identically")
	}
	zeroLabel := root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageZero]).Render("0/4  0%")
	completeLabel := root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageComplete]).Render("4/4  100%")
	if !strings.Contains(zero, zeroLabel) || !strings.Contains(complete, completeLabel) {
		t.Fatal("Today progress output does not render its label with the shared stage style")
	}
}

func TestCompletionCalendarRowsFitMinimumOverviewWidth(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	rendered := stripANSI(root.renderCompletionCalendar(27))
	for i, line := range strings.Split(rendered, "\n") {
		if width := lipgloss.Width(line); width > 27 {
			t.Fatalf("completion calendar row %d is %d cells wide, want <= 27: %q", i, width, line)
		}
	}
}

func TestReminderCalendarStylePreservesTodayAndReminderDates(t *testing.T) {
	root := rootWithGroups()
	date := domain.MustParseDate("2026-08-10")

	todayStyle := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, IsToday: true})
	if !todayStyle.GetBold() {
		t.Fatal("today must remain visibly emphasized in the conventional calendar")
	}
	reminderStyle := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, HasReminder: true})
	if !reminderStyle.GetUnderline() {
		t.Fatal("reminder date must remain visibly marked in the conventional calendar")
	}
	both := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, IsToday: true, HasReminder: true})
	if !both.GetBold() || !both.GetUnderline() {
		t.Fatal("a date that is both today and reminder-related must preserve both representations")
	}
}

func TestReminderCalendarMutesAdjacentMonthsWithTodayPrecedence(t *testing.T) {
	root := rootWithGroups()
	date := domain.MustParseDate("2026-08-10")

	adjacent := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: false})
	if !adjacent.GetFaint() || adjacent.GetForeground() != root.theme.FgDim {
		t.Fatalf("adjacent-month style is not subordinate: faint=%v foreground=%v", adjacent.GetFaint(), adjacent.GetForeground())
	}
	active := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true})
	if active.GetFaint() {
		t.Fatal("active-month day was incorrectly muted")
	}
	adjacentReminder := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: false, HasReminder: true})
	if !adjacentReminder.GetFaint() || !adjacentReminder.GetUnderline() || adjacentReminder.GetForeground() != root.theme.Warning {
		t.Fatal("adjacent reminder should remain marked while retaining muted intensity")
	}
	adjacentToday := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: false, IsToday: true, HasReminder: true})
	if adjacentToday.GetFaint() || !adjacentToday.GetBold() || !adjacentToday.GetUnderline() || adjacentToday.GetForeground() != root.theme.Accent {
		t.Fatal("current-day styling did not take precedence over adjacent-month/reminder color")
	}
}

func TestCalendarGridRowsUseResponsiveSharedSpacingAndCentering(t *testing.T) {
	cases := []struct {
		name      string
		width     int
		cellWidth int
		wantGap   int
	}{
		{"completion minimum", 27, 3, 1},
		{"completion roomy", 33, 3, 2},
		{"conventional minimum", 25, 2, 1},
		{"conventional roomy", 29, 2, 2},
		{"conventional wide", 39, 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := []string{"A", "B", "C", "D", "E", "F", "G"}
			row := calendarGridRow(tc.width, tc.cellWidth, cells)
			if got := lipgloss.Width(row); got > tc.width {
				t.Fatalf("row width = %d, available = %d: %q", got, tc.width, row)
			}
			positions := make([]int, len(cells))
			for i, cell := range cells {
				positions[i] = strings.Index(row, cell)
				if positions[i] < 0 {
					t.Fatalf("cell %q missing from %q", cell, row)
				}
			}
			for i := 1; i < len(positions); i++ {
				if got := positions[i] - positions[i-1]; got != tc.cellWidth+tc.wantGap {
					t.Fatalf("cell stride = %d, want %d (cell=%d gap=%d): %q", got, tc.cellWidth+tc.wantGap, tc.cellWidth, tc.wantGap, row)
				}
			}
			gridWidth := tc.cellWidth*7 + tc.wantGap*6
			left := (tc.width - gridWidth) / 2
			right := tc.width - gridWidth - left
			if delta := left - right; delta < -1 || delta > 1 {
				t.Fatalf("grid is not centered: left=%d right=%d row=%q", left, right, row)
			}
		})
	}
}

func TestBothCalendarRenderersFitRepresentativeWidths(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	for _, width := range []int{27, 33, 41} {
		for name, rendered := range map[string]string{
			"completion":   root.renderCompletionCalendar(width),
			"conventional": root.renderReminderCalendar(width),
		} {
			for row, line := range strings.Split(rendered, "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("%s calendar width %d row %d = %d cells: %q", name, width, row, got, stripANSI(line))
				}
			}
		}
	}
}
