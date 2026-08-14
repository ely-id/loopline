package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
		{199, 200, theme.CompletionStageHigh},
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

func TestCompletionCellDatePreservesWeekdayWeekMonthYearAndLeapTransitions(t *testing.T) {
	start := domain.MustParseDate("2024-02-25") // Sunday before leap day.
	cases := []struct {
		week, row int
		want      string
	}{
		{0, int(time.Sunday), "2024-02-25"},
		{0, int(time.Thursday), "2024-02-29"},
		{0, int(time.Friday), "2024-03-01"},
		{0, int(time.Saturday), "2024-03-02"},
		{1, int(time.Sunday), "2024-03-03"},
	}
	for _, tc := range cases {
		if got := completionCellDate(start, tc.week, tc.row); got.String() != tc.want {
			t.Errorf("cell week=%d row=%d = %s, want %s", tc.week, tc.row, got, tc.want)
		}
	}

	yearStart := domain.MustParseDate("2026-12-27")
	if got := completionCellDate(yearStart, 0, int(time.Thursday)); got.String() != "2026-12-31" {
		t.Fatalf("year-end Thursday = %s, want 2026-12-31", got)
	}
	if got := completionCellDate(yearStart, 0, int(time.Friday)); got.String() != "2027-01-01" {
		t.Fatalf("year-start Friday = %s, want 2027-01-01 in the same week", got)
	}
}

func TestCompletionTimelinePartialCurrentWeekHasTodayAndNoFutureCells(t *testing.T) {
	today := domain.MustParseDate("2026-08-12") // Wednesday.
	backend := newFakeBackend(today).withHabits(domain.Habit{
		Title:     "Daily",
		CreatedOn: domain.MustParseDate("2026-08-09"),
	})
	root := newReadyRoot(t, backend)
	lines := strings.Split(stripANSI(root.renderCompletionTimeline(27)), "\n")
	if len(lines) != 9 {
		t.Fatalf("Completion timeline height = %d, want title + month labels + seven weekdays", len(lines))
	}
	capacity := completionGeometry(27).Capacity
	for row := 0; row < 7; row++ {
		want := capacity - 1
		if row <= int(time.Wednesday) {
			want = capacity
		}
		if got := strings.Count(lines[2+row], theme.HeatmapSquare); got != want {
			t.Errorf("weekday row %d has %d cells, want %d occurred dates: %q", row, got, want, lines[2+row])
		}
	}
	wantCells := (capacity-1)*7 + int(time.Wednesday) + 1
	if got := strings.Count(strings.Join(lines[2:], "\n"), theme.HeatmapSquare); got != wantCells {
		t.Fatalf("partial current week contains %d day cells, want %d historical/current dates", got, wantCells)
	}
	if style := root.completionDayStyle(today); !style.GetBold() || !style.GetUnderline() {
		t.Fatal("today lacks the non-color bold/underline marker")
	}
}

func TestCompletionVisibleWindowIsPresentAnchoredAndResponsive(t *testing.T) {
	start := domain.MustParseDate("2025-01-01")
	today := domain.MustParseDate("2026-08-12")

	narrow := completionVisibleWindow(start, today, domain.Date{}, 27)
	if narrow.Capacity != 12 || narrow.Count != 12 {
		t.Fatalf("27-cell window = capacity %d count %d, want 12 visible weeks", narrow.Capacity, narrow.Count)
	}
	if !narrow.End.Equal(completionSunday(today)) {
		t.Fatalf("default window ends %s, want current week %s", narrow.End, completionSunday(today))
	}
	if got := narrow.End.DaysSince(narrow.Start); got != 11*7 {
		t.Fatalf("visible range spans %d days, want 11 week intervals", got)
	}

	wide := completionVisibleWindow(start, today, domain.Date{}, 41)
	if wide.Capacity <= narrow.Capacity || wide.Count != wide.Capacity {
		t.Fatalf("responsive width did not increase visible weeks: narrow=%+v wide=%+v", narrow, wide)
	}

	short := completionVisibleWindow(today.AddDays(-2), today, domain.Date{}, 27)
	if short.Count != short.Capacity || short.Leading != 0 {
		t.Fatalf("short completion history did not retain the full calendar viewport: %+v", short)
	}
	if got := short.End.DaysSince(short.Start); got != (short.Capacity-1)*7 {
		t.Fatalf("short-history calendar range spans %d days, want %d", got, (short.Capacity-1)*7)
	}
}

func TestCompletionHistoricalNoDataDatesRenderAsMutedCalendarCells(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	root := newReadyRoot(t, newFakeBackend(today))
	geometry := completionGeometry(27)
	window := completionVisibleWindow(root.snap.Completion.Start, today, domain.Date{}, 27)
	historical := completionCellDate(window.Start, 0, int(time.Sunday))
	if !historical.Before(root.snap.Completion.Start) {
		t.Fatalf("test date %s is not before completion data start %s", historical, root.snap.Completion.Start)
	}
	row := stripANSI(root.renderCompletionWeekday(window, geometry, int(time.Sunday), "Su"))
	if got := strings.Count(row, theme.HeatmapSquare); got != geometry.Capacity {
		t.Fatalf("historical Sunday row has %d cells, want %d including no-data dates", got, geometry.Capacity)
	}
	want := root.theme.CompletionStyles[theme.CompletionStageNone].GetForeground()
	if got := root.completionDayStyle(historical).GetForeground(); got != want {
		t.Fatalf("no-data historical date foreground = %v, want muted stage %v", got, want)
	}
}

func TestCompletionHistoricalScrollingUsesWeekUnitsAndClampsBothBoundaries(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	backend := newFakeBackend(today).withHabits(domain.Habit{
		Title:     "Long history",
		CreatedOn: today.AddDays(-400),
	})
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})

	root = pressKey(t, root, "left")
	if root.completionEndWeek.IsZero() {
		t.Fatal("left did not leave present anchoring")
	}
	firstEnd := root.completionEndWeek
	root = pressKey(t, root, "left")
	if got := firstEnd.DaysSince(root.completionEndWeek); got != 7 {
		t.Fatalf("left moved %d days, want exactly one week", got)
	}
	root = pressKey(t, root, "right")
	if !root.completionEndWeek.Equal(firstEnd) {
		t.Fatalf("right did not move exactly one week toward present: got %s want %s", root.completionEndWeek, firstEnd)
	}

	for range 200 {
		root = pressKey(t, root, "left")
	}
	oldest := root.completionEndWeek
	root = pressKey(t, root, "left")
	if !root.completionEndWeek.Equal(oldest) {
		t.Fatalf("left boundary moved from %s to %s", oldest, root.completionEndWeek)
	}
	window := completionVisibleWindow(root.snap.Completion.Start, root.today, root.completionEndWeek, root.completionTimelineWidth())
	wantOldestEnd := window.Earliest.AddDays((window.Count - 1) * 7)
	if !oldest.Equal(wantOldestEnd) {
		t.Fatalf("oldest end = %s, want %s", oldest, wantOldestEnd)
	}

	for range 200 {
		root = pressKey(t, root, "right")
	}
	if !root.completionEndWeek.IsZero() {
		t.Fatalf("right boundary did not restore present anchoring: %s", root.completionEndWeek)
	}
	root = pressKey(t, root, "right")
	if !root.completionEndWeek.IsZero() {
		t.Fatal("right moved beyond the current week")
	}
}

func TestCompletionHistoryAliasesAndWheelReachBeforeApril(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	backend := newFakeBackend(today).withHabits(domain.Habit{
		Title:     "Long history",
		CreatedOn: domain.MustParseDate("2026-01-01"),
	})
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})

	root = pressKey(t, root, "<")
	firstEnd := root.completionEndWeek
	if firstEnd.IsZero() {
		t.Fatal("< did not leave present anchoring")
	}
	root = pressKey(t, root, "[")
	if got := firstEnd.DaysSince(root.completionEndWeek); got != 7 {
		t.Fatalf("[ moved %d days, want one week older", got)
	}

	beforeWheel := root.completionEndWeek
	root = send(t, root, tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if got := beforeWheel.DaysSince(root.completionEndWeek); got != 7 {
		t.Fatalf("wheel up moved %d days, want one week older", got)
	}

	root = pressKey(t, root, ">")
	root = pressKey(t, root, "]")
	root = send(t, root, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if !root.completionEndWeek.IsZero() {
		t.Fatalf("newer aliases did not restore present anchoring: %s", root.completionEndWeek)
	}

	for range 20 {
		root = pressKey(t, root, "<")
	}
	window := completionVisibleWindow(
		root.snap.Completion.Start,
		root.today,
		root.completionEndWeek,
		root.completionTimelineWidth(),
	)
	if !window.Start.Before(domain.MustParseDate("2026-04-01")) {
		t.Fatalf("history stopped at %s, want navigation before April", window.Start)
	}
}

func TestCompletionHistoryRemainsBoundedAcrossResponsiveHeaderTransitions(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	backend := newFakeBackend(today).withHabits(domain.Habit{
		Title:     "Long history",
		CreatedOn: domain.MustParseDate("2025-01-01"),
	})
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})

	for range 20 {
		root = pressKey(t, root, "left")
	}
	if root.completionEndWeek.IsZero() {
		t.Fatal("test setup did not enter a historical viewport")
	}

	assertState := func(width, height int, wantASCII bool) {
		t.Helper()
		root = send(t, root, tea.WindowSizeMsg{Width: width, Height: height})
		view := root.View()
		if !view.AltScreen {
			t.Fatalf("%dx%d resize lost alternate-screen mode", width, height)
		}
		if got := lipgloss.Height(view.Content); got > height {
			t.Fatalf("%dx%d view height = %d", width, height, got)
		}
		hasASCII := strings.Contains(stripANSI(root.renderHeader()), "┓      ┓•")
		if hasASCII != wantASCII {
			t.Fatalf("%dx%d ASCII header = %v, want %v", width, height, hasASCII, wantASCII)
		}

		window := completionVisibleWindow(
			root.snap.Completion.Start,
			root.today,
			root.completionEndWeek,
			root.completionTimelineWidth(),
		)
		if window.Start.Before(window.Earliest) || window.End.After(window.Latest) {
			t.Fatalf("%dx%d completion window escaped history bounds: %+v", width, height, window)
		}
	}

	assertState(120, 35, false)
	for range 5 {
		root = send(t, root, tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	}
	assertState(120, 36, true)
	assertState(200, 60, true)
	for range 5 {
		root = send(t, root, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	}
	assertState(120, 35, false)
}

func TestCompletionPresentAnchorFollowsDateWhileHistoricalViewportStaysPut(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Daily", CreatedOn: today.AddDays(-200)})
	root := newReadyRoot(t, backend)
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})

	root = pressKey(t, root, "left")
	historicalEnd := root.completionEndWeek
	backend.today = today.AddDays(8)
	snap, err := backend.Snapshot(nil, app.SnapshotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	root.applySnapshot(snap)
	if !root.completionEndWeek.Equal(historicalEnd) {
		t.Fatalf("historical viewport moved across date change: got %s want %s", root.completionEndWeek, historicalEnd)
	}

	root.completionEndWeek = domain.Date{}
	backend.today = backend.today.AddDays(7)
	snap, err = backend.Snapshot(nil, app.SnapshotRequest{})
	if err != nil {
		t.Fatal(err)
	}
	root.applySnapshot(snap)
	if !root.completionEndWeek.IsZero() {
		t.Fatal("present-anchored viewport lost its anchor after date change")
	}
	window := completionVisibleWindow(root.snap.Completion.Start, root.today, root.completionEndWeek, root.completionTimelineWidth())
	if !window.End.Equal(completionSunday(root.today)) {
		t.Fatalf("anchored window ends %s, want current week %s", window.End, completionSunday(root.today))
	}
}

func TestCompletionMonthLabelsTrackContinuousWeeksWithoutOverlap(t *testing.T) {
	geometry := completionGeometry(43)
	window := completionWeekWindow{
		Start:    domain.MustParseDate("2026-03-29"),
		Count:    10,
		Capacity: geometry.Capacity,
	}
	marks := completionMonthMarks(window, geometry)
	if len(marks) != 3 {
		t.Fatalf("month marks = %+v, want apr/may/jun", marks)
	}
	want := []struct {
		label  string
		column int
	}{{"apr", 0}, {"may", 8}, {"jun", 18}}
	for i := range want {
		if marks[i].Label != want[i].label || marks[i].Column != want[i].column {
			t.Errorf("mark %d = %+v, want %s at %d", i, marks[i], want[i].label, want[i].column)
		}
		if i > 0 && marks[i].Column < marks[i-1].Column+lipgloss.Width(marks[i-1].Label)+1 {
			t.Fatalf("month labels overlap: %+v", marks)
		}
	}

	yearWindow := completionWeekWindow{Start: domain.MustParseDate("2026-12-27"), Count: 2, Capacity: geometry.Capacity}
	yearMarks := completionMonthMarks(yearWindow, geometry)
	if len(yearMarks) != 1 || yearMarks[0].Label != "jan" || yearMarks[0].Column != 0 {
		t.Fatalf("year-transition marks = %+v, want jan on the spanning week", yearMarks)
	}
}

func TestCompletionMonthLabelsDoNotAnnounceFutureMonth(t *testing.T) {
	today := domain.MustParseDate("2026-04-30")
	window := completionVisibleWindow(domain.MustParseDate("2026-04-01"), today, domain.Date{}, 27)
	marks := completionMonthMarks(window, completionGeometry(27))
	for _, mark := range marks {
		if mark.Label == "may" {
			t.Fatalf("future May label rendered before May 1 occurred: %+v", marks)
		}
	}
}

func TestCompletionMonthLabelsMoveWithHistoricalWeekScrolling(t *testing.T) {
	today := domain.MustParseDate("2027-01-06")
	root := newReadyRoot(t, newFakeBackend(today).withHabits(domain.Habit{
		Title:     "History",
		CreatedOn: domain.MustParseDate("2026-08-01"),
	}))
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})
	width := root.completionTimelineWidth()
	geometry := completionGeometry(width)

	present := completionVisibleWindow(root.snap.Completion.Start, root.today, root.completionEndWeek, width)
	presentMarks := completionMonthMarks(present, geometry)
	if !hasCompletionMonthMark(presentMarks, "jan") {
		t.Fatalf("present month marks = %+v, want January across the year boundary", presentMarks)
	}
	presentLabels := stripANSI(root.renderCompletionMonthLabels(present, geometry))
	if !strings.Contains(presentLabels, "jan") {
		t.Fatalf("present label row = %q, want rendered January label", presentLabels)
	}

	for range 5 {
		root = pressKey(t, root, "left")
	}
	historical := completionVisibleWindow(root.snap.Completion.Start, root.today, root.completionEndWeek, width)
	historicalMarks := completionMonthMarks(historical, geometry)
	if historical.Start.Equal(present.Start) || historical.End.Equal(present.End) {
		t.Fatalf("historical scrolling did not move the visible calendar range: present=%+v historical=%+v", present, historical)
	}
	if hasCompletionMonthMark(historicalMarks, "jan") {
		t.Fatalf("January label remained after its boundary left the viewport: %+v", historicalMarks)
	}
	if !hasCompletionMonthMark(historicalMarks, "oct") {
		t.Fatalf("scrolled month marks = %+v, want October boundary entering the viewport", historicalMarks)
	}
	historicalLabels := stripANSI(root.renderCompletionMonthLabels(historical, geometry))
	if strings.Contains(historicalLabels, "jan") || !strings.Contains(historicalLabels, "oct") {
		t.Fatalf("scrolled label row = %q, want October and no January", historicalLabels)
	}
}

func hasCompletionMonthMark(marks []completionMonthMark, label string) bool {
	for _, mark := range marks {
		if mark.Label == label {
			return true
		}
	}
	return false
}

func TestCompletionEmptyHistoryStillRendersVisibleCalendarHistory(t *testing.T) {
	root := newReadyRoot(t, newFakeBackend(domain.MustParseDate("2026-08-12")))
	if len(root.snap.Completion.Days) != 1 || !root.snap.Completion.Start.Equal(root.today) {
		t.Fatalf("empty-history timeline = %+v, want one inactive today summary", root.snap.Completion)
	}
	geometry := completionGeometry(27)
	rendered := stripANSI(root.renderCompletionTimeline(27))
	want := (geometry.Capacity-1)*7 + int(root.today.Weekday()) + 1
	if got := strings.Count(rendered, theme.HeatmapSquare); got != want {
		t.Fatalf("empty completion history has %d calendar cells, want %d occurred dates", got, want)
	}
}

func TestCompletionRenderingIsBoundedToVisibleWeeks(t *testing.T) {
	today := domain.MustParseDate("2026-08-12")
	root := newReadyRoot(t, newFakeBackend(today).withHabits(domain.Habit{
		Title:     "A decade",
		CreatedOn: today.AddDays(-3650),
	}))
	geometry := completionGeometry(27)
	rendered := stripANSI(root.renderCompletionTimeline(27))
	if got := strings.Count(rendered, theme.HeatmapSquare); got > geometry.Capacity*7 {
		t.Fatalf("rendered %d cells, want at most %d visible cells", got, geometry.Capacity*7)
	}
	for _, line := range strings.Split(rendered, "\n")[2:] {
		if strings.ContainsAny(line, "0123456789") {
			t.Fatalf("Completion rendered a numeric date: %q", line)
		}
	}
	for _, unwanted := range []string{"less", "more", "progress"} {
		if strings.Contains(strings.ToLower(rendered), unwanted) {
			t.Fatalf("Completion retained a permanent legend entry %q", unwanted)
		}
	}
}

func TestHeatmapSquareHasStableTerminalCellWidth(t *testing.T) {
	if theme.HeatmapSquare != "\uf0c8" {
		t.Fatalf("heatmap square = %q, want exact U+F0C8", theme.HeatmapSquare)
	}
	if got := lipgloss.Width(theme.HeatmapSquare); got != 1 {
		t.Fatalf("U+F0C8 heatmap square width = %d, want one terminal cell", got)
	}
}

func TestTodayProgressUsesSameCompletionStageStyle(t *testing.T) {
	root := rootWithGroups()
	zero := root.renderDayProgress(30, domain.DailySummary{Scheduled: 4})
	complete := root.renderDayProgress(30, domain.DailySummary{Scheduled: 4, Completed: 4})
	if zero == complete {
		t.Fatal("0% and 100% Today progress must not render identically")
	}
	zeroColor := root.theme.CompletionStyles[theme.CompletionStageZero].GetForeground()
	completeColor := root.theme.CompletionStyles[theme.CompletionStageComplete].GetForeground()
	if root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageZero]).GetForeground() != zeroColor ||
		root.theme.ProgressLabel.Inherit(root.theme.CompletionStyles[theme.CompletionStageComplete]).GetForeground() != completeColor {
		t.Fatal("Today progress labels do not inherit the shared completion colors")
	}
}

func TestCompletionTimelineRowsFitRepresentativeWidths(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	for _, width := range []int{27, 33, 41} {
		for row, line := range strings.Split(root.renderCompletionTimeline(width), "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d row %d = %d cells: %q", width, row, got, stripANSI(line))
			}
		}
	}
}

func TestReminderCalendarStylePreservesTodayAndReminderDates(t *testing.T) {
	root := rootWithGroups()
	date := domain.MustParseDate("2026-08-10")
	todayStyle := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, IsToday: true})
	reminderStyle := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, HasReminder: true})
	both := root.reminderDayStyle(app.CalendarDay{Date: date, InMonth: true, IsToday: true, HasReminder: true})
	if !todayStyle.GetBold() || !reminderStyle.GetUnderline() || !both.GetBold() || !both.GetUnderline() {
		t.Fatal("conventional Calendar lost today/reminder precedence")
	}
}

func TestReminderCalendarStartsOnSunday(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(sixWeekMonthToday))
	if got := root.snap.Calendar.Weeks[0][0].Date.Weekday(); got != time.Sunday {
		t.Fatalf("Reminder Calendar first column = %s, want Sunday", got)
	}
	header := strings.Split(stripANSI(root.renderReminderCalendar(29)), "\n")[2]
	if !strings.Contains(header, "Su  Mo  Tu  We  Th  Fr  Sa") {
		t.Fatalf("Reminder Calendar header is not Sunday-first: %q", header)
	}
}
