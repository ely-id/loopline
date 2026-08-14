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
	completion := m.renderCompletionCalendar(width)
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

func (m *Root) renderCompletionCalendar(width int) string {
	cal := m.snap.Calendar
	lines := []string{
		m.overviewBlockTitle(OverviewCompletion).Render(theme.IconCompletion + " Completion"),
		m.theme.Dim.Render(centeredLine(calendarMonthLabel(cal), width)),
	}

	var cells []string
	for _, week := range cal.Weeks {
		for _, day := range week {
			if day.InMonth {
				cells = append(cells, m.completionDayCell(day))
			}
		}
	}
	lines = append(lines, justifiedCompletionRows(cells, width)...)
	return strings.Join(lines, "\n")
}

// justifiedCompletionRows renders chronological month days as a compact flow
// without a weekday axis. Rows are balanced before their inter-cell spacing is
// expanded, avoiding a nearly empty final row while still using the available
// width deliberately.
func justifiedCompletionRows(cells []string, width int) []string {
	if len(cells) == 0 || width <= 0 {
		return nil
	}
	maxPerRow := max((width+1)/2, 1) // at least one cell of separation
	rowCount := (len(cells) + maxPerRow - 1) / maxPerRow
	base, extra := len(cells)/rowCount, len(cells)%rowCount

	rows := make([]string, 0, rowCount)
	start := 0
	for row := 0; row < rowCount; row++ {
		count := base
		if row < extra {
			count++
		}
		rows = append(rows, justifyCompletionRow(cells[start:start+count], width))
		start += count
	}
	return rows
}

func justifyCompletionRow(cells []string, width int) string {
	if len(cells) == 0 {
		return ""
	}
	if len(cells) == 1 {
		return centeredLine(cells[0], width)
	}

	cellsWidth := 0
	for _, cell := range cells {
		cellsWidth += lipgloss.Width(cell)
	}
	space := max(width-cellsWidth, len(cells)-1)
	baseGap, extra := space/(len(cells)-1), space%(len(cells)-1)

	var row strings.Builder
	for i, cell := range cells {
		row.WriteString(cell)
		if i == len(cells)-1 {
			continue
		}
		gap := baseGap
		if i < extra {
			gap++
		}
		row.WriteString(strings.Repeat(" ", gap))
	}
	return row.String()
}

func calendarMonthLabel(cal app.CalendarMonth) string {
	return fmt.Sprintf("< %s %d >", cal.Month.String()[:3], cal.Year)
}

// completionDayCell renders exactly one requested U+F0C8 square for an
// active-month day. Adjacent-month positions remain visually empty.
func (m *Root) completionDayCell(day app.CalendarDay) string {
	if !day.InMonth {
		return ""
	}

	stage := completionStage(day.Summary)
	if day.IsFuture {
		stage = theme.CompletionStageNone
	}
	return m.completionDayStyle(day, stage).Render(theme.HeatmapSquare)
}

func (m *Root) completionDayStyle(day app.CalendarDay, stage theme.CompletionStage) lipgloss.Style {
	style := m.theme.CompletionStyles[stage]
	if day.IsToday {
		style = style.Bold(true).Underline(true)
	}
	return style
}

// completionStage is the single percentage-to-semantics mapping used by the
// month and Today. IsComplete, rather than a rounded 100%, owns the final
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
