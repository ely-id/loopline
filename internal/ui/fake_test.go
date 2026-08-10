package ui

import (
	"context"
	"errors"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
)

// fakeBackend is an in-memory Backend for tests. It's just enough state to
// exercise navigation, rendering and the mutating commands without any real
// storage or Bubble Tea program.
type fakeBackend struct {
	today domain.Date

	habits    []domain.Habit
	routines  []domain.Routine
	reminders []domain.Reminder
	done      map[int64]map[domain.Date]bool // habitID -> date -> completed

	snapshotErr error
	nextID      int64

	calls []string
}

func newFakeBackend(today domain.Date) *fakeBackend {
	return &fakeBackend{today: today, done: map[int64]map[domain.Date]bool{}, nextID: 1}
}

// withHabits seeds habits, auto-assigning IDs and CreatedOn=today if unset.
func (f *fakeBackend) withHabits(habits ...domain.Habit) *fakeBackend {
	for _, h := range habits {
		if h.ID == 0 {
			h.ID = f.nextID
			f.nextID++
		}
		if h.CreatedOn.IsZero() {
			h.CreatedOn = f.today
		}
		if h.Schedule.Kind == "" {
			h.Schedule = domain.DailySchedule()
		}
		f.habits = append(f.habits, h)
	}
	return f
}

func (f *fakeBackend) withRoutines(routines ...domain.Routine) *fakeBackend {
	for _, r := range routines {
		if r.ID == 0 {
			r.ID = f.nextID
			f.nextID++
		}
		if r.CreatedOn.IsZero() {
			r.CreatedOn = f.today
		}
		f.routines = append(f.routines, r)
	}
	return f
}

func (f *fakeBackend) withReminders(reminders ...domain.Reminder) *fakeBackend {
	for _, r := range reminders {
		if r.ID == 0 {
			r.ID = f.nextID
			f.nextID++
		}
		f.reminders = append(f.reminders, r)
	}
	return f
}

func (f *fakeBackend) Today() domain.Date { return f.today }

func (f *fakeBackend) Snapshot(_ context.Context, req app.SnapshotRequest) (app.Snapshot, error) {
	f.calls = append(f.calls, "Snapshot")
	if f.snapshotErr != nil {
		return app.Snapshot{}, f.snapshotErr
	}

	month := req.Month
	if month.IsZero() {
		month = f.today
	}
	statsRange := req.Range
	if statsRange == "" {
		statsRange = app.RangeMonth
	}

	index := map[domain.Date]map[int64]bool{}
	for hid, byDate := range f.done {
		for d, ok := range byDate {
			if !ok {
				continue
			}
			if index[d] == nil {
				index[d] = map[int64]bool{}
			}
			index[d][hid] = true
		}
	}

	reminderDays := map[domain.Date]bool{}
	openReminders := 0
	for _, r := range f.reminders {
		reminderDays[r.Date] = true
		if !r.Completed {
			openReminders++
		}
	}

	today := f.today
	activeRoutines := make([]domain.Routine, 0, len(f.routines))
	for _, r := range f.routines {
		if r.ArchivedOn == nil || today.Before(*r.ArchivedOn) {
			activeRoutines = append(activeRoutines, r)
		}
	}

	groups := make([]app.HabitGroup, 0, len(activeRoutines)+1)
	byRoutine := map[int64]*app.HabitGroup{}
	ungrouped := app.HabitGroup{Title: ""}
	tmp := make([]app.HabitGroup, len(activeRoutines))
	for i, r := range activeRoutines {
		id := r.ID
		tmp[i] = app.HabitGroup{RoutineID: &id, Title: r.Title}
		byRoutine[id] = &tmp[i]
	}
	doneToday := index[today]
	for _, h := range f.habits {
		if !h.ScheduledOn(today) {
			continue
		}
		item := app.HabitItem{Habit: h, Done: doneToday[h.ID], Streak: domain.HabitStreak(h, index, today)}
		target := &ungrouped
		if h.RoutineID != nil {
			if g, ok := byRoutine[*h.RoutineID]; ok {
				target = g
			}
		}
		target.Items = append(target.Items, item)
		target.Scheduled++
		if item.Done {
			target.Completed++
		}
	}
	if len(ungrouped.Items) > 0 {
		groups = append(groups, ungrouped)
	}
	for _, g := range tmp {
		if len(g.Items) > 0 {
			groups = append(groups, g)
		}
	}

	calStart := month.StartOfMonth().StartOfWeek()
	calEnd := month.EndOfMonth().StartOfWeek().AddDays(6)
	var weeks [][]app.CalendarDay
	for ws := calStart; !ws.After(calEnd); ws = ws.AddDays(7) {
		week := make([]app.CalendarDay, 7)
		for i := range week {
			d := ws.AddDays(i)
			week[i] = app.CalendarDay{
				Date:        d,
				InMonth:     d.Month == month.Month && d.Year == month.Year,
				IsToday:     d.Equal(today),
				IsFuture:    d.After(today),
				Summary:     domain.SummarizeDay(f.habits, index[d], d),
				HasReminder: reminderDays[d],
			}
		}
		weeks = append(weeks, week)
	}

	weekStart := today.StartOfWeek()
	isoYear, isoWeek := today.ISOWeek()
	weekDays := make([]app.CalendarDay, 7)
	for i := range weekDays {
		d := weekStart.AddDays(i)
		weekDays[i] = app.CalendarDay{
			Date:        d,
			InMonth:     true,
			IsToday:     d.Equal(today),
			IsFuture:    d.After(today),
			Summary:     domain.SummarizeDay(f.habits, index[d], d),
			HasReminder: reminderDays[d],
		}
	}

	heatStart := today.AddDays(-6)
	var cols [][]app.HeatmapCell
	for ws := heatStart.StartOfWeek(); !ws.After(today); ws = ws.AddDays(7) {
		col := make([]app.HeatmapCell, 7)
		for i := range col {
			d := ws.AddDays(i)
			if d.Before(heatStart) || d.After(today) {
				continue
			}
			sum := domain.SummarizeDay(f.habits, index[d], d)
			col[i] = app.HeatmapCell{Date: d, Summary: sum}
		}
		cols = append(cols, col)
	}

	snap := app.Snapshot{
		Today: app.TodayView{
			Date:    today,
			Groups:  groups,
			Summary: domain.SummarizeDay(f.habits, doneToday, today),
		},
		Overview: app.Overview{
			Today:          today,
			ActiveHabits:   len(f.habits),
			ActiveRoutines: len(activeRoutines),
			OpenReminders:  openReminders,
		},
		Calendar:  app.CalendarMonth{Year: month.Year, Month: month.Month, Weeks: weeks},
		Week:      app.WeekView{ISOYear: isoYear, ISOWeek: isoWeek, Start: weekStart, Days: weekDays},
		Heatmap:   app.Heatmap{Range: statsRange, Start: heatStart, End: today, Columns: cols},
		Reminders: append([]domain.Reminder(nil), f.reminders...),
		Routines:  activeRoutines,
	}
	return snap, nil
}

func (f *fakeBackend) ToggleHabit(_ context.Context, habitID int64) error {
	f.calls = append(f.calls, "ToggleHabit")
	if f.done[habitID] == nil {
		f.done[habitID] = map[domain.Date]bool{}
	}
	f.done[habitID][f.today] = !f.done[habitID][f.today]
	return nil
}

func (f *fakeBackend) AddHabit(_ context.Context, in app.NewHabit) error {
	f.calls = append(f.calls, "AddHabit")
	if in.Title == "" {
		return errors.New("title required")
	}
	f.habits = append(f.habits, domain.Habit{
		ID: f.nextID, Title: in.Title, RoutineID: in.RoutineID,
		Schedule: in.Schedule, CreatedOn: f.today, Notes: in.Notes,
	})
	f.nextID++
	return nil
}

func (f *fakeBackend) AddRoutine(_ context.Context, in app.NewRoutine) error {
	f.calls = append(f.calls, "AddRoutine")
	if in.Title == "" {
		return errors.New("title required")
	}
	f.routines = append(f.routines, domain.Routine{ID: f.nextID, Title: in.Title, CreatedOn: f.today})
	f.nextID++
	return nil
}

func (f *fakeBackend) AddReminder(_ context.Context, in app.NewReminder) error {
	f.calls = append(f.calls, "AddReminder")
	if in.Title == "" {
		return errors.New("title required")
	}
	f.reminders = append(f.reminders, domain.Reminder{ID: f.nextID, Title: in.Title, Date: in.Date, Notes: in.Notes})
	f.nextID++
	return nil
}

func (f *fakeBackend) ToggleReminder(_ context.Context, id int64) error {
	f.calls = append(f.calls, "ToggleReminder")
	for i, r := range f.reminders {
		if r.ID == id {
			f.reminders[i].Completed = !f.reminders[i].Completed
			return nil
		}
	}
	return errors.New("not found")
}

func (f *fakeBackend) DeleteHabit(_ context.Context, id int64) error {
	f.calls = append(f.calls, "DeleteHabit")
	for i, h := range f.habits {
		if h.ID == id {
			f.habits = append(f.habits[:i], f.habits[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func (f *fakeBackend) DeleteRoutine(_ context.Context, id int64) error {
	f.calls = append(f.calls, "DeleteRoutine")
	for i, r := range f.routines {
		if r.ID == id {
			f.routines = append(f.routines[:i], f.routines[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func (f *fakeBackend) DeleteReminder(_ context.Context, id int64) error {
	f.calls = append(f.calls, "DeleteReminder")
	for i, r := range f.reminders {
		if r.ID == id {
			f.reminders = append(f.reminders[:i], f.reminders[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

var _ Backend = (*fakeBackend)(nil)
