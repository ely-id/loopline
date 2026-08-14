package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

var appKeys = struct {
	NextPanel, PreviousPanel key.Binding
	Up, Down                 key.Binding
	Left, Right              key.Binding
	Enter, Back              key.Binding
	Edit, Toggle, Delete     key.Binding
	PreviousMonth, NextMonth key.Binding
	Help, Quit               key.Binding
}{
	NextPanel:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "panel")),
	PreviousPanel: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous panel")),
	Up:            key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "previous item")),
	Down:          key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next item")),
	Left:          key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "older history")),
	Right:         key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "newer history")),
	Enter:         key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Back:          key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "menu")),
	Edit:          key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Toggle:        key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle")),
	Delete:        key.NewBinding(key.WithKeys("delete"), key.WithHelp("del", "delete")),
	PreviousMonth: key.NewBinding(key.WithKeys("<", "["), key.WithHelp("</[", "previous month")),
	NextMonth:     key.NewBinding(key.WithKeys(">", "]"), key.WithHelp(">/]", "next month")),
	Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:          key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
}

// handleKey dispatches a key press when no modal is open. Header mode owns
// global panel selection; panel mode owns vertical local navigation.
func (m *Root) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.help {
		switch {
		case key.Matches(msg, appKeys.Back), key.Matches(msg, appKeys.Help):
			m.help = false
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, appKeys.Quit):
		return m, tea.Quit
	case key.Matches(msg, appKeys.Help):
		m.help = true
		return m, nil
	case m.navigation == NavigationHeader && key.Matches(msg, appKeys.NextPanel):
		m.moveGlobal(1)
		return m, nil
	case m.navigation == NavigationHeader && key.Matches(msg, appKeys.PreviousPanel):
		m.moveGlobal(-1)
		return m, nil
	case m.navigation == NavigationHeader && key.Matches(msg, appKeys.Enter):
		m.navigation = NavigationPanel
		m.clampFocus()
		return m, nil
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Back):
		m.navigation = NavigationHeader
		return m, nil
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Up):
		m.moveLocal(-1)
		return m, nil
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Down):
		m.moveLocal(1)
		return m, nil
	case m.navigation == NavigationPanel && m.onCompletionTimeline() && key.Matches(msg, appKeys.Left):
		m.scrollCompletion(-1)
		return m, nil
	case m.navigation == NavigationPanel && m.onCompletionTimeline() && key.Matches(msg, appKeys.Right):
		m.scrollCompletion(1)
		return m, nil
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Enter):
		if m.pending {
			return m, nil // a mutation is already in flight; drop this activate
		}
		return m.activate()
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Edit):
		if m.pending {
			return m, nil
		}
		return m.editSelected()
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Toggle):
		m.toggleExpansion()
		return m, nil
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.Delete):
		if m.pending {
			return m, nil
		}
		return m.deleteSelected()
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.PreviousMonth):
		return m, m.stepCalendar(-1)
	case m.navigation == NavigationPanel && key.Matches(msg, appKeys.NextMonth):
		return m, m.stepCalendar(1)
	}
	return m, nil
}

func (m *Root) onCalendar() bool {
	return m.navigation == NavigationPanel && m.focus.Column == ColReminders && m.focus.Block == RemindersCalendar
}

func (m *Root) onCompletionTimeline() bool {
	return m.navigation == NavigationPanel && m.focus.Column == ColOverview && m.focus.Block == OverviewCompletion
}

// stepCalendar handles </> (aliased to [/]) on the conventional Calendar.
// Elsewhere it is a no-op. Up/down remain reserved for item navigation.
func (m *Root) stepCalendar(delta int) tea.Cmd {
	if m.onCalendar() {
		return m.shiftMonth(delta)
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

// activate runs the effect of Enter on whatever is currently focused:
// opening a modal or toggling a habit or reminder.
func (m *Root) activate() (tea.Model, tea.Cmd) {
	switch m.focus.Column {
	case ColOverview:
		return m, nil
	case ColToday:
		return m.activateToday()
	case ColReminders:
		return m.activateReminders()
	}
	return m, nil
}

func (m *Root) activateToday() (tea.Model, tea.Cmd) {
	if m.focus.Block == TodayActions {
		switch m.focus.Item {
		case ActionAddHabit:
			return m.openAddHabit()
		case ActionAddRoutine:
			return m.openAddRoutine()
		}
		return m, nil
	}

	groups := m.snap.Today.Groups
	group := m.focus.Block - 1
	if group < 0 || group >= len(groups) {
		return m, nil
	}
	if groups[group].RoutineID != nil && m.focus.Item == 0 {
		m.toggleExpansion()
		return m, nil
	}
	if m.routineCollapsed(groups[group]) {
		return m, nil
	}
	items := groups[group].Items
	itemIndex := m.focus.Item
	if groups[group].RoutineID != nil {
		itemIndex--
	}
	if itemIndex < 0 || itemIndex >= len(items) {
		return m, nil
	}
	return m, m.toggleHabitCmd(items[itemIndex].Habit.ID)
}

// toggleExpansion changes presentation state only. Routine expansion applies
// to the routine containing the selected habit; reminder expansion applies
// to the selected row. Enter keeps its existing primary-action meaning.
func (m *Root) toggleExpansion() {
	switch m.focus.Column {
	case ColToday:
		groupIndex := m.focus.Block - 1
		if groupIndex < 0 || groupIndex >= len(m.snap.Today.Groups) {
			return
		}
		group := m.snap.Today.Groups[groupIndex]
		if group.RoutineID == nil {
			return // ungrouped habits are not a collapsible routine
		}
		if m.collapsedRoutines == nil {
			m.collapsedRoutines = make(map[int64]bool)
		}
		id := *group.RoutineID
		m.collapsedRoutines[id] = !m.collapsedRoutines[id]
		if !m.collapsedRoutines[id] {
			delete(m.collapsedRoutines, id)
		}
		m.focus.Item = 0
	case ColReminders:
		if m.focus.Block != RemindersList {
			return
		}
		reminders := m.sortedReminders()
		if m.focus.Item < 0 || m.focus.Item >= len(reminders) {
			return
		}
		id := reminders[m.focus.Item].ID
		if m.expandedReminder == id {
			m.expandedReminder = 0
		} else {
			m.expandedReminder = id
		}
	}
}

func (m *Root) expansionHint() string {
	switch m.focus.Column {
	case ColToday:
		groupIndex := m.focus.Block - 1
		if groupIndex < 0 || groupIndex >= len(m.snap.Today.Groups) {
			return ""
		}
		group := m.snap.Today.Groups[groupIndex]
		if group.RoutineID == nil {
			return ""
		}
		if m.routineCollapsed(group) {
			return "expand"
		}
		return "collapse"
	case ColReminders:
		if m.focus.Block != RemindersList {
			return ""
		}
		reminders := m.sortedReminders()
		if m.focus.Item < 0 || m.focus.Item >= len(reminders) {
			return ""
		}
		if m.expandedReminder == reminders[m.focus.Item].ID {
			return "collapse"
		}
		return "expand"
	default:
		return ""
	}
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
