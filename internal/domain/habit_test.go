package domain

import "testing"

func TestHabitActiveOn(t *testing.T) {
	created := MustParseDate("2026-08-01")
	archived := MustParseDate("2026-08-10")
	h := Habit{Title: "x", Schedule: DailySchedule(), CreatedOn: created, ArchivedOn: &archived}

	if h.ActiveOn(MustParseDate("2026-07-31")) {
		t.Error("should not be active before creation")
	}
	if !h.ActiveOn(created) {
		t.Error("should be active on creation date")
	}
	if !h.ActiveOn(MustParseDate("2026-08-09")) {
		t.Error("should be active the day before archiving")
	}
	if h.ActiveOn(archived) {
		t.Error("should not be active on the archive date")
	}
	if h.ActiveOn(MustParseDate("2026-08-11")) {
		t.Error("should not be active after the archive date")
	}
}

func TestHabitActiveOnNeverArchived(t *testing.T) {
	h := Habit{Title: "x", Schedule: DailySchedule(), CreatedOn: MustParseDate("2026-08-01")}
	if !h.ActiveOn(MustParseDate("2030-01-01")) {
		t.Error("habit with no archive date should stay active indefinitely")
	}
}

func TestHabitScheduledOnCombinesActiveAndSchedule(t *testing.T) {
	h := Habit{
		Title:     "gym",
		Schedule:  Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(MustParseDate("2026-08-10").Weekday())},
		CreatedOn: MustParseDate("2026-08-01"),
	}
	if !h.ScheduledOn(MustParseDate("2026-08-10")) {
		t.Error("expected scheduled on matching weekday within active window")
	}
	if h.ScheduledOn(MustParseDate("2026-07-01")) {
		t.Error("should not be scheduled before creation even on a matching weekday")
	}
}

func TestHabitValidate(t *testing.T) {
	valid := Habit{Title: "Drink water", Schedule: DailySchedule()}
	if err := valid.Validate(); err != nil {
		t.Errorf("expected valid habit, got error: %v", err)
	}

	noTitle := Habit{Title: "", Schedule: DailySchedule()}
	if err := noTitle.Validate(); err == nil {
		t.Error("expected error for empty title")
	}

	badSchedule := Habit{Title: "x", Schedule: Schedule{Kind: ScheduleWeekdays, Weekdays: 0}}
	if err := badSchedule.Validate(); err == nil {
		t.Error("expected error for invalid schedule")
	}
}
