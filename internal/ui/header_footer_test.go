package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
	"loopline/internal/theme"
)

func TestResponsiveHeaderUsesExactASCIIThenTextThenMinimalIdentity(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.width, root.height = wideLayoutMinWidth-1, 40
	full := stripANSI(root.renderHeader())
	for _, logoLine := range strings.Split(looplineASCII, "\n") {
		if !strings.Contains(full, strings.TrimRight(logoLine, " ")) {
			t.Fatalf("comfortable header is missing exact ASCII logo row %q:\n%s", logoLine, full)
		}
	}
	for _, want := range []string{theme.IconOverview + " OVERVIEW", theme.IconHabits + " HABITS", theme.IconReminders + " REMINDERS"} {
		if !strings.Contains(full, want) {
			t.Fatalf("full header is missing %q: %q", want, full)
		}
	}
	habitsAt := strings.Index(full, "HABITS")
	remindersAt := strings.Index(full, "REMINDERS")
	overviewAt := strings.Index(full, "OVERVIEW")
	if habitsAt < 0 || !(habitsAt < remindersAt && remindersAt < overviewAt) {
		t.Fatalf("compact header order is not Habits → Reminders → Overview: %q", full)
	}

	root.width, root.height = MinWidth, MinHeight
	text := stripANSI(root.renderHeader())
	if strings.Contains(text, "┓      ┓•") || !strings.Contains(text, "loopline") {
		t.Fatalf("low-height header did not use textual fallback: %q", text)
	}

	root.width, root.height = 24, 50
	minimal := root.renderHeader()
	if lipgloss.Width(minimal) > root.width || strings.Contains(stripANSI(minimal), "loopline") {
		t.Fatalf("minimal header fallback overflows or retained text branding: %q", stripANSI(minimal))
	}
}

func TestASCIIHeaderBreakpointUsesMeasuredContentAndFrameBudget(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.width = wideLayoutMinWidth - 1

	root.height = 35
	compact := root.renderHeader()
	if strings.Contains(stripANSI(compact), "┓      ┓•") || lipgloss.Height(compact) != 1 {
		t.Fatalf("height 35 should retain the one-line fallback: %q", stripANSI(compact))
	}

	root.height = 36
	full := root.renderHeader()
	if !strings.Contains(stripANSI(full), "┓      ┓•") || lipgloss.Height(full) != lipgloss.Height(looplineASCII) {
		t.Fatalf("height 36 should render the natural ASCII mark: %q", stripANSI(full))
	}

	footerHeight := lipgloss.Height(root.renderFooter())
	frameHeight := root.theme.Column.GetVerticalFrameSize()
	minimumContent := MinHeight - lipgloss.Height(compact) - footerHeight - frameHeight
	availableContent := root.height - lipgloss.Height(full) - footerHeight - frameHeight
	if availableContent != minimumContent {
		t.Fatalf("breakpoint content budget = %d, want the supported minimum %d (footer=%d frame=%d)", availableContent, minimumContent, footerHeight, frameHeight)
	}

	content := root.View().Content
	if got := lipgloss.Height(content); got > root.height {
		t.Fatalf("full-logo view height = %d, terminal height = %d", got, root.height)
	}
	for _, want := range []string{"Completion", "Calendar", "Year"} {
		if !strings.Contains(stripANSI(content), want) {
			t.Fatalf("full-logo breakpoint clipped %q from the body:\n%s", want, stripANSI(content))
		}
	}
}

func TestRootViewsAlwaysUseStableFullscreenCanvas(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newRichBackend(today)

	assertFullscreen := func(name string, root *Root) {
		t.Helper()
		view := root.View()
		if !view.AltScreen {
			t.Errorf("%s view did not request Bubble Tea's alternate screen", name)
		}
		if view.MouseMode != tea.MouseModeCellMotion {
			t.Errorf("%s view mouse mode = %d, want cell motion for internal wheel routing", name, view.MouseMode)
		}
	}

	loading := New(backend)
	loading.width, loading.height = 120, 40
	assertFullscreen("loading", loading)

	startupError := New(backend)
	startupError.width, startupError.height, startupError.status = 120, 40, "load failed"
	assertFullscreen("startup error", startupError)

	root := newReadyRoot(t, backend)
	root.width, root.height = wideLayoutMinWidth-1, 35
	assertFullscreen("compact header", root)
	root.width = wideLayoutMinWidth
	assertFullscreen("identity rail", root)
	root.width, root.height = wideLayoutMinWidth-1, 36
	assertFullscreen("ASCII header", root)

	root.width, root.height = 80, 20
	assertFullscreen("minimum-size fallback", root)
	root.width, root.height = 120, 40
	root.help = true
	assertFullscreen("help", root)
	root.help = false
	root.modal = newAddRoutineForm(backend)
	assertFullscreen("modal", root)
}

func TestHeaderSelectionTreatmentTracksNavigationLevel(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	if !root.theme.HeaderSelected.GetReverse() {
		t.Fatal("header-focused entry lacks a terminal-safe reverse focus treatment")
	}
	root = pressKey(t, root, "enter")
	if root.navigation != NavigationPanel || !root.theme.HeaderActive.GetUnderline() {
		t.Fatal("panel mode lacks a distinct active-header treatment")
	}
}

func TestFooterIsCenteredOneLineAndContextAware(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.width = 120
	headerFooter := root.renderFooter()
	plain := stripANSI(headerFooter)
	if lipgloss.Height(headerFooter) != 1 || lipgloss.Width(headerFooter) != root.width {
		t.Fatalf("header footer dimensions = %dx%d, want %dx1", lipgloss.Width(headerFooter), lipgloss.Height(headerFooter), root.width)
	}
	trimmed := strings.TrimSpace(plain)
	left := len(plain) - len(strings.TrimLeft(plain, " "))
	right := len(plain) - len(strings.TrimRight(plain, " "))
	if delta := left - right; delta < -1 || delta > 1 {
		t.Fatalf("footer group is not centered: left=%d right=%d content=%q", left, right, trimmed)
	}
	flat := flattenWhitespace(plain)
	if !strings.Contains(flat, "tab panel") || strings.Contains(flat, "↑↓ item") {
		t.Fatalf("header-mode footer is not contextual: %q", flat)
	}

	setPanelFocus(root, Focus{Column: ColToday, Block: 2, Item: 0})
	panel := flattenWhitespace(stripANSI(root.renderFooter()))
	for _, want := range []string{"↑↓ item", "enter act", "e edit", "t collapse", "del delete", "esc menu"} {
		if !strings.Contains(panel, want) {
			t.Fatalf("panel footer missing %q: %q", want, panel)
		}
	}
	if strings.Contains(panel, "tab panel") {
		t.Fatalf("panel footer advertises unavailable header navigation: %q", panel)
	}
	if !root.theme.FooterKey.GetReverse() || root.theme.FooterKey.GetHorizontalPadding() == 0 {
		t.Fatal("footer keys do not render as compact padded keycaps")
	}
}

func TestFooterOmitsUnavailablePrimaryAction(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	root.width = MinWidth
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewLifetime, Item: 0})
	footer := flattenWhitespace(stripANSI(root.renderFooter()))
	if strings.Contains(footer, "enter act") {
		t.Fatalf("non-actionable Lifetime selection advertises Enter: %q", footer)
	}
	setPanelFocus(root, Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0})
	footer = flattenWhitespace(stripANSI(root.renderFooter()))
	if strings.Contains(footer, "<> month") || !strings.Contains(footer, "←→ history") {
		t.Fatalf("Completion footer did not expose only week history navigation: %q", footer)
	}

	setPanelFocus(root, Focus{Column: ColReminders, Block: RemindersCalendar, Item: 0})
	footer = flattenWhitespace(stripANSI(root.renderFooter()))
	if strings.Contains(footer, "enter act") || !strings.Contains(footer, "<> month") {
		t.Fatalf("Calendar footer did not expose only its available action: %q", footer)
	}
}

func TestInteriorPanelsStartWithContentRatherThanGlobalTitles(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	if first := strings.Split(stripANSI(root.renderOverview(27, 29)), "\n")[0]; !strings.Contains(first, "Lifetime") {
		t.Fatalf("Overview interior starts with %q, want Lifetime", first)
	}
	if first := strings.Split(stripANSI(root.renderToday(30, 29)), "\n")[0]; first != "Add" {
		t.Fatalf("Habits interior starts with %q, want Add", first)
	}
	if first := strings.Split(stripANSI(root.renderReminders(25, 29)), "\n")[0]; !strings.Contains(first, "Add reminder") {
		t.Fatalf("Reminders interior starts with %q, want Add reminder", first)
	}
}
