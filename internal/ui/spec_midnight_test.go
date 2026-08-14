package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

// fireTick simulates exactly one firing of Root's once-a-minute midnight poll
// (see commands.go's tickMidnightCmd) without chasing the self-rearming tick
// forever. It deliberately does not use the shared step() helper for the
// outer tickMsg dispatch: step() treats every tickMsg it encounters as a
// terminal "never chase this" leaf (by design, since the real tick blocks on
// a wall-clock timer), but here the *first* tickMsg is the one under test —
// we need to run the fetchSnapshotCmd it produces to completion, while still
// refusing to chase the tick's own rearm cmd. So this helper unpacks the
// batch by hand: any sub-cmd whose message is itself a tickMsg is dropped
// (that's the rearm), everything else is applied via the shared send().
func fireTick(t *testing.T, root *Root) *Root {
	t.Helper()
	next, cmd := root.Update(tickMsg{})
	root = next.(*Root)
	if cmd == nil {
		return root
	}
	msg := cmd()
	cmds := []tea.Cmd{func() tea.Msg { return msg }}
	if batch, ok := msg.(tea.BatchMsg); ok {
		cmds = batch
	}
	for _, c := range cmds {
		if c == nil {
			continue
		}
		m := c()
		if m == nil {
			continue
		}
		if _, ok := m.(tickMsg); ok {
			continue // the rearm; never chase it
		}
		root = send(t, root, m)
	}
	return root
}

// styleRenders reports whether two styles produce identical output for the
// same input, i.e. whether they're the "same style" as far as anything
// visible is concerned. lipgloss.Style holds unexported internal state that
// isn't safe to compare with == (and may not even be comparable), so this is
// the only reliable way to ask "is this the overdue style or the due-soon
// style" from outside the theme package.
func styleRenders(a, b lipgloss.Style) bool {
	const probe = "x"
	return a.Render(probe) == b.Render(probe)
}

// TestSpecMidnightRolloverWhileRunning covers the SPEC requirement that "when
// the date changes, the previous day's state must remain available" together
// with the architecture contract that m.today is derived solely from
// snap.Today.Date (model.go's applySnapshot) and kept fresh by the
// once-a-minute tick (commands.go's tickMidnightCmd) — i.e. a session left
// open across midnight must notice the new day without any key press.
func TestSpecMidnightRolloverWhileRunning(t *testing.T) {
	day1 := domain.MustParseDate("2026-08-10")
	day2 := day1.AddDays(1)

	backend := newFakeBackend(day1)
	backend.withReminders(domain.Reminder{Title: "Turned in", Date: day1})
	root := newReadyRoot(t, backend)

	if root.today != day1 {
		t.Fatalf("today before rollover = %s, want %s", root.today, day1)
	}
	before := root.reminderStatusStyle(backend.reminders[0])
	if !styleRenders(before, root.theme.ReminderDueSoon) {
		t.Fatalf("reminder dated today should render as due-soon before rollover")
	}
	if styleRenders(before, root.theme.ReminderOverdue) {
		t.Fatalf("reminder dated today should not render as overdue before rollover")
	}

	// Midnight passes while the process keeps running: the backend's clock
	// advances, but nothing has told Root yet until the next tick fires.
	backend.today = day2
	if root.today != day1 {
		t.Fatalf("today must not change until the tick notices it: got %s", root.today)
	}

	root = fireTick(t, root)

	if root.today != day2 {
		t.Fatalf("today after midnight tick = %s, want %s", root.today, day2)
	}
	if root.snap.Today.Date != day2 {
		t.Fatalf("snap.Today.Date after rollover = %s, want %s (single source of truth for \"today\")", root.snap.Today.Date, day2)
	}

	after := root.reminderStatusStyle(backend.reminders[0])
	if !styleRenders(after, root.theme.ReminderOverdue) {
		t.Fatalf("reminder dated yesterday should render as overdue after rollover")
	}
	if styleRenders(after, root.theme.ReminderDueSoon) {
		t.Fatalf("reminder dated yesterday should not still render as due-soon after rollover")
	}

	// The "add reminder" form's date default must follow the new today, not
	// the day the process happened to start on.
	root = pressKey(t, root, "left") // Overview -> Reminders
	if root.focus.Column != ColReminders {
		t.Fatalf("focus.Column = %d, want ColReminders", root.focus.Column)
	}
	root = pressKey(t, root, "enter") // Actions block, item 0 = Add reminder
	if root.modal == nil {
		t.Fatal("enter on Reminders actions should open the Add reminder modal")
	}
	view := root.modal.form.View()
	if !strings.Contains(view, day2.String()) {
		t.Fatalf("Add reminder form default date = %q, want it to contain the new today %s", view, day2)
	}
	if strings.Contains(view, day1.String()) {
		t.Fatalf("Add reminder form default date still shows the stale today %s: %q", day1, view)
	}
}
