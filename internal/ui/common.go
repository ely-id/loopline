package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"routine-tui/internal/domain"
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
