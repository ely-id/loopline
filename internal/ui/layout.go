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
func (m *Root) styledColumn(col, outerWidth, outerHeight int, content string) string {
	style := m.theme.Column
	if m.focus.Column == col {
		style = m.theme.ColumnFocused
	}
	contentWidth := max(outerWidth-2, 0)
	contentHeight := max(outerHeight-2, 0)
	style = style.Width(contentWidth).Height(contentHeight).MaxWidth(outerWidth).MaxHeight(outerHeight)
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

func (m *Root) renderModal() string {
	style := m.theme.ModalBorder.MaxWidth(m.width).MaxHeight(m.height)
	box := style.Render(m.modal.form.View())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
