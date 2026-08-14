package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"loopline/internal/domain"
)

// ErrNotFound is returned when an operation targets a missing entity.
var ErrNotFound = errors.New("not found")

// Service owns the application's use cases. It is the only component that
// talks to storage, so every state transition and every read model is
// assembled in one place.
type Service struct {
	store Store
	clock domain.Clock
}

// New builds a Service over a Store and a Clock.
func New(store Store, clock domain.Clock) *Service {
	return &Service{store: store, clock: clock}
}

// Today is the application's current local calendar date.
func (s *Service) Today() domain.Date { return s.clock.Today() }

// Snapshot assembles every read model the UI needs in one pass. It performs a
// single completions query covering all requested views so the columns can
// never disagree with each other.
func (s *Service) Snapshot(ctx context.Context, req SnapshotRequest) (Snapshot, error) {
	today := s.clock.Today()

	month := req.Month
	if month.IsZero() {
		month = today
	}

	habits, err := s.store.Habits(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load habits: %w", err)
	}
	routines, err := s.store.Routines(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load routines: %w", err)
	}
	reminders, err := s.store.Reminders(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load reminders: %w", err)
	}

	first, ok, err := s.store.FirstActivityDate(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("first activity: %w", err)
	}
	if !ok {
		first = today
	}
	// Persisted data can legitimately sit in the future relative to the local
	// date after a timezone move or a corrected system clock. Clamping here
	// keeps every range that starts at "first" non-negative.
	if first.After(today) {
		first = today
	}

	calStart := sundayWeekStart(month.StartOfMonth())
	calEnd := sundayWeekStart(month.EndOfMonth()).AddDays(6)
	from := earliest(first, calStart)
	to := latest(today, calEnd)

	comps, err := s.store.CompletionsBetween(ctx, from, to)
	if err != nil {
		return Snapshot{}, fmt.Errorf("load completions: %w", err)
	}
	index := domain.CompletionIndex(comps)

	reminderDays := make(map[domain.Date]bool, len(reminders))
	for _, r := range reminders {
		reminderDays[r.Date] = true
	}

	snap := Snapshot{
		Today:     s.todayView(habits, routines, index, today),
		Weekly:    s.weeklyOverview(habits, index, today),
		Calendar:  s.calendar(habits, index, reminderDays, month, today),
		Reminders: reminders,
		Routines:  activeRoutines(routines, today),
	}
	snap.Overview = s.overview(habits, index, first, today)
	return snap, nil
}

// todayView groups today's scheduled habits: ungrouped first, then one block
// per routine in position order.
func (s *Service) todayView(habits []domain.Habit, routines []domain.Routine, index map[domain.Date]map[int64]bool, today domain.Date) TodayView {
	done := index[today]

	active := activeRoutines(routines, today)
	groups := make([]HabitGroup, len(active))
	ungrouped := HabitGroup{Title: ""}
	byRoutine := make(map[int64]*HabitGroup, len(active))

	for i, r := range active {
		id := r.ID
		groups[i] = HabitGroup{RoutineID: &id, Title: r.Title}
		byRoutine[id] = &groups[i]
	}

	for _, h := range habits {
		if !h.ScheduledOn(today) {
			continue
		}
		item := HabitItem{
			Habit:  h,
			Done:   done[h.ID],
			Streak: domain.HabitStreak(h, index, today),
		}
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

	out := make([]HabitGroup, 0, len(groups)+1)
	if len(ungrouped.Items) > 0 {
		out = append(out, ungrouped)
	}
	out = append(out, groups...)

	return TodayView{
		Date:    today,
		Groups:  out,
		Summary: domain.SummarizeDay(habits, done, today),
	}
}

// weeklyOverview derives a Sunday-first habit matrix from the same
// completion index as Today and Completion. A habit is included when it is
// scheduled at least once during the current week, including a habit that was
// archived partway through the week and therefore still has meaningful
// history in an earlier cell.
func (s *Service) weeklyOverview(habits []domain.Habit, index map[domain.Date]map[int64]bool, today domain.Date) WeeklyOverview {
	start := sundayWeekStart(today)
	weekly := WeeklyOverview{Start: start, End: start.AddDays(6)}
	weekly.Rows = make([]WeeklyHabit, 0, len(habits))

	for _, habit := range habits {
		row := WeeklyHabit{Habit: habit}
		include := false
		for i := range row.Days {
			d := start.AddDays(i)
			scheduled := habit.ScheduledOn(d)
			row.Days[i] = WeeklyDay{
				Date:      d,
				Scheduled: scheduled,
				Completed: scheduled && index[d][habit.ID],
				IsToday:   d.Equal(today),
				IsFuture:  d.After(today),
			}
			include = include || scheduled
		}
		if include {
			weekly.Rows = append(weekly.Rows, row)
		}
	}
	return weekly
}

// calendar builds the Sunday-first month grid used by the Reminder Calendar.
func (s *Service) calendar(habits []domain.Habit, index map[domain.Date]map[int64]bool, reminderDays map[domain.Date]bool, month, today domain.Date) CalendarMonth {
	start := month.StartOfMonth()
	end := month.EndOfMonth()

	cal := CalendarMonth{Year: start.Year, Month: start.Month}
	for weekStart := sundayWeekStart(start); !weekStart.After(end); weekStart = weekStart.AddDays(7) {
		week := make([]CalendarDay, 7)
		for i := range week {
			d := weekStart.AddDays(i)
			week[i] = s.calendarDay(habits, index, reminderDays, d, today)
			week[i].InMonth = d.Year == start.Year && d.Month == start.Month
		}
		cal.Weeks = append(cal.Weeks, week)
	}
	return cal
}

// sundayWeekStart is deliberately presentation-specific. Domain.Date's
// StartOfWeek remains ISO/Monday-first for streak and Year calculations.
func sundayWeekStart(d domain.Date) domain.Date {
	return d.AddDays(-int(d.Weekday()))
}

func (s *Service) calendarDay(habits []domain.Habit, index map[domain.Date]map[int64]bool, reminderDays map[domain.Date]bool, d, today domain.Date) CalendarDay {
	return CalendarDay{
		Date:        d,
		IsToday:     d.Equal(today),
		IsFuture:    d.After(today),
		Summary:     domain.SummarizeDay(habits, index[d], d),
		HasReminder: reminderDays[d],
	}
}

// overview computes the lifetime block.
func (s *Service) overview(habits []domain.Habit, index map[domain.Date]map[int64]bool, first, today domain.Date) Overview {
	summaries := make([]domain.DailySummary, 0, today.DaysSince(first)+1)
	scheduled, completed, tracked := 0, 0, 0
	for d := first; !d.After(today); d = d.AddDays(1) {
		sum := domain.SummarizeDay(habits, index[d], d)
		summaries = append(summaries, sum)
		scheduled += sum.Scheduled
		completed += sum.Completed
		if sum.Scheduled > 0 {
			tracked++
		}
	}

	pct := 0
	if scheduled > 0 {
		pct = completed * 100 / scheduled
	}

	return Overview{
		CurrentStreak: domain.CurrentStreak(summaries, today),
		BestStreak:    domain.BestStreak(summaries),
		// Counted from the same authoritative summaries the calendar uses, rather
		// than from raw completion rows, so a completion that no longer counts
		// as scheduled cannot inflate the lifetime total.
		TotalCompletions: completed,
		DaysTracked:      tracked,
		LifetimePercent:  pct,
	}
}

// ErrNotScheduled is returned when a completion is attempted for a habit that
// is not scheduled on the target date, for instance an archived habit or a
// weekday habit on a day it does not apply to.
var ErrNotScheduled = errors.New("habit is not scheduled on that date")

// ToggleHabit flips today's completion for a habit and refreshes the derived
// daily summary. The flip itself is atomic in the store, so two racing toggles
// cannot collapse into one.
func (s *Service) ToggleHabit(ctx context.Context, habitID int64) error {
	today := s.clock.Today()

	habits, err := s.store.Habits(ctx)
	if err != nil {
		return fmt.Errorf("load habits: %w", err)
	}
	h := findHabit(habits, habitID)
	if h == nil {
		return fmt.Errorf("toggle habit %d: %w", habitID, ErrNotFound)
	}
	// Guards against a stale UI selection, most plausibly one held across
	// midnight, writing a completion that no summary would ever count.
	if !h.ScheduledOn(today) {
		return fmt.Errorf("toggle habit %d on %s: %w", habitID, today, ErrNotScheduled)
	}

	// The toggle and the derived summary land in one transaction, so a failure
	// can never leave a committed completion behind a stale snapshot.
	if _, err := s.store.ToggleCompletion(ctx, habitID, today, s.clock.Now(), scheduledIDs(habits, today)); err != nil {
		return fmt.Errorf("toggle completion: %w", err)
	}
	return nil
}

// scheduledIDs lists the habits the domain considers scheduled on a date. The
// store needs the set, not the scheduling rules, so the decision stays in
// internal/domain.
func scheduledIDs(habits []domain.Habit, on domain.Date) []int64 {
	ids := make([]int64, 0, len(habits))
	for _, h := range habits {
		if h.ScheduledOn(on) {
			ids = append(ids, h.ID)
		}
	}
	return ids
}

// AddHabit creates a habit, effective from today.
func (s *Service) AddHabit(ctx context.Context, in NewHabit) error {
	today := s.clock.Today()
	h := domain.Habit{
		Title:     strings.TrimSpace(in.Title),
		RoutineID: in.RoutineID,
		Schedule:  in.Schedule,
		CreatedOn: today,
		Notes:     strings.TrimSpace(in.Notes),
	}
	if h.Schedule.Kind == "" {
		h.Schedule = domain.DailySchedule()
	}
	if err := h.Validate(); err != nil {
		return err
	}

	habits, err := s.store.Habits(ctx)
	if err != nil {
		return fmt.Errorf("load habits: %w", err)
	}
	if h.RoutineID != nil {
		routines, err := s.store.Routines(ctx)
		if err != nil {
			return fmt.Errorf("load routines: %w", err)
		}
		if findRoutine(routines, *h.RoutineID) == nil {
			return fmt.Errorf("routine %d: %w", *h.RoutineID, ErrNotFound)
		}
	}
	h.Position = nextHabitPosition(habits, h.RoutineID)

	if _, err := s.store.CreateHabit(ctx, h); err != nil {
		return fmt.Errorf("create habit: %w", err)
	}
	return s.refreshSummary(ctx, today)
}

// AddRoutine creates a routine.
func (s *Service) AddRoutine(ctx context.Context, in NewRoutine) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return errors.New("routine title must not be empty")
	}
	routines, err := s.store.Routines(ctx)
	if err != nil {
		return fmt.Errorf("load routines: %w", err)
	}
	pos := 0
	for _, r := range routines {
		if r.Position >= pos {
			pos = r.Position + 1
		}
	}
	r := domain.Routine{Title: title, Position: pos, CreatedOn: s.clock.Today()}
	if _, err := s.store.CreateRoutine(ctx, r); err != nil {
		return fmt.Errorf("create routine: %w", err)
	}
	return nil
}

// AddReminder creates a reminder.
func (s *Service) AddReminder(ctx context.Context, in NewReminder) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return errors.New("reminder title must not be empty")
	}
	if in.Date.IsZero() {
		return errors.New("reminder date must be set")
	}
	r := domain.Reminder{Title: title, Date: in.Date, Notes: strings.TrimSpace(in.Notes)}
	if _, err := s.store.CreateReminder(ctx, r); err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

// EditHabit updates an existing habit in place. Identity, creation/archive
// boundaries and completion rows are preserved; only the fields represented
// by NewHabit are editable.
func (s *Service) EditHabit(ctx context.Context, id int64, in NewHabit) error {
	today := s.clock.Today()
	habits, err := s.store.Habits(ctx)
	if err != nil {
		return fmt.Errorf("load habits: %w", err)
	}
	existing := findHabit(habits, id)
	if existing == nil || existing.ArchivedOn != nil {
		return fmt.Errorf("habit %d: %w", id, ErrNotFound)
	}

	updated := *existing
	updated.Title = strings.TrimSpace(in.Title)
	updated.RoutineID = in.RoutineID
	updated.Schedule = in.Schedule
	updated.Notes = strings.TrimSpace(in.Notes)
	if updated.Schedule.Kind == "" {
		updated.Schedule = domain.DailySchedule()
	}
	if err := updated.Validate(); err != nil {
		return err
	}
	if updated.RoutineID != nil {
		routines, err := s.store.Routines(ctx)
		if err != nil {
			return fmt.Errorf("load routines: %w", err)
		}
		if findRoutine(routines, *updated.RoutineID) == nil {
			return fmt.Errorf("routine %d: %w", *updated.RoutineID, ErrNotFound)
		}
	}
	if !sameRoutine(existing.RoutineID, updated.RoutineID) {
		updated.Position = nextHabitPosition(habits, updated.RoutineID)
	}
	if err := s.store.UpdateHabit(ctx, updated); err != nil {
		return fmt.Errorf("update habit: %w", err)
	}
	return s.refreshSummary(ctx, today)
}

// EditRoutine renames a routine in place. Position, identity and contained
// habit relationships are preserved.
func (s *Service) EditRoutine(ctx context.Context, id int64, in NewRoutine) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return errors.New("routine title must not be empty")
	}
	routines, err := s.store.Routines(ctx)
	if err != nil {
		return fmt.Errorf("load routines: %w", err)
	}
	existing := findRoutine(routines, id)
	if existing == nil || existing.ArchivedOn != nil {
		return fmt.Errorf("routine %d: %w", id, ErrNotFound)
	}
	updated := *existing
	updated.Title = title
	if err := s.store.UpdateRoutine(ctx, updated); err != nil {
		return fmt.Errorf("update routine: %w", err)
	}
	return nil
}

// EditReminder updates editable reminder fields in place while preserving its
// completed status and identity.
func (s *Service) EditReminder(ctx context.Context, id int64, in NewReminder) error {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return errors.New("reminder title must not be empty")
	}
	if in.Date.IsZero() {
		return errors.New("reminder date must be set")
	}
	reminders, err := s.store.Reminders(ctx)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}
	for _, existing := range reminders {
		if existing.ID != id {
			continue
		}
		existing.Title = title
		existing.Date = in.Date
		existing.Notes = strings.TrimSpace(in.Notes)
		if err := s.store.UpdateReminder(ctx, existing); err != nil {
			return fmt.Errorf("update reminder: %w", err)
		}
		return nil
	}
	return fmt.Errorf("reminder %d: %w", id, ErrNotFound)
}

// ToggleReminder flips a reminder's completed state.
func (s *Service) ToggleReminder(ctx context.Context, id int64) error {
	reminders, err := s.store.Reminders(ctx)
	if err != nil {
		return fmt.Errorf("load reminders: %w", err)
	}
	for _, r := range reminders {
		if r.ID != id {
			continue
		}
		r.Completed = !r.Completed
		if err := s.store.UpdateReminder(ctx, r); err != nil {
			return fmt.Errorf("update reminder: %w", err)
		}
		return nil
	}
	return fmt.Errorf("reminder %d: %w", id, ErrNotFound)
}

// DeleteHabit archives a habit from today onward. History before today is
// preserved, which keeps past scheduled counts stable.
func (s *Service) DeleteHabit(ctx context.Context, id int64) error {
	today := s.clock.Today()
	habits, err := s.store.Habits(ctx)
	if err != nil {
		return fmt.Errorf("load habits: %w", err)
	}
	h := findHabit(habits, id)
	if h == nil {
		return fmt.Errorf("habit %d: %w", id, ErrNotFound)
	}
	// Archiving again on a later date would move the boundary forward and make
	// the habit count as scheduled for the dates in between, rewriting history.
	// The first archive date is the one that stands.
	if h.ArchivedOn != nil {
		return nil
	}
	if err := s.store.ArchiveHabit(ctx, id, today); err != nil {
		return fmt.Errorf("archive habit: %w", err)
	}
	return s.refreshSummary(ctx, today)
}

// DeleteRoutine removes a routine and detaches its habits, which survive as
// ungrouped habits.
func (s *Service) DeleteRoutine(ctx context.Context, id int64) error {
	routines, err := s.store.Routines(ctx)
	if err != nil {
		return fmt.Errorf("load routines: %w", err)
	}
	if findRoutine(routines, id) == nil {
		return fmt.Errorf("routine %d: %w", id, ErrNotFound)
	}
	if err := s.store.DeleteRoutine(ctx, id); err != nil {
		return fmt.Errorf("delete routine: %w", err)
	}
	return nil
}

// DeleteReminder removes a reminder permanently.
func (s *Service) DeleteReminder(ctx context.Context, id int64) error {
	if err := s.store.DeleteReminder(ctx, id); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}
	return nil
}

// EnsureToday writes the derived summary for the current date. cmd/loopline
// calls it at startup so a day with no interaction still has its snapshot.
func (s *Service) EnsureToday(ctx context.Context) error {
	return s.refreshSummary(ctx, s.clock.Today())
}

// refreshSummary recomputes the derived daily summary for a date from the
// completions source of truth and writes it through, recounting inside the
// store's transaction. Mutations that change the habit set rather than the
// completions call this; the completion path writes its summary atomically as
// part of the toggle instead.
func (s *Service) refreshSummary(ctx context.Context, d domain.Date) error {
	habits, err := s.store.Habits(ctx)
	if err != nil {
		return fmt.Errorf("load habits: %w", err)
	}
	if err := s.store.RefreshDailySummary(ctx, d, scheduledIDs(habits, d)); err != nil {
		return fmt.Errorf("refresh daily summary: %w", err)
	}
	return nil
}

func activeRoutines(routines []domain.Routine, on domain.Date) []domain.Routine {
	out := make([]domain.Routine, 0, len(routines))
	for _, r := range routines {
		if r.ArchivedOn != nil && !on.Before(*r.ArchivedOn) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out
}

func findHabit(habits []domain.Habit, id int64) *domain.Habit {
	for i := range habits {
		if habits[i].ID == id {
			return &habits[i]
		}
	}
	return nil
}

func findRoutine(routines []domain.Routine, id int64) *domain.Routine {
	for i := range routines {
		if routines[i].ID == id {
			return &routines[i]
		}
	}
	return nil
}

func nextHabitPosition(habits []domain.Habit, routineID *int64) int {
	pos := 0
	for _, h := range habits {
		if !sameRoutine(h.RoutineID, routineID) {
			continue
		}
		if h.Position >= pos {
			pos = h.Position + 1
		}
	}
	return pos
}

func sameRoutine(a, b *int64) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

func earliest(dates ...domain.Date) domain.Date {
	out := dates[0]
	for _, d := range dates[1:] {
		if d.Before(out) {
			out = d
		}
	}
	return out
}

func latest(dates ...domain.Date) domain.Date {
	out := dates[0]
	for _, d := range dates[1:] {
		if d.After(out) {
			out = d
		}
	}
	return out
}
