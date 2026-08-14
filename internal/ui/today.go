package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// todayChrome is the fixed content around the habit viewport: compact Add
// block, section spacing, and the bottom-pinned progress bar.
const todayChrome = 6

// renderToday builds the whole "today" column: title, one visually separated
// block per habit group (windowed to fit height so the focused habit is
// always visible), then the day's progress bar pinned to the bottom.
func (m *Root) renderToday(width, height int) string {
	progress := m.renderDayProgress(width, m.snap.Today.Summary)
	listHeight := max(height-todayChrome, 1)

	body := m.theme.Muted.Render(emptyScheduleMarker)
	if lines, focusLine := m.todayLines(width); len(lines) > 0 {
		body = strings.Join(windowAround(lines, listHeight, focusLine), "\n")
	}
	// Pad the list out to its full budget so the progress bar always sits at
	// the bottom of the Habits panel (SPEC: "At the bottom: daily
	// completion progress bar...") instead of trailing right under a short
	// habit list with a gap styledColumn's own padding leaves beneath it.
	body = padLinesTo(body, listHeight)

	return m.renderTodayActions() + "\n\n" + body + "\n\n" + progress
}

func (m *Root) renderTodayActions() string {
	focused := m.panelFocused(ColToday) && m.focus.Block == TodayActions
	title := m.theme.BlockTitle
	if focused {
		title = m.theme.BlockTitleOn
	}
	return strings.Join([]string{
		title.Render("Add"),
		m.renderActionLine("Habit", focused && m.focus.Item == ActionAddHabit),
		m.renderActionLine("Routine", focused && m.focus.Item == ActionAddRoutine),
	}, "\n")
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
	focusedCol := m.panelFocused(ColToday)

	for gi, g := range m.snap.Today.Groups {
		if gi > 0 {
			lines = append(lines, "")
		}
		block := gi + 1 // block 0 is the Add actions block
		if g.Title != "" {
			style := m.theme.BlockTitle
			if focusedCol && m.focus.Block == block {
				style = m.theme.BlockTitleOn
			}
			prefix := ""
			if g.RoutineID != nil {
				prefix = theme.GlyphExpanded + " "
				if m.routineCollapsed(g) {
					prefix = theme.GlyphCollapsed + " "
				}
			}
			selectedHeader := focusedCol && m.focus.Block == block && g.RoutineID != nil && m.focus.Item == 0
			titleText := fmt.Sprintf("%s%s  %d/%d", prefix, g.Title, g.Completed, g.Scheduled)
			title := style.Render(titleText)
			if selectedHeader {
				title = m.theme.ItemSelected.Render(padTo(titleText, width))
				focusLine = len(lines)
			}
			lines = append(lines, wrapPhysical(title, width)...)
		}
		if m.routineCollapsed(g) {
			continue
		}
		for ii, item := range g.Items {
			focusItem := ii
			if g.RoutineID != nil {
				focusItem++ // item 0 is the routine header
			}
			selected := focusedCol && m.focus.Block == block && m.focus.Item == focusItem
			itemLines := m.renderHabitLines(width, item, selected)
			if selected {
				focusLine = len(lines)
			}
			lines = append(lines, itemLines...)
		}
	}
	return lines, focusLine
}

func (m *Root) routineCollapsed(group app.HabitGroup) bool {
	return group.RoutineID != nil && m.collapsedRoutines[*group.RoutineID]
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
	stage := completionStage(sum)
	stageStyle := m.theme.CompletionStyles[stage]
	bar := renderBar(barWidth, sum.Completed, sum.Scheduled, stageStyle, m.theme.ProgressBarEmpty, theme.ProgressFilled, theme.ProgressEmpty)

	label := m.theme.ProgressLabel.Inherit(stageStyle).Render(fmt.Sprintf("%d/%d  %d%%", sum.Completed, sum.Scheduled, pct))
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
