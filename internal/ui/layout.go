package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"loopline/internal/theme"
)

// looplineASCII is embedded from Reference/ascii-loopline.txt. Runtime
// rendering never depends on the development-only reference file.
const looplineASCII = "┓      ┓•    \n┃┏┓┏┓┏┓┃┓┏┓┏┓\n┗┗┛┗┛┣┛┗┗┛┗┗ \n     ┛       "

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

	header := m.renderHeader()
	footer := m.renderFooter()
	bodyHeight := max(m.height-lipgloss.Height(header)-lipgloss.Height(footer), 1)
	body := m.renderColumns(bodyHeight)
	return tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
}

// columnWidths gives the execution-heavy Today column modestly more room.
// The 96-column minimum starts at 31/34/29 outer cells; additional width is
// distributed 1:2:1 while all three widths plus the two gaps still sum to
// the terminal width exactly.
func columnWidths(total int) [3]int {
	const gap = 2 // one space between col0/col1 and col1/col2
	avail := total - gap
	if avail < 94 {
		base := max(avail/3, 1)
		w := [3]int{base, base, base}
		for i := base * 3; i < avail; i++ {
			w[i-base*3]++
		}
		return w
	}

	w := [3]int{31, 34, 29}
	extra := avail - 94
	q, r := extra/4, extra%4
	w[0] += q
	w[1] += 2 * q
	w[2] += q
	order := [...]int{1, 0, 2}
	for i := 0; i < r; i++ {
		w[order[i]]++
	}
	return w
}

func (m *Root) renderColumns(bodyHeight int) string {
	outer := columnWidths(m.width)
	innerHeight := max(bodyHeight-2, 1)

	text := [3]int{}
	for i, ow := range outer {
		tw := ow - 4 // border (2) + padding (2)
		if tw < 4 {
			tw = 4
		}
		text[i] = tw
	}

	col0 := m.styledColumn(ColOverview, outer[0], bodyHeight, m.renderOverview(text[0], innerHeight))
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
	if m.panelFocused(col) {
		style = m.theme.ColumnFocused
	}
	style = style.Width(outerWidth).Height(outerHeight).MaxWidth(outerWidth).MaxHeight(outerHeight)
	return style.Render(content)
}

func (m *Root) renderHeader() string {
	nav := m.renderHeaderNavigation(false)
	fullIdentity := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.theme.HeaderBrand.Render(theme.IconLoop),
		"  ",
		m.theme.HeaderBrand.Render(looplineASCII),
	)
	full := lipgloss.JoinHorizontal(lipgloss.Center, fullIdentity, "   ", nav)
	if m.height >= 40 && lipgloss.Width(full)+4 <= m.width {
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, full)
	}

	textIdentity := m.theme.HeaderBrand.Render(theme.IconLoop + "  loopline")
	text := lipgloss.JoinHorizontal(lipgloss.Center, textIdentity, "   ", nav)
	if lipgloss.Width(text)+2 <= m.width {
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, text)
	}

	minimal := lipgloss.JoinHorizontal(
		lipgloss.Center,
		m.theme.HeaderBrand.Render(theme.IconLoop),
		"  ",
		m.renderHeaderNavigation(true),
	)
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, ansi.Truncate(minimal, m.width, ""))
}

func (m *Root) renderHeaderNavigation(compact bool) string {
	labels := [numColumns]string{
		theme.IconOverview + " OVERVIEW",
		theme.IconHabits + " HABITS",
		theme.IconReminders + " REMINDERS",
	}
	if compact {
		labels = [numColumns]string{theme.IconOverview, theme.IconHabits, theme.IconReminders}
	}
	parts := make([]string, 0, numColumns*2-1)
	for col, label := range labels {
		style := m.theme.HeaderItem
		if m.focus.Column == col {
			if m.navigation == NavigationHeader {
				style = m.theme.HeaderSelected
			} else {
				style = m.theme.HeaderActive
			}
		}
		parts = append(parts, style.Render(label))
		if col < numColumns-1 {
			parts = append(parts, m.theme.Dim.Render(" │ "))
		}
	}
	return strings.Join(parts, "")
}

func (m *Root) renderFooter() string {
	if m.status != "" {
		status := m.theme.ReminderOverdue.Render(ansi.Truncate(m.status, m.width, "…"))
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, status)
	}
	type hint struct{ key, description string }
	var hints []hint
	if m.navigation == NavigationHeader {
		hints = []hint{{"tab", "panel"}, {"enter", "open"}, {"?", "help"}, {"q", "quit"}}
	} else {
		hints = []hint{{"↑↓", "item"}}
		if m.primaryActionAvailable() {
			hints = append(hints, hint{"enter", "act"})
		}
		if m.editableSelection().valid() {
			hints = append(hints, hint{"e", "edit"})
		}
		if expansion := m.expansionHint(); expansion != "" {
			hints = append(hints, hint{"t", expansion})
		}
		if m.editableSelection().valid() {
			hints = append(hints, hint{"del", "delete"})
		}
		if m.onCalendar() {
			hints = append(hints, hint{"<>", "month"})
		}
		hints = append(hints, hint{"esc", "menu"}, hint{"?", "help"}, hint{"q", "quit"})
	}

	render := func(items []hint) string {
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = m.theme.FooterKey.Render(item.key) + " " + m.theme.FooterDesc.Render(item.description)
		}
		return strings.Join(parts, "  ")
	}
	for len(hints) > 2 && lipgloss.Width(render(hints)) > m.width {
		// Preserve the navigation pair and Esc; progressively drop the least
		// essential trailing discovery/action hints at constrained widths.
		remove := len(hints) - 2
		if hints[remove].key == "esc" {
			remove--
		}
		hints = append(hints[:remove], hints[remove+1:]...)
	}
	content := ansi.Truncate(render(hints), m.width, "")
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content)
}

func (m *Root) primaryActionAvailable() bool {
	if m.navigation != NavigationPanel {
		return false
	}
	switch m.focus.Column {
	case ColToday:
		return m.focus.Block == TodayActions || m.editableSelection().valid()
	case ColReminders:
		return m.focus.Block == RemindersActions ||
			(m.focus.Block == RemindersList && m.editableSelection().valid())
	default:
		return false
	}
}

func (m *Root) renderMinSize() string {
	msg := fmt.Sprintf(
		"loopline needs a little more room.\nNeed at least %d x %d, have %d x %d.",
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
		m.theme.Dim.Render("loopline · loading…"),
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
