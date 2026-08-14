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

const (
	// At 120 cells the natural-width identity rail plus the three content
	// panels retain 28/35/34 outer cells for Habits/Reminders/Overview. Below this
	// breakpoint the established three-panel layout is more useful than
	// squeezing in a fourth region.
	wideLayoutMinWidth     = 120
	wideLayoutGaps         = 3 // one cell between each of four regions
	identityRailExtraWidth = 3 // modest breathing room beyond the logo's natural frame
)

// View implements tea.Model.
func (m *Root) View() tea.View {
	if m.width < MinWidth || m.height < MinHeight {
		return newRootView(m.renderMinSize())
	}
	if m.modal != nil {
		return newRootView(m.renderModal())
	}
	if m.help {
		return newRootView(m.renderHelp())
	}
	if !m.ready {
		if m.status != "" {
			return newRootView(m.renderStartupError())
		}
		return newRootView(m.renderLoading())
	}

	footer := m.renderFooter()
	if m.usesIdentityRail() {
		bodyHeight := max(m.height-lipgloss.Height(footer), 1)
		body := m.renderWideColumns(bodyHeight)
		return newRootView(lipgloss.JoinVertical(lipgloss.Left, body, footer))
	}

	header := m.renderHeader()
	bodyHeight := max(m.height-lipgloss.Height(header)-lipgloss.Height(footer), 1)
	body := m.renderColumns(bodyHeight)
	return newRootView(lipgloss.JoinVertical(lipgloss.Left, header, body, footer))
}

// newRootView establishes the terminal contract shared by every UI state.
// loopline is a terminal-sized dashboard, not an inline prompt: the alternate
// screen keeps redraws out of normal scrollback, while cell-motion mouse mode
// lets wheel input reach the focused internal history viewport.
func newRootView(content string) tea.View {
	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
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

func (m *Root) usesIdentityRail() bool {
	return m.width >= wideLayoutMinWidth
}

// wideColumnWidths returns identity, Habits, Reminders and Overview outer
// widths. The identity rail is measured from the ASCII mark and never grows;
// the three content panels start at their proven compact minima. Additional
// space is weighted 1:2:2 so short habit rows stay compact while Reminders and
// Overview gain useful data width.
func (m *Root) wideColumnWidths() [4]int {
	identity := lipgloss.Width(looplineASCII) + m.theme.Column.GetHorizontalFrameSize() + identityRailExtraWidth
	panelSpace := m.width - identity - wideLayoutGaps
	panels := [3]int{28, 35, 34}
	extra := max(panelSpace-(panels[0]+panels[1]+panels[2]), 0)
	habitsExtra := extra / 5
	remindersExtra := extra * 2 / 5
	panels[0] += habitsExtra
	panels[1] += remindersExtra
	panels[2] += extra - habitsExtra - remindersExtra
	return [4]int{identity, panels[0], panels[1], panels[2]}
}

func (m *Root) panelColumnWidths() [3]int {
	if !m.usesIdentityRail() {
		return columnWidths(m.width)
	}
	wide := m.wideColumnWidths()
	return [3]int{wide[3], wide[1], wide[2]}
}

func (m *Root) renderColumns(bodyHeight int) string {
	outer := columnWidths(m.width)
	innerHeight := max(bodyHeight-m.theme.Column.GetVerticalFrameSize(), 1)

	text := [3]int{}
	for i, ow := range outer {
		tw := ow - m.theme.Column.GetHorizontalFrameSize()
		if tw < 4 {
			tw = 4
		}
		text[i] = tw
	}

	habits := m.styledColumn(ColToday, outer[ColToday], bodyHeight, m.renderToday(text[ColToday], innerHeight))
	reminders := m.styledColumn(ColReminders, outer[ColReminders], bodyHeight, m.renderReminders(text[ColReminders], innerHeight))
	overview := m.styledColumn(ColOverview, outer[ColOverview], bodyHeight, m.renderOverview(text[ColOverview], innerHeight))

	return lipgloss.JoinHorizontal(lipgloss.Top, habits, " ", reminders, " ", overview)
}

func (m *Root) renderWideColumns(bodyHeight int) string {
	outer := m.wideColumnWidths()
	innerHeight := max(bodyHeight-m.theme.Column.GetVerticalFrameSize(), 1)
	contentHeight := max(innerHeight-1, 1) // one persistent panel-title row

	text := [3]int{}
	for i, ow := range outer[1:] {
		text[i] = max(ow-m.theme.Column.GetHorizontalFrameSize(), 4)
	}

	identity := m.renderIdentityRail(outer[0], bodyHeight)
	habits := m.renderWidePanel(ColToday, "Habits", outer[1], bodyHeight, text[0], m.renderToday(text[0], contentHeight))
	reminders := m.renderWidePanel(ColReminders, "Reminders", outer[2], bodyHeight, text[1], m.renderReminders(text[1], contentHeight))
	overview := m.renderWidePanel(ColOverview, "Overview", outer[3], bodyHeight, text[2], m.renderOverview(text[2], contentHeight))

	return lipgloss.JoinHorizontal(lipgloss.Top, identity, " ", habits, " ", reminders, " ", overview)
}

func (m *Root) renderWidePanel(col int, title string, outerWidth, outerHeight, textWidth int, content string) string {
	heading := lipgloss.PlaceHorizontal(textWidth, lipgloss.Center, m.theme.HeaderItem.Render(title))
	return m.styledColumn(col, outerWidth, outerHeight, heading+"\n"+content)
}

func (m *Root) renderIdentityRail(outerWidth, outerHeight int) string {
	style := m.theme.Column.Width(outerWidth).Height(outerHeight).MaxWidth(outerWidth).MaxHeight(outerHeight)
	innerWidth := max(outerWidth-m.theme.Column.GetHorizontalFrameSize(), 1)
	logo := lipgloss.PlaceHorizontal(innerWidth, lipgloss.Center, m.theme.HeaderBrand.Render(looplineASCII))
	content := lipgloss.JoinVertical(lipgloss.Left, logo, "", m.renderIdentityNavigation(innerWidth))
	return style.Render(content)
}

func (m *Root) renderIdentityNavigation(width int) string {
	labels := [numColumns]string{
		theme.IconOverview + " OVERVIEW",
		theme.IconHabits + " HABITS",
		theme.IconReminders + " REMINDERS",
	}
	lines := make([]string, 0, len(labels))
	for _, col := range globalColumnOrder {
		lines = append(lines, lipgloss.PlaceHorizontal(width, lipgloss.Left, m.headerNavigationStyle(col).Render(labels[col])))
	}
	return strings.Join(lines, "\n")
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
	textIdentity := m.theme.HeaderBrand.Render(theme.IconLoop + "  loopline")
	text := lipgloss.JoinHorizontal(lipgloss.Center, textIdentity, "   ", nav)
	if m.fullHeaderHasRoom(full, text) && lipgloss.Width(full)+4 <= m.width {
		return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, full)
	}
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

// fullHeaderHasRoom compares actual rendered heights and the real column
// frame. The full logo is allowed only when it leaves the same usable panel
// content height that the compact header leaves at the supported minimum.
func (m *Root) fullHeaderHasRoom(full, compact string) bool {
	footerHeight := lipgloss.Height(m.renderFooter())
	frameHeight := m.theme.Column.GetVerticalFrameSize()
	minimumContentHeight := MinHeight - lipgloss.Height(compact) - footerHeight - frameHeight
	availableContentHeight := m.height - lipgloss.Height(full) - footerHeight - frameHeight
	return availableContentHeight >= minimumContentHeight
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
	for position, col := range globalColumnOrder {
		parts = append(parts, m.headerNavigationStyle(col).Render(labels[col]))
		if position < len(globalColumnOrder)-1 {
			parts = append(parts, m.theme.Dim.Render(" │ "))
		}
	}
	return strings.Join(parts, "")
}

func (m *Root) headerNavigationStyle(col int) lipgloss.Style {
	if m.focus.Column != col {
		return m.theme.HeaderItem
	}
	if m.navigation == NavigationHeader {
		return m.theme.HeaderSelected
	}
	return m.theme.HeaderActive
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
		if m.onCompletionTimeline() {
			hints = append(hints, hint{"←→", "history"})
		} else if m.onCalendar() {
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
