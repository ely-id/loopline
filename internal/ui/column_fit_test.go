package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

// assertViewFits renders the full composed view and checks the invariants a
// three-column TUI must never break: it fits the terminal exactly, no row is
// wider than the terminal, and all three columns close their border.
func assertViewFits(t *testing.T, m *Root, w, h int, ctx string) string {
	t.Helper()
	out := stripANSI(m.View().Content)
	lines := strings.Split(out, "\n")
	if len(lines) > h {
		t.Fatalf("%s: view is %d rows tall, terminal is %d", ctx, len(lines), h)
	}
	for i, l := range lines {
		if lw := lipgloss.Width(l); lw > w {
			t.Fatalf("%s: row %d is %d cells wide, terminal is %d: %q", ctx, i, lw, w, l)
		}
	}
	if got := strings.Count(out, "└"); got != 3 {
		t.Fatalf("%s: %d columns closed their bottom border, want 3 (the column overflowed and was clipped):\n%s", ctx, got, out)
	}
	return out
}

// TestColumnContentNeverOutgrowsItsBorder is the regression test for a real
// blocker: styledColumn used to pass outerWidth-2 to Style.Width, but in Lip
// Gloss v2 Width is the TOTAL box width (border and padding included), so
// every column rendered two cells narrower than the width renderColumns had
// already handed its renderers. Content pre-wrapped to fit was then wrapped a
// second time, the extra physical lines overflowed the height budget, and
// MaxHeight clipped the bottom of the column away — taking the day progress
// bar and the column's own bottom border with it.
//
// The sweep matters: the failure only appears at title lengths whose second
// wrap crosses the budget, so spot-checking one length hides it.
func TestColumnContentNeverOutgrowsItsBorder(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")

	for _, dim := range [][2]int{{MinWidth, MinHeight}, {100, 36}, {120, 40}} {
		w, h := dim[0], dim[1]
		for titleLen := 1; titleLen <= 70; titleLen++ {
			backend := newFakeBackend(today)
			for i := 0; i < 15; i++ {
				backend.habits = append(backend.habits, domain.Habit{
					ID:        int64(i + 1),
					Title:     strings.Repeat("a", titleLen),
					Schedule:  domain.DailySchedule(),
					CreatedOn: today.AddDays(-30),
				})
			}
			m := New(backend)
			m = send(t, m, tea.WindowSizeMsg{Width: w, Height: h})
			m = step(t, m, m.Init())

			ctx := fmt.Sprintf("%dx%d titleLen=%d", w, h, titleLen)
			out := assertViewFits(t, m, w, h, ctx)

			// SPEC Column 2 requires the progress bar; clipping used to eat it.
			if !strings.Contains(out, "/15") {
				t.Fatalf("%s: day progress bar (\"N/15\") missing from the view:\n%s", ctx, out)
			}
		}
	}
}

// TestExpandedMultilineReminderNoteStaysWithinColumn is the regression guard
// for expanded free text: the full note is part of the windowed list, so it
// may wrap but can never escape the column's physical-row budget.
func TestExpandedMultilineReminderNoteStaysWithinColumn(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	for i := 0; i < 20; i++ {
		backend.reminders = append(backend.reminders, domain.Reminder{
			ID:    int64(i + 1),
			Title: fmt.Sprintf("Reminder %d", i),
			Date:  today.AddDays(i),
			Notes: "line one\nline two\nline three\nline four\nline five\nline six",
		})
	}
	m := New(backend)
	m = send(t, m, tea.WindowSizeMsg{Width: MinWidth, Height: MinHeight})
	m = step(t, m, m.Init())
	m.focus = Focus{Column: ColReminders, Block: RemindersList, Item: 0}
	m = pressKey(t, m, "t")

	out := assertViewFits(t, m, MinWidth, MinHeight, "multiline reminder note")
	details := flattenWhitespace(stripANSI(strings.Join(m.renderReminderDetails(25, m.sortedReminders()[0]), "\n")))
	if !strings.Contains(details, "line one line two") {
		t.Fatalf("expanded multi-line note should be safely normalized and wrapped, got:\n%s", out)
	}
	remindersColumn := flattenWhitespace(stripANSI(m.renderReminders(25, MinHeight-3)))
	if !strings.Contains(remindersColumn, "line five line six") {
		t.Fatalf("expanded reminder details that fit the viewport lost the note tail:\n%s", remindersColumn)
	}
}
