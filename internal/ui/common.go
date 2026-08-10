package ui

import (
	"fmt"

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
