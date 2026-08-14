package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"loopline/internal/domain"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape codes so a rendered line can be compared
// against plain expected text. Needed because lipgloss reformats (and can
// reposition) escape sequences when it re-wraps already-styled content
// inside an outer Width()/Padding() style, so comparing raw ANSI-embedded
// bytes verbatim is unreliable — only the visible text is a stable target.
func stripANSI(s string) string { return ansiEscape.ReplaceAllString(s, "") }

// manyShortItemsBackend seeds enough habits and reminders to force
// windowAround's vertical scrolling (see today.go/reminders.go) at any
// terminal size this package tests at, while keeping every title short
// enough that it can never itself be word-wrapped by the outer column
// framing — isolating "did the focused item survive scrolling and column
// framing" from "did a too-long title get wrapped", which is a separate,
// already-covered concern (render_test.go's bigSnapshotBackend / TestViewNeverExceedsWindowWidth).
func manyShortItemsBackend(today domain.Date) *fakeBackend {
	b := newFakeBackend(today)
	habits := make([]domain.Habit, 0, 30)
	for i := 1; i <= 30; i++ {
		habits = append(habits, domain.Habit{Title: fmt.Sprintf("Habit%02d", i)})
	}
	b.withHabits(habits...)
	reminders := make([]domain.Reminder, 0, 30)
	for i := 1; i <= 30; i++ {
		reminders = append(reminders, domain.Reminder{Title: fmt.Sprintf("Rem%02d", i), Date: today.AddDays(i)})
	}
	b.withReminders(reminders...)
	return b
}

// TestSpecRenderCalendarsProgressAndFocusedItemArePresent is qa's independent
// check of the render-safety requirement beyond "doesn't panic" (already
// covered by render_test.go's TestViewDoesNotPanicAcrossFocusPositions and
// TestViewNeverExceedsWindowWidth): both calendars, the Today progress
// bar, and the currently focused item must actually appear as visible text
// in View().Content — at the accepted minimum terminal size, and with
// oversized habit/reminder lists that force windowAround's scrolling. This
// is the SPEC "must not corrupt the UI" requirement made concrete: a column
// that silently dropped its bottom half (e.g. a height-clamping regression)
// would pass every existing panic/width test while failing this one.
func TestSpecRenderCalendarsProgressAndFocusedItemArePresent(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")

	sizes := [][2]int{{MinWidth, MinHeight}, {140, 45}}
	for _, wh := range sizes {
		w, h := wh[0], wh[1]
		t.Run(fmt.Sprintf("%dx%d", w, h), func(t *testing.T) {
			root := newReadyRoot(t, manyShortItemsBackend(today))
			root = send(t, root, tea.WindowSizeMsg{Width: w, Height: h})

			// Focus the last (30th) habit, buried at the bottom of a list
			// long enough to require scrolling.
			setPanelFocus(root, Focus{Column: ColToday, Block: 1, Item: 29})
			content := stripANSI(root.View().Content)

			if !strings.Contains(content, "Completion") || !strings.Contains(content, "Calendar") {
				t.Error("completion and reminder calendar headers must both be present")
			}

			sum := root.snap.Today.Summary
			progressLabel := fmt.Sprintf("%d/%d", sum.Completed, sum.Scheduled)
			if !strings.Contains(content, progressLabel) {
				t.Errorf("Today progress bar label %q not present in the rendered view", progressLabel)
			}

			focusedHabit := root.snap.Today.Groups[0].Items[29].Habit.Title
			if !strings.Contains(content, focusedHabit) {
				t.Errorf("focused habit %q (item 30 of 30) not present in the rendered view; windowAround may have lost track of focus", focusedHabit)
			}

			// Now do the same check for the reminders column: focus the
			// last (30th, chronologically furthest) reminder.
			setPanelFocus(root, Focus{Column: ColReminders, Block: RemindersList, Item: 29})
			content = stripANSI(root.View().Content)

			focusedReminder := root.sortedReminders()[29].Title
			if !strings.Contains(content, focusedReminder) {
				t.Errorf("focused reminder %q (item 30 of 30) not present in the rendered view; windowAround may have lost track of focus", focusedReminder)
			}
		})
	}
}
