package ui

import (
	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/app"
)

// handleKey dispatches a key press when no modal is open. Bindings follow
// SPEC.md exactly; h/j/k/l are added as vim-style aliases.
func (m *Root) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.help {
		switch msg.String() {
		case "esc", "?":
			m.help = false
		}
		return m, nil
	}

	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "?":
		m.help = true
		return m, nil
	case "left", "h":
		m.moveColumn(-1)
		return m, nil
	case "right", "l":
		m.moveColumn(1)
		return m, nil
	case "tab":
		m.moveBlock(1)
		return m, nil
	case "shift+tab":
		m.moveBlock(-1)
		return m, nil
	case "up", "k":
		m.moveItem(-1)
		return m, nil
	case "down", "j":
		m.moveItem(1)
		return m, nil
	case "enter":
		if m.pending {
			return m, nil // a mutation is already in flight; drop this activate
		}
		return m.activate()
	case "<", "[":
		return m, m.stepCalendarOrHeatmap(-1)
	case ">", "]":
		return m, m.stepCalendarOrHeatmap(1)
	}
	return m, nil
}

func (m *Root) onHeatmap() bool {
	return m.focus.Column == ColOverview && m.focus.Block == OverviewHeatmap
}

func (m *Root) onCalendar() bool {
	return m.focus.Column == ColOverview && m.focus.Block == OverviewLifetime
}

// stepCalendarOrHeatmap handles </> (aliased to [/]): on the calendar block
// it moves a month, on the heatmap block it cycles the stats range.
// Elsewhere it's a no-op. Up/down are reserved for item navigation only, per
// SPEC.
func (m *Root) stepCalendarOrHeatmap(delta int) tea.Cmd {
	switch {
	case m.onCalendar():
		return m.shiftMonth(delta)
	case m.onHeatmap():
		return m.cycleRange(delta)
	}
	return nil
}

// shiftMonth normalises to the 1st of the current month before adding, so
// e.g. Jan 31 + 1 month lands on Feb (not Mar 3) and Mar 31 - 1 month lands
// on Feb (not stays in Mar).
func (m *Root) shiftMonth(n int) tea.Cmd {
	base := m.calMonth
	if base.IsZero() {
		base = m.today
	}
	m.calMonth = base.StartOfMonth().AddMonths(n)
	return m.fetchSnapshotCmd()
}

func (m *Root) cycleRange(delta int) tea.Cmd {
	idx := 0
	for i, r := range app.StatsRanges {
		if r == m.statsRange {
			idx = i
			break
		}
	}
	m.statsRange = app.StatsRanges[wrap(idx+delta, len(app.StatsRanges))]
	return m.fetchSnapshotCmd()
}

// activate runs the effect of Enter on whatever is currently focused:
// opening a modal, toggling a habit or reminder, or cycling the heatmap
// range.
func (m *Root) activate() (tea.Model, tea.Cmd) {
	switch m.focus.Column {
	case ColOverview:
		return m.activateOverview()
	case ColToday:
		return m.activateToday()
	case ColReminders:
		return m.activateReminders()
	}
	return m, nil
}

func (m *Root) activateOverview() (tea.Model, tea.Cmd) {
	switch m.focus.Block {
	case OverviewActions:
		switch m.focus.Item {
		case ActionAddHabit:
			return m.openAddHabit()
		case ActionAddRoutine:
			return m.openAddRoutine()
		}
	case OverviewHeatmap:
		return m, m.cycleRange(1)
	}
	return m, nil
}

func (m *Root) activateToday() (tea.Model, tea.Cmd) {
	groups := m.snap.Today.Groups
	if m.focus.Block < 0 || m.focus.Block >= len(groups) {
		return m, nil
	}
	items := groups[m.focus.Block].Items
	if m.focus.Item < 0 || m.focus.Item >= len(items) {
		return m, nil
	}
	return m, m.toggleHabitCmd(items[m.focus.Item].Habit.ID)
}

func (m *Root) activateReminders() (tea.Model, tea.Cmd) {
	switch m.focus.Block {
	case RemindersActions:
		return m.openAddReminder()
	case RemindersList:
		reminders := m.sortedReminders()
		if m.focus.Item < 0 || m.focus.Item >= len(reminders) {
			return m, nil
		}
		return m, m.toggleReminderCmd(reminders[m.focus.Item].ID)
	}
	return m, nil
}
