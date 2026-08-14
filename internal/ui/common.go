package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

// columnTitle renders a column header, emphasised when that column holds
// focus. label is the column-specific text (may include live data such as
// today's date).
func (m *Root) columnTitle(col int, label string) string {
	style := m.theme.ColumnTitle
	if m.focus.Column == col {
		style = m.theme.ColumnTitleOn
	}
	return style.Render(label)
}

// renderActionLine renders a single-item "button" row, such as "+ Add habit".
func (m *Root) renderActionLine(label string, selected bool) string {
	text := fmt.Sprintf("%s + %s", markerGlyph(selected), label)
	if selected {
		return m.theme.ItemSelected.Render(text)
	}
	return m.theme.Item.Render(text)
}

// weekdayLabel renders a date as "Mon, Jan 2".
func weekdayLabel(d domain.Date) string {
	return d.Time().Format("Mon, Jan 2")
}

// shortDate renders a date as "Jan 2".
func shortDate(d domain.Date) string {
	return d.Time().Format("Jan 2")
}

func centeredLine(s string, width int) string {
	left := max((width-lipgloss.Width(s))/2, 0)
	return strings.Repeat(" ", left) + s
}

func calendarCell(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// calendarGridRow lays out a seven-cell calendar row with responsive one- or
// two-cell gaps and centers the resulting grid in the available column. Both
// calendars use it so weekday headers and dates always share exact columns.
func calendarGridRow(width, cellWidth int, cells []string) string {
	if len(cells) == 0 {
		return ""
	}
	gap := 0
	if len(cells) > 1 {
		gap = clampInt((width-cellWidth*len(cells))/(len(cells)-1), 1, 2)
	}
	gridWidth := cellWidth*len(cells) + gap*(len(cells)-1)
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = calendarCell(cell, cellWidth)
	}
	return strings.Repeat(" ", max((width-gridWidth)/2, 0)) + strings.Join(parts, strings.Repeat(" ", gap))
}

// renderBar is the shared fixed-width progress primitive. Callers choose the
// semantic style and glyph vocabulary; the fill math remains identical.
func renderBar(width, numerator, denominator int, filled, empty lipgloss.Style, filledGlyph, emptyGlyph string) string {
	fill := 0
	if denominator > 0 {
		fill = width * numerator / denominator
	}
	fill = clampInt(fill, 0, width)
	return filled.Render(strings.Repeat(filledGlyph, fill)) +
		empty.Render(strings.Repeat(emptyGlyph, width-fill))
}

// windowAround returns the slice of lines that fits within height, keeping
// the line at focus visible (centred when there's room on both sides,
// clamped at either end of lines otherwise). focus < 0 means "no anchor":
// the window starts at the top. Used to keep a long habit or reminder list
// from silently pushing whatever comes after it (a progress bar, a notes
// panel) off the bottom of a fixed-height column.
func windowAround(lines []string, height, focus int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	start := 0
	if focus >= 0 {
		start = focus - height/2
	}
	if start < 0 {
		start = 0
	}
	if start > len(lines)-height {
		start = len(lines) - height
	}
	return lines[start : start+height]
}

// windowAroundRange is windowAround for a multi-line selected item. It keeps
// the full half-open [focusStart, focusEnd) span visible whenever that span
// fits; an oversized expansion starts at its selected row and is clipped only
// at the viewport boundary.
func windowAroundRange(lines []string, height, focusStart, focusEnd int) []string {
	if focusStart < 0 || focusEnd <= focusStart {
		return windowAround(lines, height, -1)
	}
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return lines
	}
	span := focusEnd - focusStart
	start := focusStart
	if span < height {
		start -= (height - span) / 2
	}
	start = clampInt(start, 0, len(lines)-height)
	if span <= height && start+height < focusEnd {
		start = focusEnd - height
	}
	return lines[start : start+height]
}

// wrapPhysical splits s (which may already carry ANSI styling) into the
// individual physical lines it occupies once rendered at width, using the
// same ANSI-aware word-wrap Lip Gloss applies whenever a Style.Width()
// renders a too-long line (falling back to a hard character break for a
// single word wider than width). Height budgeting must measure wrapped
// height at the real usable column width, not logical string count (see
// docs/ARCHITECTURE.md): callers building a line-per-row slice for
// windowAround pre-wrap every item through this so the slice they hand to
// windowAround already has one entry per physical row, and the outer column
// framing's own Width() never needs to wrap anything further.
// flattenWhitespace collapses every run of whitespace — newlines included —
// into a single space, so free-text a user typed into a multiline form field
// can be rendered on a single row that a one-line height budget can hold.
func flattenWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func wrapPhysical(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	return strings.Split(lipgloss.NewStyle().Width(width).Render(s), "\n")
}
