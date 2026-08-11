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
	listHeight := max(height-todayChrome, 1)

	body := m.theme.Muted.Render("No habits scheduled today.")
	if lines, focusLine := m.todayLines(width); len(lines) > 0 {
		body = strings.Join(windowAround(lines, listHeight, focusLine), "\n")
	}
	// Pad the list out to its full budget so the progress bar always sits at
	// the bottom of the column (SPEC Column 2: "At the bottom: daily
	// completion progress bar...") instead of trailing right under a short
	// habit list with a gap styledColumn's own padding leaves beneath it.
	body = padLinesTo(body, listHeight)

	return title + "\n\n" + body + "\n\n" + progress
}

// todayLines flattens every habit group into one *physical* line per row (an
// optional group title, then one or more wrapped lines per habit), plus a
// blank separator between groups. Every entry is pre-wrapped to width via
// wrapPhysical so the caller windows on real screen rows, not logical items
// — see wrapPhysical's doc comment. It also returns the absolute index of
// the first physical line of the focused item, or -1 if focus isn't
// currently in this column, so the caller can window the list around it.
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
			title := style.Render(fmt.Sprintf("%s  %d/%d", g.Title, g.Completed, g.Scheduled))
			lines = append(lines, wrapPhysical(title, width)...)
		}
		for ii, item := range g.Items {
			selected := focusedCol && m.focus.Block == gi && m.focus.Item == ii
			itemLines := m.renderHabitLines(width, item, selected)
			if selected {
				focusLine = len(lines)
			}
			lines = append(lines, itemLines...)
		}
	}
	return lines, focusLine
}

// renderHabitLines renders one habit row and wraps it to width, returning
// every physical line it occupies. A selected item is padded and reversed
// per physical line (rather than once over the whole, possibly multi-line,
// row) so the highlight covers the full width of every wrapped line, not
// just the first.
func (m *Root) renderHabitLines(width int, item app.HabitItem, selected bool) []string {
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
		rows := wrapPhysical(line, width)
		out := make([]string, len(rows))
		for i, row := range rows {
			out[i] = m.theme.ItemSelected.Render(padTo(row, width))
		}
		return out
	}

	check2 := m.theme.CheckPending
	title := m.theme.Text
	if item.Done {
		check2 = m.theme.CheckDone
		title = m.theme.HabitDone
	}
	composite := fmt.Sprintf("%s [%s] %s%s",
		theme.GlyphUnselected,
		check2.Render(check),
		title.Render(item.Habit.Title),
		m.theme.Dim.Render(streak),
	)
	return wrapPhysical(composite, width)
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

// padLinesTo appends blank lines to body until it has exactly height lines
// (a no-op if it already has height or more). Used to pin content that
// follows a variable-length list — such as Today's progress bar — to the
// bottom of a fixed-height column instead of trailing right beneath a short
// list.
func padLinesTo(body string, height int) string {
	lines := strings.Split(body, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
