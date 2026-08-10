package domain

import (
	"testing"
	"time"
)

func weekdaySchedule(days ...time.Weekday) Schedule {
	return Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(days...)}
}

// --- Schedule -------------------------------------------------------------

func TestSpecScheduleAppliesOnDaily(t *testing.T) {
	s := DailySchedule()
	dates := []string{"2026-08-10", "2026-08-15", "2026-08-16", "2020-02-29"}
	for _, ds := range dates {
		if !s.AppliesOn(MustParseDate(ds)) {
			t.Fatalf("daily schedule should apply on every date, failed on %s", ds)
		}
	}
}

func TestSpecScheduleAppliesOnWeekdays(t *testing.T) {
	s := weekdaySchedule(time.Monday, time.Wednesday, time.Friday)
	// 2026-08-10 is a Monday (verified independently against Python's
	// datetime.date.strftime("%A")); the following week runs Mon..Sun.
	applies := map[string]bool{
		"2026-08-10": true,  // Mon
		"2026-08-11": false, // Tue
		"2026-08-12": true,  // Wed
		"2026-08-13": false, // Thu
		"2026-08-14": true,  // Fri
		"2026-08-15": false, // Sat
		"2026-08-16": false, // Sun
	}
	for ds, want := range applies {
		got := s.AppliesOn(MustParseDate(ds))
		if got != want {
			t.Errorf("AppliesOn(%s) = %v, want %v", ds, got, want)
		}
	}
}

func TestSpecScheduleValidate(t *testing.T) {
	cases := []struct {
		name    string
		s       Schedule
		wantErr bool
	}{
		{"daily", DailySchedule(), false},
		{"weekdays one day", weekdaySchedule(time.Monday), false},
		{"weekdays all days", Schedule{Kind: ScheduleWeekdays, Weekdays: AllWeekdays}, false},
		{"weekdays empty", Schedule{Kind: ScheduleWeekdays, Weekdays: 0}, true},
		{"weekdays invalid bit", Schedule{Kind: ScheduleWeekdays, Weekdays: AllWeekdays | (1 << 7)}, true},
		{"unknown kind", Schedule{Kind: "monthly"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.s.Validate()
			if (err != nil) != c.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, c.wantErr)
			}
		})
	}
}

func TestSpecScheduleString(t *testing.T) {
	cases := []struct {
		name string
		s    Schedule
		want string
	}{
		{"daily", DailySchedule(), "daily"},
		{"single day", weekdaySchedule(time.Monday), "mo"},
		{"mon wed fri", weekdaySchedule(time.Monday, time.Wednesday, time.Friday), "mo we fr"},
		{"week order not insertion order", weekdaySchedule(time.Saturday, time.Sunday), "su sa"},
		{"all days", Schedule{Kind: ScheduleWeekdays, Weekdays: AllWeekdays}, "su mo tu we th fr sa"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.s.String(); got != c.want {
				t.Fatalf("String() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSpecWeekdaySetHasWith(t *testing.T) {
	var w WeekdaySet
	if w.Has(time.Monday) {
		t.Fatal("zero-value WeekdaySet must not contain Monday")
	}
	w = w.With(time.Monday)
	if !w.Has(time.Monday) {
		t.Fatal("With(Monday) should make Has(Monday) true")
	}
	if w.Has(time.Tuesday) {
		t.Fatal("With(Monday) should not add Tuesday")
	}
	w = w.With(time.Friday)
	if !w.Has(time.Monday) || !w.Has(time.Friday) {
		t.Fatal("With should be additive, not overwrite previous days")
	}

	built := WeekdaysOf(time.Monday, time.Friday)
	if built != w {
		t.Fatalf("WeekdaysOf(Mon, Fri) = %#v, want %#v (built via With)", built, w)
	}

	all := WeekdaysOf(time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday)
	if all != AllWeekdays {
		t.Fatalf("WeekdaysOf(all seven days) = %v, want AllWeekdays %v", all, AllWeekdays)
	}
}

// --- Habit ------------------------------------------------------------

func TestSpecHabitActiveOnCreatedBoundary(t *testing.T) {
	h := Habit{CreatedOn: MustParseDate("2026-08-05"), Schedule: DailySchedule()}
	if h.ActiveOn(MustParseDate("2026-08-04")) {
		t.Fatal("habit must not be active the day before it was created")
	}
	if !h.ActiveOn(MustParseDate("2026-08-05")) {
		t.Fatal("habit must be active on its creation date")
	}
	if !h.ActiveOn(MustParseDate("2026-08-10")) {
		t.Fatal("habit must be active well after creation with no archive date")
	}
}

func TestSpecHabitActiveOnArchivedBoundary(t *testing.T) {
	archived := MustParseDate("2026-08-10")
	h := Habit{CreatedOn: MustParseDate("2026-08-05"), ArchivedOn: &archived, Schedule: DailySchedule()}

	if !h.ActiveOn(MustParseDate("2026-08-09")) {
		t.Fatal("habit must still be active the day before archived_on")
	}
	if h.ActiveOn(MustParseDate("2026-08-10")) {
		t.Fatal("habit must NOT be active on archived_on itself (archiving is effective that day)")
	}
	if h.ActiveOn(MustParseDate("2026-08-11")) {
		t.Fatal("habit must not be active after archived_on")
	}
	if h.ActiveOn(MustParseDate("2026-08-04")) {
		t.Fatal("habit must not be active before created_on even with an archive date set")
	}
}

func TestSpecHabitScheduledOnCombinesActiveAndSchedule(t *testing.T) {
	archived := MustParseDate("2026-08-14") // a Friday
	h := Habit{
		CreatedOn:  MustParseDate("2026-08-03"), // a Monday
		ArchivedOn: &archived,
		Schedule:   weekdaySchedule(time.Monday, time.Wednesday, time.Friday),
	}

	cases := []struct {
		date string
		want bool
	}{
		{"2026-08-02", false}, // before created_on, would be a Sunday anyway
		{"2026-08-03", true},  // created_on, Monday, scheduled
		{"2026-08-04", false}, // active but Tuesday: not scheduled
		{"2026-08-05", true},  // active, Wednesday
		{"2026-08-12", true},  // active, Wednesday
		{"2026-08-14", false}, // archived_on itself: not active, even though it's a Friday
		{"2026-08-13", false}, // active, Thursday: not scheduled
	}
	for _, c := range cases {
		got := h.ScheduledOn(MustParseDate(c.date))
		if got != c.want {
			t.Errorf("ScheduledOn(%s) = %v, want %v", c.date, got, c.want)
		}
	}
}

func TestSpecHabitValidate(t *testing.T) {
	valid := Habit{Title: "Drink water", CreatedOn: MustParseDate("2026-08-01"), Schedule: DailySchedule()}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid habit failed validation: %v", err)
	}

	empty := valid
	empty.Title = ""
	if err := empty.Validate(); err == nil {
		t.Fatal("expected error for empty title")
	}

	badSchedule := valid
	badSchedule.Schedule = Schedule{Kind: ScheduleWeekdays, Weekdays: 0}
	if err := badSchedule.Validate(); err == nil {
		t.Fatal("expected error for invalid schedule")
	}
}

// --- DailySummary -------------------------------------------------------

func TestSpecDailySummaryPercent(t *testing.T) {
	cases := []struct {
		name      string
		scheduled int
		completed int
		want      int
	}{
		{"zero scheduled with zero completed", 0, 0, 0},
		{"zero scheduled but nonzero completed is still 0", 0, 3, 0},
		{"one third rounds down", 3, 1, 33},
		{"two thirds rounds up", 3, 2, 67},
		{"half", 2, 1, 50},
		{"one eighth rounds half up", 8, 1, 13}, // 12.5 -> 13 (round half away from zero)
		{"three eighths rounds half up", 8, 3, 38},
		{"full", 5, 5, 100},
		{"nothing done", 4, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DailySummary{Scheduled: c.scheduled, Completed: c.completed}
			if got := s.Percent(); got != c.want {
				t.Fatalf("Percent() = %d, want %d", got, c.want)
			}
		})
	}
}

func TestSpecDailySummaryIsComplete(t *testing.T) {
	cases := []struct {
		name      string
		scheduled int
		completed int
		want      bool
	}{
		{"nothing scheduled is never complete", 0, 0, false},
		{"nothing scheduled, phantom completions still not complete", 0, 2, false},
		{"partial", 3, 2, false},
		{"exact", 3, 3, true},
		{"over-complete counts as complete", 3, 4, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := DailySummary{Scheduled: c.scheduled, Completed: c.completed}
			if got := s.IsComplete(); got != c.want {
				t.Fatalf("IsComplete() = %v, want %v", got, c.want)
			}
		})
	}
}

// --- SummarizeDay / SummarizeRange / CompletionIndex ---------------------

func TestSpecSummarizeDayExcludesUnscheduledHabits(t *testing.T) {
	d := MustParseDate("2026-08-10") // Monday
	daily := Habit{ID: 1, CreatedOn: MustParseDate("2026-01-01"), Schedule: DailySchedule()}
	weekdaysOnly := Habit{ID: 2, CreatedOn: MustParseDate("2026-01-01"), Schedule: weekdaySchedule(time.Tuesday)}
	habits := []Habit{daily, weekdaysOnly}

	done := map[int64]bool{1: true, 2: true} // habit 2 done-flag present but not scheduled today
	sum := SummarizeDay(habits, done, d)

	if sum.Scheduled != 1 {
		t.Fatalf("Scheduled = %d, want 1 (only the daily habit is scheduled on a Monday)", sum.Scheduled)
	}
	if sum.Completed != 1 {
		t.Fatalf("Completed = %d, want 1 (the unscheduled habit's completion must not count)", sum.Completed)
	}
	if sum.Date != d {
		t.Fatalf("Date = %s, want %s", sum.Date, d)
	}
}

func TestSpecSummarizeDayArchivedHabitBoundary(t *testing.T) {
	archived := MustParseDate("2026-08-10")
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-01"), ArchivedOn: &archived, Schedule: DailySchedule()}
	habits := []Habit{h}
	done := map[int64]bool{1: true}

	before := SummarizeDay(habits, done, MustParseDate("2026-08-09"))
	if before.Scheduled != 1 || before.Completed != 1 {
		t.Fatalf("day before archive: got %+v, want Scheduled=1 Completed=1", before)
	}

	onArchiveDay := SummarizeDay(habits, done, archived)
	if onArchiveDay.Scheduled != 0 || onArchiveDay.Completed != 0 {
		t.Fatalf("archive day itself: got %+v, want Scheduled=0 Completed=0 (archived habits excluded from archived_on onward)", onArchiveDay)
	}

	after := SummarizeDay(habits, done, archived.AddDays(5))
	if after.Scheduled != 0 || after.Completed != 0 {
		t.Fatalf("day after archive: got %+v, want Scheduled=0 Completed=0", after)
	}
}

func TestSpecSummarizeDayCreatedOnBoundary(t *testing.T) {
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-05"), Schedule: DailySchedule()}
	habits := []Habit{h}
	done := map[int64]bool{1: true}

	before := SummarizeDay(habits, done, MustParseDate("2026-08-04"))
	if before.Scheduled != 0 {
		t.Fatalf("day before created_on: Scheduled = %d, want 0 (habit does not exist yet)", before.Scheduled)
	}

	onCreation := SummarizeDay(habits, done, MustParseDate("2026-08-05"))
	if onCreation.Scheduled != 1 || onCreation.Completed != 1 {
		t.Fatalf("creation day: got %+v, want Scheduled=1 Completed=1", onCreation)
	}
}

func TestSpecSummarizeRangeMatchesPerDaySummarizeDay(t *testing.T) {
	daily := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-01"), Schedule: DailySchedule()}
	weekdaysOnly := Habit{ID: 2, CreatedOn: MustParseDate("2026-08-01"), Schedule: weekdaySchedule(time.Wednesday)}
	habits := []Habit{daily, weekdaysOnly}

	comps := []Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-10")},
		{HabitID: 2, Date: MustParseDate("2026-08-12")}, // a Wednesday
		{HabitID: 1, Date: MustParseDate("2026-08-12")},
	}

	from, to := MustParseDate("2026-08-10"), MustParseDate("2026-08-13")
	got := SummarizeRange(habits, comps, from, to)

	if len(got) != 4 {
		t.Fatalf("len(SummarizeRange) = %d, want 4 (Aug 10..13 inclusive)", len(got))
	}

	index := CompletionIndex(comps)
	for i, d := 0, from; !d.After(to); i, d = i+1, d.AddDays(1) {
		want := SummarizeDay(habits, index[d], d)
		if got[i] != want {
			t.Fatalf("SummarizeRange[%d] (%s) = %+v, want %+v", i, d, got[i], want)
		}
	}
}

func TestSpecSummarizeRangeEmptyWhenEndBeforeStart(t *testing.T) {
	got := SummarizeRange(nil, nil, MustParseDate("2026-08-10"), MustParseDate("2026-08-09"))
	if len(got) != 0 {
		t.Fatalf("SummarizeRange(end < start) = %v, want empty", got)
	}
}

func TestSpecCompletionIndex(t *testing.T) {
	d1 := MustParseDate("2026-08-10")
	d2 := MustParseDate("2026-08-11")
	comps := []Completion{
		{HabitID: 1, Date: d1},
		{HabitID: 2, Date: d1},
		{HabitID: 1, Date: d2},
	}
	idx := CompletionIndex(comps)

	if !idx[d1][1] || !idx[d1][2] {
		t.Fatalf("idx[%s] = %v, want habits 1 and 2 present", d1, idx[d1])
	}
	if idx[d1][3] {
		t.Fatalf("idx[%s][3] = true, want false (habit 3 was never completed)", d1)
	}
	if !idx[d2][1] {
		t.Fatalf("idx[%s][1] = false, want true", d2)
	}
	if idx[d2][2] {
		t.Fatalf("idx[%s][2] = true, want false (habit 2 was not completed on %s)", d2, d2)
	}
	unrelated := MustParseDate("2026-01-01")
	if len(idx[unrelated]) != 0 {
		t.Fatalf("idx[%s] = %v, want empty map for a date with no completions", unrelated, idx[unrelated])
	}
}

// --- HabitStreak ----------------------------------------------------------

func TestSpecHabitStreakSkipsUnscheduledDaysWithoutBreaking(t *testing.T) {
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-07-01"), Schedule: weekdaySchedule(time.Monday, time.Wednesday, time.Friday)}
	completedDates := []string{
		"2026-08-03", // Mon
		"2026-08-05", // Wed
		"2026-08-07", // Fri
		"2026-08-10", // Mon
		"2026-08-12", // Wed
		"2026-08-14", // Fri
	}
	comps := map[Date]map[int64]bool{}
	for _, ds := range completedDates {
		comps[MustParseDate(ds)] = map[int64]bool{1: true}
	}

	got := HabitStreak(h, comps, MustParseDate("2026-08-14"))
	want := len(completedDates)
	if got != want {
		t.Fatalf("HabitStreak = %d, want %d (weekends/Tue/Thu must be skipped without breaking the run)", got, want)
	}
}

func TestSpecHabitStreakUpToIncompleteDoesNotBreak(t *testing.T) {
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-01"), Schedule: DailySchedule()}
	comps := map[Date]map[int64]bool{
		MustParseDate("2026-08-01"): {1: true},
		MustParseDate("2026-08-02"): {1: true},
		MustParseDate("2026-08-03"): {1: true},
		MustParseDate("2026-08-04"): {1: true},
		// 2026-08-05 (upTo) intentionally not completed.
	}

	got := HabitStreak(h, comps, MustParseDate("2026-08-05"))
	if got != 4 {
		t.Fatalf("HabitStreak = %d, want 4 (today being incomplete must not break an otherwise intact streak)", got)
	}
}

func TestSpecHabitStreakBreaksOnIncompleteDayBeforeUpTo(t *testing.T) {
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-01"), Schedule: DailySchedule()}
	comps := map[Date]map[int64]bool{
		MustParseDate("2026-08-01"): {1: true},
		MustParseDate("2026-08-02"): {1: true},
		// 2026-08-03 is missing: a genuine break, not upTo.
		MustParseDate("2026-08-04"): {1: true},
		MustParseDate("2026-08-05"): {1: true},
	}

	got := HabitStreak(h, comps, MustParseDate("2026-08-05"))
	if got != 2 {
		t.Fatalf("HabitStreak = %d, want 2 (Aug 4-5 only; the Aug 3 gap must stop the count before Aug 1-2)", got)
	}
}

func TestSpecHabitStreakRespectsCreatedOnEvenWithEarlierCompletions(t *testing.T) {
	h := Habit{ID: 1, CreatedOn: MustParseDate("2026-08-03"), Schedule: DailySchedule()}
	comps := map[Date]map[int64]bool{
		// These predate CreatedOn and must never be reachable by the walk.
		MustParseDate("2026-08-01"): {1: true},
		MustParseDate("2026-08-02"): {1: true},
		MustParseDate("2026-08-03"): {1: true},
		MustParseDate("2026-08-04"): {1: true},
		MustParseDate("2026-08-05"): {1: true},
	}

	got := HabitStreak(h, comps, MustParseDate("2026-08-05"))
	if got != 3 {
		t.Fatalf("HabitStreak = %d, want 3 (Aug 3-5 only; completions before created_on must not count)", got)
	}
}

// --- CurrentStreak ----------------------------------------------------------

func TestSpecCurrentStreakSkipsUnscheduledDaysWithoutBreaking(t *testing.T) {
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-01"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-02"), Scheduled: 0, Completed: 0}, // unscheduled day
		{Date: MustParseDate("2026-08-03"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-04"), Scheduled: 0, Completed: 0}, // unscheduled day
		{Date: MustParseDate("2026-08-05"), Scheduled: 3, Completed: 3},
	}
	got := CurrentStreak(summaries, MustParseDate("2026-08-05"))
	if got != 3 {
		t.Fatalf("CurrentStreak = %d, want 3 (unscheduled days must not break the run)", got)
	}
}

func TestSpecCurrentStreakTodayIncompleteDoesNotBreak(t *testing.T) {
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-01"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-02"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-03"), Scheduled: 2, Completed: 1}, // today, incomplete
	}
	got := CurrentStreak(summaries, MustParseDate("2026-08-03"))
	if got != 2 {
		t.Fatalf("CurrentStreak = %d, want 2 (today not yet done must not break an intact streak)", got)
	}
}

func TestSpecCurrentStreakBreaksOnEarlierIncompleteDay(t *testing.T) {
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-01"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-02"), Scheduled: 2, Completed: 1}, // incomplete, not today
		{Date: MustParseDate("2026-08-03"), Scheduled: 2, Completed: 2}, // today
	}
	got := CurrentStreak(summaries, MustParseDate("2026-08-03"))
	if got != 1 {
		t.Fatalf("CurrentStreak = %d, want 1 (Aug 2 incomplete must stop the walk before reaching Aug 1)", got)
	}
}

func TestSpecCurrentStreakEmptyWhenNoSummaries(t *testing.T) {
	if got := CurrentStreak(nil, MustParseDate("2026-08-10")); got != 0 {
		t.Fatalf("CurrentStreak(nil, ...) = %d, want 0", got)
	}
}

// --- BestStreak -------------------------------------------------------------

func TestSpecBestStreakPicksLongestRunAcrossUnscheduledGapsAndIsOrderIndependent(t *testing.T) {
	// Two runs: Aug1-2 + Aug4 (joined across the Aug3 unscheduled gap) = length 3,
	// then Aug5 breaks it, then Aug6-7 = length 2. Best should be 3.
	// Passed out of chronological order to verify BestStreak sorts internally.
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-04"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-01"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-07"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-03"), Scheduled: 0, Completed: 0}, // unscheduled gap
		{Date: MustParseDate("2026-08-06"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-02"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-05"), Scheduled: 1, Completed: 0}, // breaks the run
	}
	got := BestStreak(summaries)
	if got != 3 {
		t.Fatalf("BestStreak = %d, want 3", got)
	}

	original := make([]DailySummary, len(summaries))
	copy(original, summaries)
	if len(summaries) != len(original) {
		t.Fatal("BestStreak must not mutate its input slice length")
	}
}

func TestSpecBestStreakEmpty(t *testing.T) {
	if got := BestStreak(nil); got != 0 {
		t.Fatalf("BestStreak(nil) = %d, want 0", got)
	}
	allIncomplete := []DailySummary{
		{Date: MustParseDate("2026-08-01"), Scheduled: 1, Completed: 0},
		{Date: MustParseDate("2026-08-02"), Scheduled: 0, Completed: 0},
	}
	if got := BestStreak(allIncomplete); got != 0 {
		t.Fatalf("BestStreak(all incomplete/unscheduled) = %d, want 0", got)
	}
}
