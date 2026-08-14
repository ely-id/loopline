package ui

import (
	tea "charm.land/bubbletea/v2"

	"loopline/internal/domain"
)

type selectedEntity struct {
	habit    *domain.Habit
	routine  *domain.Routine
	reminder *domain.Reminder
}

// editableSelection resolves the entity implied by the current local focus.
// Routine headers are always first-class selectable entities; their children
// follow them when expanded.
func (m *Root) editableSelection() selectedEntity {
	if m.navigation != NavigationPanel {
		return selectedEntity{}
	}
	switch m.focus.Column {
	case ColToday:
		groupIndex := m.focus.Block - 1
		if groupIndex < 0 || groupIndex >= len(m.snap.Today.Groups) {
			return selectedEntity{}
		}
		group := m.snap.Today.Groups[groupIndex]
		if group.RoutineID != nil && m.focus.Item == 0 {
			for _, routine := range m.snap.Routines {
				if routine.ID == *group.RoutineID {
					copy := routine
					return selectedEntity{routine: &copy}
				}
			}
			return selectedEntity{}
		}
		itemIndex := m.focus.Item
		if group.RoutineID != nil {
			itemIndex--
		}
		if itemIndex < 0 || itemIndex >= len(group.Items) {
			return selectedEntity{}
		}
		habit := group.Items[itemIndex].Habit
		return selectedEntity{habit: &habit}
	case ColReminders:
		if m.focus.Block != RemindersList {
			return selectedEntity{}
		}
		reminders := m.sortedReminders()
		if m.focus.Item < 0 || m.focus.Item >= len(reminders) {
			return selectedEntity{}
		}
		reminder := reminders[m.focus.Item]
		return selectedEntity{reminder: &reminder}
	default:
		return selectedEntity{}
	}
}

func (s selectedEntity) valid() bool {
	return s.habit != nil || s.routine != nil || s.reminder != nil
}

func (m *Root) editSelected() (tea.Model, tea.Cmd) {
	selection := m.editableSelection()
	switch {
	case selection.habit != nil:
		return m.openModal(newEditHabitForm(m.backend, m.snap.Routines, *selection.habit))
	case selection.routine != nil:
		return m.openModal(newEditRoutineForm(m.backend, *selection.routine))
	case selection.reminder != nil:
		return m.openModal(newEditReminderForm(m.backend, *selection.reminder))
	default:
		return m, nil
	}
}

func (m *Root) deleteSelected() (tea.Model, tea.Cmd) {
	selection := m.editableSelection()
	switch {
	case selection.habit != nil:
		habit := *selection.habit
		return m.openModal(newDeleteForm(
			"habit", habit.Title,
			"Completion history is preserved; the habit stops being scheduled today.",
			func() tea.Cmd { return deleteHabitCmd(m.backend, habit.ID) },
		))
	case selection.routine != nil:
		routine := *selection.routine
		return m.openModal(newDeleteForm(
			"routine", routine.Title,
			"Its habits will remain and move to the ungrouped list.",
			func() tea.Cmd { return deleteRoutineCmd(m.backend, routine.ID) },
		))
	case selection.reminder != nil:
		reminder := *selection.reminder
		return m.openModal(newDeleteForm(
			"reminder", reminder.Title,
			"This permanently removes the reminder.",
			func() tea.Cmd { return deleteReminderCmd(m.backend, reminder.ID) },
		))
	default:
		return m, nil
	}
}
