package ui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

type helpEntry struct{ key, desc string }

var helpEntries = []helpEntry{
	{"Tab / Shift+Tab", "next / previous panel in the header"},
	{"Enter", "enter panel / act / toggle / confirm"},
	{"Up/Down, j/k", "move through panel items"},
	{"e", "edit selected habit, routine or reminder"},
	{"t", "expand / collapse routine or reminder"},
	{"Delete", "delete selected entity with confirmation"},
	{"Left / Right", "older / newer weeks in Completion"},
	{"< / > ([ / ])", "previous / next month on a calendar"},
	{"Esc", "close modal/help or return to the header"},
	{"?", "toggle this help"},
	{"q", "quit"},
}

// renderHelp is a full-screen overlay: simpler and more robust than true
// layered compositing, and still satisfies "esc closes it".
func (m *Root) renderHelp() string {
	var b strings.Builder
	b.WriteString(m.theme.ModalTitle.Render("loopline keys"))
	b.WriteString("\n\n")
	for _, e := range helpEntries {
		b.WriteString(m.theme.HelpKey.Render(padTo(e.key, 20)))
		b.WriteString(m.theme.HelpDesc.Render(e.desc))
		b.WriteString("\n")
	}
	box := m.theme.ModalBorder.Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
