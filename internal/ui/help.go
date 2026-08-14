package ui

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

type helpEntry struct{ key, desc string }

var helpEntries = []helpEntry{
	{"Left/Right, h/l", "switch column"},
	{"Tab / Shift+Tab", "switch block"},
	{"Up/Down, j/k", "move selection"},
	{"Enter", "act / toggle / confirm"},
	{"e", "edit selected habit, routine or reminder"},
	{"t", "expand / collapse routine or reminder"},
	{"Delete", "delete selected entity with confirmation"},
	{"< / > ([ / ])", "previous / next month on a calendar"},
	{"Esc", "close modal or help"},
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
