package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// modalState wraps a huh/v2 form together with the closure that turns its
// completed values into a Backend call. Form field values are bound to local
// variables captured by submit, which is the only piece a builder needs to
// hand back.
type modalState struct {
	form   *huh.Form
	submit func() tea.Cmd
	accept func() bool // nil means every completed form submits
}

// updateModal forwards a message to the open form and reacts to its
// terminal states. The root model intercepts esc itself (see Update), so
// every other message reaches the form.
func (m *Root) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	fm, cmd := m.modal.form.Update(msg)
	m.modal.form = fm.(*huh.Form)

	switch m.modal.form.State {
	case huh.StateCompleted:
		state := m.modal
		m.modal = nil
		if state.accept != nil && !state.accept() {
			return m, cmd
		}
		m.pending = true // guard: the mutation this submit dispatches is now in flight
		return m, tea.Batch(cmd, state.submit())
	case huh.StateAborted:
		m.modal = nil
		return m, nil
	}
	return m, cmd
}

// modalContentSize returns the width/height a huh form should be given so
// that, once framed by theme.ModalBorder (a 1-cell border plus (1,2)
// padding), the rendered modal can never exceed the terminal. huh auto-sizes
// a form/group to whatever tea.WindowSizeMsg it's given as long as
// WithWidth/WithHeight were never called explicitly (see resizeModal and
// openModal below) — including scrolling a group's viewport internally when
// its fields don't fit, which is exactly what an overflowing form needs.
func (m *Root) modalContentSize() (width, height int) {
	const borderAndPadding = 6 // 2 (border) + 4 (padding 1,2 both sides)
	const verticalChrome = 4   // 2 (border) + 2 (padding 1,2 both sides)
	const margin = 4           // breathing room so the box never touches the terminal edge

	width = m.width - borderAndPadding - margin
	height = m.height - verticalChrome - margin
	if width < 20 {
		width = max(m.width-borderAndPadding, 1)
	}
	if height < 5 {
		height = max(m.height-verticalChrome, 1)
	}
	if width > 80 {
		width = 80
	}
	return width, height
}

// resizeModal re-sizes the open form after a terminal resize. huh only
// auto-sizes on tea.WindowSizeMsg, and Root.Update short-circuits real
// WindowSizeMsgs before they'd otherwise reach the modal branch, so this is
// the only path that keeps an open form's size in sync with the terminal.
func (m *Root) resizeModal() (tea.Model, tea.Cmd) {
	w, h := m.modalContentSize()
	return m.updateModal(tea.WindowSizeMsg{Width: w, Height: h})
}

// openModal installs state as the active modal, sizes its form to the
// current terminal immediately (rather than waiting for a resize event that
// may never come), and returns the form's own Init command.
func (m *Root) openModal(state *modalState) (tea.Model, tea.Cmd) {
	m.modal = state
	w, h := m.modalContentSize()
	fm, _ := state.form.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.modal.form = fm.(*huh.Form)
	return m, state.form.Init()
}

func requiredText(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}

func (m *Root) openAddHabit() (tea.Model, tea.Cmd) {
	return m.openModal(newAddHabitForm(m.backend, m.snap.Routines))
}

func (m *Root) openAddRoutine() (tea.Model, tea.Cmd) {
	return m.openModal(newAddRoutineForm(m.backend))
}

func (m *Root) openAddReminder() (tea.Model, tea.Cmd) {
	return m.openModal(newAddReminderForm(m.backend, m.today))
}

func newAddHabitForm(backend Backend, routines []domain.Routine) *modalState {
	initial := app.NewHabit{Schedule: domain.DailySchedule()}
	return newHabitForm("Add habit", routines, initial, func(in app.NewHabit) tea.Cmd {
		return addHabitCmd(backend, in)
	})
}

func newEditHabitForm(backend Backend, routines []domain.Routine, habit domain.Habit) *modalState {
	initial := app.NewHabit{
		Title:     habit.Title,
		RoutineID: habit.RoutineID,
		Schedule:  habit.Schedule,
		Notes:     habit.Notes,
	}
	return newHabitForm("Edit habit", routines, initial, func(in app.NewHabit) tea.Cmd {
		return editHabitCmd(backend, habit.ID, in)
	})
}

func newHabitForm(heading string, routines []domain.Routine, initial app.NewHabit, save func(app.NewHabit) tea.Cmd) *modalState {
	title := initial.Title
	routineID := int64(0)
	if initial.RoutineID != nil {
		routineID = *initial.RoutineID
	}
	kind := string(initial.Schedule.Kind)
	if kind == "" {
		kind = string(domain.ScheduleDaily)
	}
	var weekdays []time.Weekday
	weekdayOrder := []time.Weekday{
		time.Monday, time.Tuesday, time.Wednesday, time.Thursday,
		time.Friday, time.Saturday, time.Sunday,
	}
	if initial.Schedule.Kind == domain.ScheduleWeekdays {
		for _, day := range weekdayOrder {
			if initial.Schedule.Weekdays.Has(day) {
				weekdays = append(weekdays, day)
			}
		}
	}
	notes := initial.Notes

	routineOpts := []huh.Option[int64]{huh.NewOption("(none)", int64(0))}
	for _, r := range routines {
		routineOpts = append(routineOpts, huh.NewOption(r.Title, r.ID))
	}

	weekdayOpts := make([]huh.Option[time.Weekday], 0, len(weekdayOrder))
	for _, day := range weekdayOrder {
		weekdayOpts = append(weekdayOpts, huh.NewOption(day.String()[:3], day))
	}

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Habit title").Value(&title).Validate(requiredText("title")),
		huh.NewSelect[int64]().Title("Routine").Options(routineOpts...).Value(&routineID),
		huh.NewSelect[string]().Title("Schedule").
			Options(
				huh.NewOption("Every day", "daily"),
				huh.NewOption("Specific weekdays", "weekdays"),
			).Value(&kind),
		huh.NewMultiSelect[time.Weekday]().
			Title("Weekdays (only used when schedule = specific weekdays)").
			Options(weekdayOpts...).Value(&weekdays),
		huh.NewText().Title("Notes").Value(&notes),
	).Title(heading)).WithTheme(theme.HuhTheme())

	submit := func() tea.Cmd {
		sched := domain.DailySchedule()
		if kind == "weekdays" {
			sched = domain.Schedule{Kind: domain.ScheduleWeekdays, Weekdays: domain.WeekdaysOf(weekdays...)}
		}
		var routine *int64
		if routineID != 0 {
			id := routineID
			routine = &id
		}
		return save(app.NewHabit{
			Title:     title,
			RoutineID: routine,
			Schedule:  sched,
			Notes:     notes,
		})
	}

	return &modalState{form: form, submit: submit}
}

func newAddRoutineForm(backend Backend) *modalState {
	return newRoutineForm("Add routine", "", func(in app.NewRoutine) tea.Cmd {
		return addRoutineCmd(backend, in)
	})
}

func newEditRoutineForm(backend Backend, routine domain.Routine) *modalState {
	return newRoutineForm("Edit routine", routine.Title, func(in app.NewRoutine) tea.Cmd {
		return editRoutineCmd(backend, routine.ID, in)
	})
}

func newRoutineForm(heading, initialTitle string, save func(app.NewRoutine) tea.Cmd) *modalState {
	title := initialTitle

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Routine title").Value(&title).Validate(requiredText("title")),
	).Title(heading)).WithTheme(theme.HuhTheme())

	submit := func() tea.Cmd {
		return save(app.NewRoutine{Title: title})
	}

	return &modalState{form: form, submit: submit}
}

func newAddReminderForm(backend Backend, today domain.Date) *modalState {
	initial := app.NewReminder{Date: today}
	return newReminderForm("Add reminder", initial, func(in app.NewReminder) tea.Cmd {
		return addReminderCmd(backend, in)
	})
}

func newEditReminderForm(backend Backend, reminder domain.Reminder) *modalState {
	initial := app.NewReminder{Title: reminder.Title, Date: reminder.Date, Notes: reminder.Notes}
	return newReminderForm("Edit reminder", initial, func(in app.NewReminder) tea.Cmd {
		return editReminderCmd(backend, reminder.ID, in)
	})
}

func newReminderForm(heading string, initial app.NewReminder, save func(app.NewReminder) tea.Cmd) *modalState {
	title := initial.Title
	date := initial.Date.String()
	notes := initial.Notes

	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Reminder title").Value(&title).Validate(requiredText("title")),
		huh.NewInput().Title("Date (YYYY-MM-DD)").Value(&date).Validate(func(s string) error {
			if _, err := domain.ParseDate(strings.TrimSpace(s)); err != nil {
				return fmt.Errorf("date must look like 2026-08-10")
			}
			return nil
		}),
		huh.NewText().Title("Notes").Value(&notes),
	).Title(heading)).WithTheme(theme.HuhTheme())

	submit := func() tea.Cmd {
		d, err := domain.ParseDate(strings.TrimSpace(date))
		if err != nil {
			return func() tea.Msg { return actionErrMsg{err: err} }
		}
		return save(app.NewReminder{Title: title, Date: d, Notes: notes})
	}

	return &modalState{form: form, submit: submit}
}

func newDeleteForm(kind, title, description string, submit func() tea.Cmd) *modalState {
	confirmed := false
	confirm := huh.NewConfirm().
		Title(fmt.Sprintf("Delete %s %q?", kind, title)).
		Description(description).
		Affirmative("Delete").
		Negative("Cancel").
		Value(&confirmed)
	form := huh.NewForm(huh.NewGroup(confirm).Title("Confirm deletion")).WithTheme(theme.HuhTheme())
	return &modalState{form: form, submit: submit, accept: func() bool { return confirmed }}
}
