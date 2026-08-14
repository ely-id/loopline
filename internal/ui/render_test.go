package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

// bigSnapshotBackend seeds enough habits, routines and reminders to stress
// the renderers' width/height clamping, including a long title that would
// overflow a column if truncation weren't working.
func bigSnapshotBackend(today domain.Date) *fakeBackend {
	b := newFakeBackend(today)
	b.withRoutines(domain.Routine{Title: "Morning"}, domain.Routine{Title: "Evening"})
	morning := b.routines[0].ID
	evening := b.routines[1].ID

	b.withHabits(
		domain.Habit{Title: "Wake up before 8am, no snoozing, ever, absolutely never again"},
		domain.Habit{Title: "Medicine"},
		domain.Habit{Title: "Drink water", RoutineID: &morning},
		domain.Habit{Title: "Brush teeth", RoutineID: &morning},
		domain.Habit{Title: "Stretch", RoutineID: &morning},
		domain.Habit{Title: "Meditate", RoutineID: &evening},
		domain.Habit{Title: "Journal", RoutineID: &evening},
		domain.Habit{Title: "Read", RoutineID: &evening},
	)
	for i := 0; i < 20; i++ {
		b.withReminders(domain.Reminder{
			Title: "Reminder number this is a fairly long title to test wrapping",
			Date:  today.AddDays(i),
		})
	}
	return b
}

func TestViewNeverExceedsWindowWidth(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	sizes := []struct{ w, h int }{
		{MinWidth, MinHeight},
		{100, 24},
		{140, 45},
		{220, 60},
	}
	for _, sz := range sizes {
		m := newReadyRoot(t, bigSnapshotBackend(today))
		m = send(t, m, tea.WindowSizeMsg{Width: sz.w, Height: sz.h})

		content := m.View().Content
		for i, line := range splitLines(content) {
			if w := lipgloss.Width(line); w > sz.w {
				t.Fatalf("size %dx%d: line %d is %d cells wide (> %d): %q", sz.w, sz.h, i, w, sz.w, line)
			}
		}
	}
}

func TestViewDoesNotPanicAcrossFocusPositions(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := bigSnapshotBackend(today)
	m := newReadyRoot(t, backend)
	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})

	for col := 0; col < numColumns; col++ {
		m.focus.Column = col
		bc := m.blockCount(col)
		for block := 0; block < max(bc, 1); block++ {
			m.focus.Block = block
			ic := m.itemCount(col, block)
			for item := 0; item < max(ic, 1); item++ {
				m.focus.Item = item
				_ = m.View().Content // must not panic
			}
		}
	}
}

func TestEmptyStateRendersWithoutPanic(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	m := newReadyRoot(t, newFakeBackend(today)) // no habits, routines or reminders
	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	content := m.View().Content
	if content == "" {
		t.Fatal("empty-state view should still render something")
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
