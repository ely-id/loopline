package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// renderOverview builds the whole overview column: quick actions, the
// lifetime summary + month calendar, the current ISO week, and the
// contribution heatmap.
func (m *Root) renderOverview(width int) string {
	var b strings.Builder

	b.WriteString(m.columnTitle(ColOverview, "Overview"))
	b.WriteString("\n\n")

	b.WriteString(m.renderOverviewActions())
	b.WriteString("\n\n")

	b.WriteString(m.renderCalendar())
	b.WriteString("\n\n")

	b.WriteString(m.renderWeek())
	b.WriteString("\n\n")

	b.WriteString(m.renderHeatmap(width))
	return b.String()
}

func (m *Root) renderOverviewActions() string {
	focused := m.focus.Column == ColOverview && m.focus.Block == OverviewActions
	lines := []string{
		m.renderActionLine("Add habit", focused && m.focus.Item == ActionAddHabit),
		m.renderActionLine("Add routine", focused && m.focus.Item == ActionAddRoutine),
	}
	return strings.Join(lines, "\n")
}

func (m *Root) blockTitleStyle(block int) lipgloss.Style {
	if m.focus.Column == ColOverview && m.focus.Block == block {
		return m.theme.BlockTitleOn
	}
	return m.theme.BlockTitle
}

func (m *Root) renderOverviewStats() string {
	ov := m.snap.Overview
	line1 := fmt.Sprintf("Streak %d  Best %d  Lifetime %d%%", ov.CurrentStreak, ov.BestStreak, ov.LifetimePercent)
	line2 := fmt.Sprintf("Habits %d  Routines %d  Open reminders %d", ov.ActiveHabits, ov.ActiveRoutines, ov.OpenReminders)
	return m.theme.Text.Render(line1) + "\n" + m.theme.Dim.Render(line2)
}

func (m *Root) renderCalendar() string {
	title := m.blockTitleStyle(OverviewLifetime)

	var b strings.Builder
	b.WriteString(title.Render("Lifetime"))
	b.WriteString("\n")
	b.WriteString(m.renderOverviewStats())
	b.WriteString("\n\n")

	cal := m.snap.Calendar
	monthLabel := fmt.Sprintf("%s %d", cal.Month.String()[:3], cal.Year)
	b.WriteString(title.Render(monthLabel))
	b.WriteString("\n")
	b.WriteString(m.theme.Dim.Render("Mo Tu We Th Fr Sa Su"))
	b.WriteString("\n")

	for _, week := range cal.Weeks {
		cells := make([]string, len(week))
		for i, d := range week {
			cells[i] = m.dayCellStyle(d).Render(fmt.Sprintf("%2d", d.Date.Day))
		}
		b.WriteString(strings.Join(cells, " "))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Root) dayCellStyle(d app.CalendarDay) lipgloss.Style {
	var style lipgloss.Style
	switch {
	case !d.InMonth:
		style = m.theme.CalOutMonth
	case d.IsFuture:
		style = m.theme.CalFuture
	case d.Summary.Scheduled > 0 && d.Summary.IsComplete():
		style = m.theme.CalComplete
	case d.Summary.Completed > 0:
		style = m.theme.CalPartial
	default:
		style = m.theme.CalInMonth
	}
	if d.IsToday {
		style = style.Bold(true)
	}
	if d.HasReminder {
		style = style.Underline(true)
	}
	return style
}

func (m *Root) renderWeek() string {
	title := m.blockTitleStyle(OverviewWeek)
	w := m.snap.Week
	levels := m.heatmapLevelIndex()

	var b strings.Builder
	b.WriteString(title.Render(fmt.Sprintf("Week %d · %d", w.ISOWeek, w.ISOYear)))
	b.WriteString("\n")

	labels := make([]string, len(w.Days))
	cells := make([]string, len(w.Days))
	for i, d := range w.Days {
		labels[i] = d.Date.Time().Format("Mon")[:2]
		cells[i] = m.weekDayCell(d, levels)
	}
	b.WriteString(m.theme.Dim.Render(strings.Join(labels, " ")))
	b.WriteString("\n")
	b.WriteString(strings.Join(cells, " "))
	return b.String()
}

// heatmapLevelIndex maps date to the completion level internal/app already
// computed for the heatmap. The week strip prefers this wire-provided value
// over re-deriving its own bucket, so the two views can never drift apart.
func (m *Root) heatmapLevelIndex() map[domain.Date]int {
	idx := make(map[domain.Date]int, 7)
	for _, col := range m.snap.Heatmap.Columns {
		for _, cell := range col {
			if !cell.Date.IsZero() {
				idx[cell.Date] = cell.Level
			}
		}
	}
	return idx
}

func (m *Root) weekDayCell(d app.CalendarDay, levels map[domain.Date]int) string {
	level, ok := levels[d.Date]
	if !ok {
		// The selected heatmap range doesn't happen to cover this day (e.g.
		// early in a month, with the default month range): fall back to a
		// locally computed bucket so the week strip is never blank.
		level = fallbackLevel(d.Summary)
	}
	style := m.theme.HeatmapLevels[level]
	if d.IsToday {
		style = style.Bold(true).Underline(true)
	}
	return style.Render(strings.Repeat(string(theme.HeatmapGlyphs[level]), 2))
}

func (m *Root) renderHeatmap(width int) string {
	focused := m.focus.Column == ColOverview && m.focus.Block == OverviewHeatmap
	title := m.blockTitleStyle(OverviewHeatmap)
	hm := m.snap.Heatmap

	var b strings.Builder
	b.WriteString(title.Render(fmt.Sprintf("Heatmap · %s", hm.Range.Label())))
	b.WriteString("\n")
	if focused {
		b.WriteString(m.theme.Dim.Render("enter or </> change range"))
	} else {
		b.WriteString(m.theme.Dim.Render(" "))
	}
	b.WriteString("\n")

	cols := hm.Columns
	maxCols := clampInt(width-2, 1, len(cols))
	if len(cols) > maxCols {
		cols = cols[len(cols)-maxCols:]
	}

	for row := 0; row < 7; row++ {
		var line strings.Builder
		for _, col := range cols {
			cell := col[row]
			if cell.Date.IsZero() {
				line.WriteString(" ")
				continue
			}
			line.WriteString(m.theme.HeatmapLevels[cell.Level].Render(string(theme.HeatmapGlyphs[cell.Level])))
		}
		b.WriteString(line.String())
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// fallbackLevel buckets a day's completion into the same five shades
// internal/app's heatLevel uses. It exists only for weekDayCell's fallback
// path above (a day outside the selected heatmap range); every other reader
// of "level" uses the value already computed on the wire.
func fallbackLevel(s domain.DailySummary) int {
	if s.Scheduled == 0 || s.Completed == 0 {
		return 0
	}
	switch pct := s.Percent(); {
	case pct <= 25:
		return 1
	case pct <= 50:
		return 2
	case pct <= 75:
		return 3
	default:
		return 4
	}
}
