package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

// View implements tea.Model.
func (m *Root) View() tea.View {
	if m.width < MinWidth || m.height < MinHeight {
		return tea.NewView(m.renderMinSize())
	}
	if m.modal != nil {
		return tea.NewView(m.renderModal())
	}
	if m.help {
		return tea.NewView(m.renderHelp())
	}
	if !m.ready {
		if m.status != "" {
			return tea.NewView(m.renderStartupError())
		}
		return tea.NewView(m.renderLoading())
	}

	body := m.renderColumns()
	footer := m.renderFooter()
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, footer))
}

// columnWidths splits the terminal width into three outer column widths
// (border included), separated by single-space gaps, that exactly sum back
// to total.
func columnWidths(total int) [3]int {
	const gap = 2 // one space between col0/col1 and col1/col2
	avail := total - gap
	base := avail / 3
	rem := avail % 3
	w := [3]int{base, base, base}
	for i := 0; i < rem; i++ {
		w[2-i]++
	}
	return w
}

func (m *Root) renderColumns() string {
	outer := columnWidths(m.width)
	bodyHeight := m.height - 2 // footer: hints line + status line
	innerHeight := max(bodyHeight-2, 1)

	text := [3]int{}
	for i, ow := range outer {
		tw := ow - 4 // border (2) + padding (2)
		if tw < 4 {
			tw = 4
		}
		text[i] = tw
	}

	col0 := m.styledColumn(ColOverview, outer[0], bodyHeight, m.renderOverview(text[0]))
	col1 := m.styledColumn(ColToday, outer[1], bodyHeight, m.renderToday(text[1], innerHeight))
	col2 := m.styledColumn(ColReminders, outer[2], bodyHeight, m.renderReminders(text[2], innerHeight))

	return lipgloss.JoinHorizontal(lipgloss.Top, col0, " ", col1, " ", col2)
}

// styledColumn wraps a column's content with its border, sized and clamped
// so it can never overflow the terminal even if the content is too tall or
// wide.
//
// Width and Height are the TOTAL box dimensions in Lip Gloss v2 — border and
// padding included — so they take outerWidth/outerHeight directly. Passing
// outerWidth-2 here (as if Width sized only the interior) made every column
// render two cells narrower than the width renderColumns had already handed
// its renderers, so content that was pre-wrapped to fit got wrapped a second
// time; the extra physical lines then overflowed the height budget and
// MaxHeight clipped the bottom of the column, taking the day progress bar
// and the closing border with it.
//
// The interior these dimensions leave is outerWidth-4 by outerHeight-2
// (border 2 + horizontal padding 2; padding is 0 vertically), which is
// exactly the text width and inner height renderColumns computes.
func (m *Root) styledColumn(col, outerWidth, outerHeight int, content string) string {
	style := m.theme.Column
	if m.focus.Column == col {
		style = m.theme.ColumnFocused
	}
	style = style.Width(outerWidth).Height(outerHeight).MaxWidth(outerWidth).MaxHeight(outerHeight)
	return style.Render(content)
}

func (m *Root) renderFooter() string {
	hints := "←→/h l column   tab block   ↑↓/j k item   enter act   </> month·range   ?  help   q  quit"
	line1 := m.theme.Footer.Render(hints)
	line2 := ""
	if m.status != "" {
		line2 = m.theme.ReminderOverdue.Render(m.status)
	}
	return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
}

func (m *Root) renderMinSize() string {
	msg := fmt.Sprintf(
		"Terminal too small.\nNeed at least %d x %d, have %d x %d.",
		MinWidth, MinHeight, m.width, m.height,
	)
	return lipgloss.Place(
		max(m.width, 1), max(m.height, 1),
		lipgloss.Center, lipgloss.Center,
		m.theme.MinSizeMessage.Render(msg),
	)
}

func (m *Root) renderLoading() string {
	return lipgloss.Place(
		max(m.width, 1), max(m.height, 1),
		lipgloss.Center, lipgloss.Center,
		m.theme.Dim.Render("Loading…"),
	)
}

// renderStartupError is shown instead of renderLoading when the very first
// Snapshot fetch fails (m.status set, m.ready still false): otherwise the
// app would be stuck on a permanent "Loading…" with the real error never
// displayed anywhere. Reuses ReminderOverdue, the same style renderFooter
// already uses for the status line's error case.
func (m *Root) renderStartupError() string {
	return lipgloss.Place(
		max(m.width, 1), max(m.height, 1),
		lipgloss.Center, lipgloss.Center,
		m.theme.ReminderOverdue.Render(m.status),
	)
}

func (m *Root) renderModal() string {
	style := m.theme.ModalBorder.MaxWidth(m.width).MaxHeight(m.height)
	box := style.Render(m.modal.form.View())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
