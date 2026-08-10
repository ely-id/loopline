package domain

import (
	"testing"
	"time"
)

func TestDailySummaryPercent(t *testing.T) {
	cases := []struct {
		s    DailySummary
		want int
	}{
		{DailySummary{Scheduled: 0, Completed: 0}, 0},
		{DailySummary{Scheduled: 4, Completed: 0}, 0},
		{DailySummary{Scheduled: 4, Completed: 4}, 100},
		{DailySummary{Scheduled: 4, Completed: 1}, 25},
		{DailySummary{Scheduled: 3, Completed: 1}, 33}, // rounds down from 33.33
		{DailySummary{Scheduled: 3, Completed: 2}, 67}, // rounds up from 66.67
	}
	for _, c := range cases {
		if got := c.s.Percent(); got != c.want {
			t.Errorf("Percent(%+v) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestDailySummaryIsComplete(t *testing.T) {
	if (DailySummary{Scheduled: 0, Completed: 0}).IsComplete() {
		t.Error("a day with nothing scheduled should not be complete")
	}
	if (DailySummary{Scheduled: 2, Completed: 1}).IsComplete() {
		t.Error("partial completion should not be complete")
	}
	if !(DailySummary{Scheduled: 2, Completed: 2}).IsComplete() {
		t.Error("Completed == Scheduled should be complete")
	}
}

func habitFixture(id int64, createdOn string) Habit {
	return Habit{ID: id, Title: "h", Schedule: DailySchedule(), CreatedOn: MustParseDate(createdOn)}
}

func TestSummarizeDay(t *testing.T) {
	h1 := habitFixture(1, "2026-08-01")
	h2 := habitFixture(2, "2026-08-01")
	habits := []Habit{h1, h2}
	d := MustParseDate("2026-08-10")

	got := SummarizeDay(habits, map[int64]bool{1: true}, d)
	want := DailySummary{Date: d, Scheduled: 2, Completed: 1}
	if got != want {
		t.Errorf("SummarizeDay = %+v, want %+v", got, want)
	}
}

func TestSummarizeDayExcludesInactiveHabits(t *testing.T) {
	future := habitFixture(1, "2030-01-01")
	d := MustParseDate("2026-08-10")
	got := SummarizeDay([]Habit{future}, nil, d)
	if got.Scheduled != 0 {
		t.Errorf("expected habit created in the future to not be scheduled, got Scheduled=%d", got.Scheduled)
	}
}

func TestSummarizeRange(t *testing.T) {
	h := habitFixture(1, "2026-08-01")
	comps := []Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-01"), CompletedAt: time.Now()},
		{HabitID: 1, Date: MustParseDate("2026-08-03"), CompletedAt: time.Now()},
	}
	got := SummarizeRange([]Habit{h}, comps, MustParseDate("2026-08-01"), MustParseDate("2026-08-03"))
	if len(got) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(got))
	}
	if !got[0].IsComplete() {
		t.Error("2026-08-01 should be complete")
	}
	if got[1].IsComplete() {
		t.Error("2026-08-02 should not be complete")
	}
	if !got[2].IsComplete() {
		t.Error("2026-08-03 should be complete")
	}
}

func TestCompletionIndex(t *testing.T) {
	comps := []Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-01")},
		{HabitID: 2, Date: MustParseDate("2026-08-01")},
		{HabitID: 1, Date: MustParseDate("2026-08-02")},
	}
	idx := CompletionIndex(comps)
	if !idx[MustParseDate("2026-08-01")][1] || !idx[MustParseDate("2026-08-01")][2] {
		t.Error("expected both habits indexed on 2026-08-01")
	}
	if !idx[MustParseDate("2026-08-02")][1] {
		t.Error("expected habit 1 indexed on 2026-08-02")
	}
	if idx[MustParseDate("2026-08-02")][2] {
		t.Error("did not expect habit 2 indexed on 2026-08-02")
	}
}

func TestHabitStreakIntactRun(t *testing.T) {
	h := habitFixture(1, "2026-08-01")
	comps := CompletionIndex([]Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-08")},
		{HabitID: 1, Date: MustParseDate("2026-08-09")},
		{HabitID: 1, Date: MustParseDate("2026-08-10")},
	})
	if got := HabitStreak(h, comps, MustParseDate("2026-08-10")); got != 3 {
		t.Errorf("HabitStreak = %d, want 3", got)
	}
}

func TestHabitStreakTodayIncompleteDoesNotBreak(t *testing.T) {
	h := habitFixture(1, "2026-08-01")
	// completed the two days before upTo, but not upTo itself.
	comps := CompletionIndex([]Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-08")},
		{HabitID: 1, Date: MustParseDate("2026-08-09")},
	})
	if got := HabitStreak(h, comps, MustParseDate("2026-08-10")); got != 2 {
		t.Errorf("HabitStreak = %d, want 2 (today's incompleteness should not break the streak)", got)
	}
}

func TestHabitStreakBreaksOnPastIncompleteDay(t *testing.T) {
	h := habitFixture(1, "2026-08-01")
	// 08-07 is scheduled but missing -> breaks the streak before it.
	comps := CompletionIndex([]Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-05")},
		{HabitID: 1, Date: MustParseDate("2026-08-06")},
		{HabitID: 1, Date: MustParseDate("2026-08-08")},
		{HabitID: 1, Date: MustParseDate("2026-08-09")},
		{HabitID: 1, Date: MustParseDate("2026-08-10")},
	})
	if got := HabitStreak(h, comps, MustParseDate("2026-08-10")); got != 3 {
		t.Errorf("HabitStreak = %d, want 3 (only days after the 08-07 gap)", got)
	}
}

func TestHabitStreakSkipsUnscheduledDays(t *testing.T) {
	// Scheduled only on Monday (2026-08-10 and 2026-08-03 are Mondays).
	h := Habit{
		ID:        1,
		Title:     "weekly",
		Schedule:  Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(time.Monday)},
		CreatedOn: MustParseDate("2026-07-01"),
	}
	comps := CompletionIndex([]Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-03")},
		{HabitID: 1, Date: MustParseDate("2026-08-10")},
	})
	if got := HabitStreak(h, comps, MustParseDate("2026-08-10")); got != 2 {
		t.Errorf("HabitStreak = %d, want 2 (unscheduled days between Mondays should not break it)", got)
	}
}

func TestHabitStreakStopsAtCreation(t *testing.T) {
	h := habitFixture(1, "2026-08-09")
	comps := CompletionIndex([]Completion{
		{HabitID: 1, Date: MustParseDate("2026-08-09")},
		{HabitID: 1, Date: MustParseDate("2026-08-10")},
	})
	if got := HabitStreak(h, comps, MustParseDate("2026-08-10")); got != 2 {
		t.Errorf("HabitStreak = %d, want 2", got)
	}
}

func TestCurrentStreakSkipsZeroScheduledDays(t *testing.T) {
	today := MustParseDate("2026-08-10")
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-08"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-09"), Scheduled: 0, Completed: 0}, // nothing scheduled, should be skipped
		{Date: MustParseDate("2026-08-10"), Scheduled: 2, Completed: 2},
	}
	if got := CurrentStreak(summaries, today); got != 2 {
		t.Errorf("CurrentStreak = %d, want 2", got)
	}
}

func TestCurrentStreakTodayIncompleteDoesNotBreak(t *testing.T) {
	today := MustParseDate("2026-08-10")
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-09"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-10"), Scheduled: 2, Completed: 1}, // today, incomplete
	}
	if got := CurrentStreak(summaries, today); got != 1 {
		t.Errorf("CurrentStreak = %d, want 1", got)
	}
}

func TestCurrentStreakBreaksOnPastIncompleteDay(t *testing.T) {
	today := MustParseDate("2026-08-10")
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-08"), Scheduled: 2, Completed: 1}, // incomplete, breaks
		{Date: MustParseDate("2026-08-09"), Scheduled: 2, Completed: 2},
		{Date: MustParseDate("2026-08-10"), Scheduled: 2, Completed: 2},
	}
	if got := CurrentStreak(summaries, today); got != 2 {
		t.Errorf("CurrentStreak = %d, want 2", got)
	}
}

func TestBestStreak(t *testing.T) {
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-01"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-02"), Scheduled: 1, Completed: 0},
		{Date: MustParseDate("2026-08-03"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-04"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-05"), Scheduled: 0, Completed: 0}, // skipped, extends the run
		{Date: MustParseDate("2026-08-06"), Scheduled: 1, Completed: 1},
	}
	if got := BestStreak(summaries); got != 3 {
		t.Errorf("BestStreak = %d, want 3", got)
	}
}

func TestBestStreakUnordered(t *testing.T) {
	summaries := []DailySummary{
		{Date: MustParseDate("2026-08-03"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-01"), Scheduled: 1, Completed: 1},
		{Date: MustParseDate("2026-08-02"), Scheduled: 1, Completed: 1},
	}
	if got := BestStreak(summaries); got != 3 {
		t.Errorf("BestStreak = %d, want 3 (input order should not matter)", got)
	}
}
