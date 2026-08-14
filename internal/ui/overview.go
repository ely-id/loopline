package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// renderOverview keeps Lifetime above a bounded Weekly Overview viewport and
// anchors Completion at the bottom of the available panel height.
func (m *Root) renderOverview(width, height int) string {
	lifetime := m.renderLifetime(width)
	completion := m.renderCompletionTimeline(width)
	fixed := lipgloss.Height(lifetime) + lipgloss.Height(completion) + 2
	weeklyHeight := max(height-fixed, 1)
	weekly := m.renderWeeklyOverview(width, weeklyHeight)
	return strings.Join([]string{lifetime, "", weekly, "", completion}, "\n")
}

func (m *Root) overviewBlockTitle(block int) lipgloss.Style {
	if m.panelFocused(ColOverview) && m.focus.Block == block {
		return m.theme.BlockTitleOn
	}
	return m.theme.BlockTitle
}

func (m *Root) renderWeeklyOverview(width, height int) string {
	if height <= 0 {
		return ""
	}
	rows := m.snap.Weekly.Rows
	titleText := theme.IconWeekly + " Weekly Overview"
	if height == 1 {
		return m.overviewBlockTitle(OverviewWeekly).Render(ansi.Truncate(titleText, width, "…"))
	}
	bodyHeight := max(height-2, 0) // title + weekday header
	if len(rows) > bodyHeight {
		titleText += "  " + m.theme.Dim.Render("↕")
	}
	title := m.overviewBlockTitle(OverviewWeekly).Render(titleText)
	header := m.renderWeeklyHeader(width)

	lines := make([]string, len(rows))
	for i, row := range rows {
		selected := m.panelFocused(ColOverview) && m.focus.Block == OverviewWeekly && m.focus.Item == i
		lines[i] = m.renderWeeklyRow(width, row, selected)
	}
	focus := -1
	if m.panelFocused(ColOverview) && m.focus.Block == OverviewWeekly {
		focus = m.focus.Item
	}
	if height == 2 {
		return title + "\n" + header
	}
	body := ""
	if len(lines) == 0 {
		body = ansi.Truncate(m.theme.Muted.Render("No habits scheduled this week."), width, "…")
	} else {
		body = strings.Join(windowAround(lines, bodyHeight, focus), "\n")
	}
	content := strings.Join([]string{title, header, body}, "\n")
	return padLinesTo(content, height)
}

func weeklyGridLayout(width int) (nameWidth int, labels []string) {
	labels = []string{"S", "M", "T", "W", "T", "F", "S"}
	gridWidth := 13 // seven one-cell labels with one-cell gaps
	if width >= 35 {
		labels = []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
		gridWidth = 20
	}
	return max(width-gridWidth-1, 1), labels
}

func (m *Root) renderWeeklyHeader(width int) string {
	nameWidth, labels := weeklyGridLayout(width)
	cells := make([]string, len(labels))
	for i, label := range labels {
		style := m.theme.Dim
		if m.snap.Weekly.Start.AddDays(i).Equal(m.today) {
			style = style.Bold(true).Underline(true)
		}
		cells[i] = style.Render(label)
	}
	return strings.Repeat(" ", nameWidth+1) + strings.Join(cells, " ")
}

func (m *Root) renderWeeklyRow(width int, row app.WeeklyHabit, selected bool) string {
	nameWidth, labels := weeklyGridLayout(width)
	name := ansi.Truncate(row.Habit.Title, nameWidth, "…")
	name = padTo(name, nameWidth)
	cells := make([]string, len(row.Days))
	for i, day := range row.Days {
		glyph := " "
		style := m.theme.Dim
		switch {
		case day.Completed:
			glyph, style = "✓", m.theme.CheckDone
		case day.Scheduled && day.IsFuture:
			glyph = "·"
		case day.Scheduled:
			glyph, style = "·", m.theme.CheckPending
		}
		if day.IsToday {
			style = style.Bold(true).Underline(true)
		}
		if lipgloss.Width(labels[i]) == 2 {
			glyph = " " + glyph
		}
		cells[i] = style.Render(glyph)
	}
	line := name + " " + strings.Join(cells, " ")
	if selected {
		return m.theme.ItemSelected.Render(padTo(line, width))
	}
	return line
}

func (m *Root) renderLifetime(width int) string {
	ov := m.snap.Overview
	lines := []string{
		m.overviewBlockTitle(OverviewLifetime).Render(theme.IconLifetime + " Lifetime"),
		m.theme.Text.MaxWidth(width).Render(fmt.Sprintf("Completed  %d · %d%%", ov.TotalCompletions, ov.LifetimePercent)),
		m.theme.Dim.MaxWidth(width).Render(fmt.Sprintf("Tracked    %d days", ov.DaysTracked)),
		m.theme.Dim.MaxWidth(width).Render(fmt.Sprintf("Streak     %d · best %d", ov.CurrentStreak, ov.BestStreak)),
	}
	return strings.Join(lines, "\n")
}

const completionWeekGap = 1

var completionWeekdayLabels = [7]string{"Su", "", "Tu", "", "Th", "", "Sa"}

type completionGridGeometry struct {
	Gutter, CellWidth, Stride, Capacity int
}

type completionWeekWindow struct {
	Start, End, Earliest, Latest, Today domain.Date
	Count, Leading, Capacity            int
}

type completionMonthMark struct {
	Column int
	Label  string
}

func completionGeometry(width int) completionGridGeometry {
	cellWidth := max(lipgloss.Width(theme.HeatmapSquare), 1)
	gutter := 3 // compact two-cell weekday label plus one separating cell
	stride := cellWidth + completionWeekGap
	usable := max(width-gutter, 0)
	capacity := 0
	if usable >= cellWidth {
		capacity = 1 + (usable-cellWidth)/stride
	}
	return completionGridGeometry{Gutter: gutter, CellWidth: cellWidth, Stride: stride, Capacity: capacity}
}

func completionSunday(d domain.Date) domain.Date {
	return d.AddDays(-int(d.Weekday()))
}

// completionVisibleWindow resolves the calendar range before rendering.
// EndWeek is zero while present-anchored; otherwise it is the user's
// historical viewport end. The viewport always contains every calendar week
// that fits; Completion.Start limits historical scrolling, not whether an
// already-occurred date exists on the calendar.
func completionVisibleWindow(start, today, endWeek domain.Date, width int) completionWeekWindow {
	geometry := completionGeometry(width)
	if start.IsZero() || start.After(today) {
		start = today
	}
	earliest := completionSunday(start)
	latest := completionSunday(today)
	count := geometry.Capacity
	window := completionWeekWindow{
		Earliest: earliest,
		Latest:   latest,
		Today:    today,
		Count:    count,
		Capacity: geometry.Capacity,
	}
	if count <= 0 {
		return window
	}

	end := endWeek
	if end.IsZero() || end.After(latest) {
		end = latest
	}
	minimumEnd := earliest.AddDays((count - 1) * 7)
	if minimumEnd.After(latest) {
		minimumEnd = latest
	}
	if end.Before(minimumEnd) {
		end = minimumEnd
	}
	window.End = end
	window.Start = end.AddDays(-(count - 1) * 7)
	return window
}

func (m *Root) renderCompletionTimeline(width int) string {
	window := completionVisibleWindow(m.snap.Completion.Start, m.today, m.completionEndWeek, width)
	geometry := completionGeometry(width)
	lines := make([]string, 0, 9)
	lines = append(lines, m.overviewBlockTitle(OverviewCompletion).Render(theme.IconCompletion+" Completion"))
	lines = append(lines, m.renderCompletionMonthLabels(window, geometry))
	for weekday, label := range completionWeekdayLabels {
		lines = append(lines, m.renderCompletionWeekday(window, geometry, weekday, label))
	}
	return strings.Join(lines, "\n")
}

func (m *Root) completionTimelineWidth() int {
	outer := columnWidths(m.width)
	return max(outer[ColOverview]-m.theme.Column.GetHorizontalFrameSize(), 1)
}

func (m *Root) clampCompletionViewport() {
	if !m.ready || m.completionEndWeek.IsZero() {
		return
	}
	window := completionVisibleWindow(
		m.snap.Completion.Start,
		m.today,
		m.completionEndWeek,
		m.completionTimelineWidth(),
	)
	if window.Count <= 0 || window.End.Equal(window.Latest) {
		m.completionEndWeek = domain.Date{}
		return
	}
	m.completionEndWeek = window.End
}

// scrollCompletion moves the visible end in whole calendar weeks. A zero
// completionEndWeek is the present anchor; reaching the current week while
// scrolling forward restores that state.
func (m *Root) scrollCompletion(delta int) {
	if delta == 0 {
		return
	}
	window := completionVisibleWindow(
		m.snap.Completion.Start,
		m.today,
		m.completionEndWeek,
		m.completionTimelineWidth(),
	)
	if window.Count <= 0 {
		return
	}

	if delta < 0 {
		minimumEnd := window.Earliest.AddDays((window.Count - 1) * 7)
		target := window.End.AddDays(-7)
		if target.Before(minimumEnd) {
			target = minimumEnd
		}
		if target.Before(window.Latest) {
			m.completionEndWeek = target
		}
		return
	}

	if m.completionEndWeek.IsZero() {
		return
	}
	target := window.End.AddDays(7)
	if !target.Before(window.Latest) {
		m.completionEndWeek = domain.Date{}
		return
	}
	m.completionEndWeek = target
}

func (m *Root) renderCompletionWeekday(window completionWeekWindow, geometry completionGridGeometry, weekday int, label string) string {
	var row strings.Builder
	row.Grow(geometry.Gutter + geometry.Capacity*geometry.Stride)
	row.WriteString(padTo(label, geometry.Gutter-1))
	row.WriteByte(' ')
	row.WriteString(strings.Repeat(" ", window.Leading*geometry.Stride))

	for week := 0; week < window.Count; week++ {
		date := completionCellDate(window.Start, week, weekday)
		if !date.After(m.today) {
			row.WriteString(m.completionDayCell(date))
		} else {
			row.WriteString(strings.Repeat(" ", geometry.CellWidth))
		}
		if week < window.Count-1 {
			row.WriteString(strings.Repeat(" ", completionWeekGap))
		}
	}
	return strings.TrimRight(row.String(), " ")
}

func completionCellDate(windowStart domain.Date, week, weekday int) domain.Date {
	return windowStart.AddDays(week*7 + weekday)
}

func (m *Root) completionSummary(date domain.Date) domain.DailySummary {
	index := date.DaysSince(m.snap.Completion.Start)
	if index < 0 || index >= len(m.snap.Completion.Days) {
		return domain.DailySummary{Date: date}
	}
	summary := m.snap.Completion.Days[index]
	if !summary.Date.Equal(date) {
		return domain.DailySummary{Date: date}
	}
	return summary
}

func (m *Root) completionDayCell(date domain.Date) string {
	return m.completionDayStyle(date).Render(theme.HeatmapSquare)
}

func (m *Root) completionDayStyle(date domain.Date) lipgloss.Style {
	style := m.theme.CompletionStyles[completionStage(m.completionSummary(date))]
	if date.Equal(m.today) {
		style = style.Bold(true).Underline(true)
	}
	return style
}

func completionMonthMarks(window completionWeekWindow, geometry completionGridGeometry) []completionMonthMark {
	if window.Count <= 0 {
		return nil
	}
	gridWidth := max(geometry.Capacity*geometry.Stride-completionWeekGap, 0)
	marks := make([]completionMonthMark, 0, window.Count/4+1)
	lastEnd := -2
	for week := 0; week < window.Count; week++ {
		weekStart := window.Start.AddDays(week * 7)
		var first domain.Date
		for weekday := 0; weekday < 7; weekday++ {
			date := weekStart.AddDays(weekday)
			if date.Day == 1 {
				first = date
				break
			}
		}
		if first.IsZero() {
			continue
		}
		if !window.Today.IsZero() && first.After(window.Today) {
			continue
		}
		label := strings.ToLower(first.Time().Format("Jan"))
		column := (window.Leading + week) * geometry.Stride
		end := column + lipgloss.Width(label)
		if column < lastEnd+1 || end > gridWidth {
			continue
		}
		marks = append(marks, completionMonthMark{Column: column, Label: label})
		lastEnd = end
	}
	return marks
}

func (m *Root) renderCompletionMonthLabels(window completionWeekWindow, geometry completionGridGeometry) string {
	var line strings.Builder
	line.WriteString(strings.Repeat(" ", geometry.Gutter))
	cursor := 0
	for _, mark := range completionMonthMarks(window, geometry) {
		line.WriteString(strings.Repeat(" ", max(mark.Column-cursor, 0)))
		line.WriteString(m.theme.Dim.Render(mark.Label))
		cursor = mark.Column + lipgloss.Width(mark.Label)
	}
	return strings.TrimRight(line.String(), " ")
}

func calendarMonthLabel(cal app.CalendarMonth) string {
	return fmt.Sprintf("< %s %d >", cal.Month.String()[:3], cal.Year)
}

// completionStage is the single percentage-to-semantics mapping used by the
// timeline and Today. IsComplete, rather than a rounded 100%, owns the final
// stage so a 199/200 day can never look fully complete.
func completionStage(sum domain.DailySummary) theme.CompletionStage {
	if sum.Scheduled <= 0 {
		return theme.CompletionStageNone
	}
	if sum.Completed <= 0 {
		return theme.CompletionStageZero
	}
	if sum.IsComplete() {
		return theme.CompletionStageComplete
	}
	switch pct := sum.Percent(); {
	case pct <= 33:
		return theme.CompletionStageLow
	case pct <= 66:
		return theme.CompletionStageMedium
	default:
		return theme.CompletionStageHigh
	}
}

type yearProgress struct {
	Day, Total int
	Percent    int
	ISOWeek    int
}

func calculateYearProgress(today domain.Date) yearProgress {
	start := domain.NewDate(today.Year, 1, 1)
	next := domain.NewDate(today.Year+1, 1, 1)
	day := today.DaysSince(start) + 1
	total := next.DaysSince(start)
	pct := (day*100 + total/2) / total
	if day < total && pct == 100 {
		pct = 99
	}
	_, week := today.ISOWeek()
	return yearProgress{Day: day, Total: total, Percent: pct, ISOWeek: week}
}

func (m *Root) renderYearProgress(width int) string {
	year := calculateYearProgress(m.today)
	label := fmt.Sprintf("%d%%", year.Percent)
	barWidth := clampInt(width-lipgloss.Width(label)-2, 8, 40)
	bar := renderBar(barWidth, year.Day, year.Total, m.theme.YearBarFilled, m.theme.ProgressBarEmpty, theme.YearFilled, theme.YearEmpty)
	return strings.Join([]string{
		m.theme.BlockTitle.Render(theme.IconYear + " Year"),
		bar + "  " + m.theme.Bold.Render(label),
		m.theme.Dim.MaxWidth(width).Render(fmt.Sprintf("%d / %d days · Week %d", year.Day, year.Total, year.ISOWeek)),
	}, "\n")
}
