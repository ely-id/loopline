package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// todayChrome is the number of lines renderToday always reserves around the
// habit list: title, a blank line, a blank line, then the progress bar. That
// bar is written unconditionally so it always renders at the bottom of the
// column, however long the habit list is.
const todayChrome = 4

// renderToday builds the whole "today" column: title, one visually separated
// block per habit group (windowed to fit height so the focused habit is
// always visible), then the day's progress bar pinned to the bottom.
func (m *Root) renderToday(width, height int) string {
	title := m.columnTitle(ColToday, fmt.Sprintf("Today — %s", weekdayLabel(m.snap.Today.Date)))
	progress := m.renderDayProgress(width, m.snap.Today.Summary)

	body := m.theme.Muted.Render("No habits scheduled today.")
	if lines, focusLine := m.todayLines(width); len(lines) > 0 {
		listHeight := max(height-todayChrome, 1)
		body = strings.Join(windowAround(lines, listHeight, focusLine), "\n")
	}

	return title + "\n\n" + body + "\n\n" + progress
}

// todayLines flattens every habit group into one line per row (an optional
// group title, then one line per habit), plus a blank separator between
// groups. It also returns the absolute index of the focused line, or -1 if
// focus isn't currently in this column, so the caller can window the list
// around it.
func (m *Root) todayLines(width int) (lines []string, focusLine int) {
	focusLine = -1
	focusedCol := m.focus.Column == ColToday

	for gi, g := range m.snap.Today.Groups {
		if gi > 0 {
			lines = append(lines, "")
		}
		if g.Title != "" {
			style := m.theme.BlockTitle
			if focusedCol && m.focus.Block == gi {
				style = m.theme.BlockTitleOn
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s  %d/%d", g.Title, g.Completed, g.Scheduled)))
		}
		for ii, item := range g.Items {
			selected := focusedCol && m.focus.Block == gi && m.focus.Item == ii
			if selected {
				focusLine = len(lines)
			}
			lines = append(lines, m.renderHabitLine(width, item, selected))
		}
	}
	return lines, focusLine
}

func (m *Root) renderHabitLine(width int, item app.HabitItem, selected bool) string {
	check := theme.GlyphPending
	if item.Done {
		check = theme.GlyphDone
	}
	streak := ""
	if item.Streak > 1 {
		streak = fmt.Sprintf(" ·%d", item.Streak)
	}
	line := fmt.Sprintf("%s [%s] %s%s", markerGlyph(selected), check, item.Habit.Title, streak)

	if selected {
		return m.theme.ItemSelected.Render(padTo(line, width))
	}

	check2 := m.theme.CheckPending
	title := m.theme.Text
	if item.Done {
		check2 = m.theme.CheckDone
		title = m.theme.HabitDone
	}
	return fmt.Sprintf("%s [%s] %s%s",
		theme.GlyphUnselected,
		check2.Render(check),
		title.Render(item.Habit.Title),
		m.theme.Dim.Render(streak),
	)
}

// renderDayProgress renders the bottom-of-column progress bar plus the
// completed/scheduled count and percentage.
func (m *Root) renderDayProgress(width int, sum domain.DailySummary) string {
	barWidth := clampInt(width-16, 10, 40)
	pct := sum.Percent()
	filled := 0
	if sum.Scheduled > 0 {
		filled = barWidth * sum.Completed / sum.Scheduled
	}
	if filled > barWidth {
		filled = barWidth
	}
	bar := m.theme.ProgressBarFilled.Render(strings.Repeat(theme.ProgressFilled, filled)) +
		m.theme.ProgressBarEmpty.Render(strings.Repeat(theme.ProgressEmpty, barWidth-filled))

	label := m.theme.ProgressLabel.Render(fmt.Sprintf("%d/%d  %d%%", sum.Completed, sum.Scheduled, pct))
	return bar + "  " + label
}

func markerGlyph(selected bool) string {
	if selected {
		return theme.GlyphSelected
	}
	return theme.GlyphUnselected
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func padTo(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
