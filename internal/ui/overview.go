package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// renderOverview keeps the history column deliberately compact: lifetime
// statistics, authoritative monthly completion, then temporal orientation.
func (m *Root) renderOverview(width int) string {
	return strings.Join([]string{
		m.columnTitle(ColOverview, theme.IconLoop+" loopline · Overview"),
		"",
		m.renderLifetime(width),
		"",
		m.renderCompletionCalendar(width),
		"",
		m.renderYearProgress(width),
	}, "\n")
}

func (m *Root) overviewBlockTitle(block int) lipgloss.Style {
	if m.focus.Column == ColOverview && m.focus.Block == block {
		return m.theme.BlockTitleOn
	}
	return m.theme.BlockTitle
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
		m.theme.Dim.Render(calendarGridRow(width, 3, []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"})),
	}

	for _, week := range cal.Weeks {
		cells := make([]string, len(week))
		for i, day := range week {
			cells[i] = m.completionDayCell(day)
		}
		lines = append(lines, calendarGridRow(width, 3, cells))
	}
	return strings.Join(lines, "\n")
}

func calendarMonthLabel(cal app.CalendarMonth) string {
	return fmt.Sprintf("< %s %d >", cal.Month.String()[:3], cal.Year)
}

// completionDayCell renders a three-cell-wide day (two date cells plus one
// density glyph). Future days and days with nothing scheduled stay neutral;
// neither is incorrectly presented as missed work.
func (m *Root) completionDayCell(day app.CalendarDay) string {
	if !day.InMonth {
		return "   "
	}

	stage := completionStage(day.Summary)
	if day.IsFuture {
		stage = theme.CompletionStageNone
	}
	numberStyle := m.completionDayNumberStyle(day, stage)
	number := numberStyle.Render(fmt.Sprintf("%2d", day.Date.Day))
	return number + m.theme.CompletionStyles[stage].Render(string(theme.CompletionGlyphs[stage]))
}

func (m *Root) completionDayNumberStyle(day app.CalendarDay, stage theme.CompletionStage) lipgloss.Style {
	style := m.theme.CompletionStyles[stage]
	if day.IsFuture {
		style = m.theme.CalFuture
	}
	// Underlining is independent of the completion palette, so today remains
	// identifiable at every completion stage without stealing the stage color.
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
