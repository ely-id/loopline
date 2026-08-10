package domain

import (
	"testing"
	"time"
)

func TestWeekdaySetHasAndWith(t *testing.T) {
	var w WeekdaySet
	if w.Has(time.Monday) {
		t.Fatal("empty set should not have Monday")
	}
	w = w.With(time.Monday)
	if !w.Has(time.Monday) {
		t.Fatal("set should have Monday after With")
	}
	if w.Has(time.Tuesday) {
		t.Fatal("set should not have Tuesday")
	}
}

func TestWeekdaysOf(t *testing.T) {
	w := WeekdaysOf(time.Monday, time.Wednesday, time.Friday)
	for _, d := range []time.Weekday{time.Monday, time.Wednesday, time.Friday} {
		if !w.Has(d) {
			t.Errorf("expected set to have %v", d)
		}
	}
	for _, d := range []time.Weekday{time.Sunday, time.Tuesday, time.Thursday, time.Saturday} {
		if w.Has(d) {
			t.Errorf("expected set not to have %v", d)
		}
	}
}

func TestDailyScheduleAppliesEveryDay(t *testing.T) {
	s := DailySchedule()
	for _, ds := range []string{"2026-08-10", "2026-08-11", "2026-08-16"} {
		if !s.AppliesOn(MustParseDate(ds)) {
			t.Errorf("daily schedule should apply on %s", ds)
		}
	}
}

func TestWeekdayScheduleAppliesOnlyOnSelectedDays(t *testing.T) {
	s := Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(time.Monday, time.Wednesday, time.Friday)}
	if !s.AppliesOn(MustParseDate("2026-08-10")) { // Monday
		t.Error("expected schedule to apply on Monday")
	}
	if s.AppliesOn(MustParseDate("2026-08-11")) { // Tuesday
		t.Error("expected schedule not to apply on Tuesday")
	}
}

func TestScheduleValidate(t *testing.T) {
	if err := DailySchedule().Validate(); err != nil {
		t.Errorf("daily schedule should be valid: %v", err)
	}
	if err := (Schedule{Kind: ScheduleWeekdays, Weekdays: 0}).Validate(); err == nil {
		t.Error("weekday schedule with no days should be invalid")
	}
	if err := (Schedule{Kind: "bogus"}).Validate(); err == nil {
		t.Error("unknown schedule kind should be invalid")
	}
	if err := (Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(time.Monday)}).Validate(); err != nil {
		t.Errorf("weekday schedule with one day should be valid: %v", err)
	}
}

func TestScheduleString(t *testing.T) {
	if got := DailySchedule().String(); got != "daily" {
		t.Errorf("DailySchedule().String() = %q, want %q", got, "daily")
	}
	s := Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(time.Monday, time.Wednesday, time.Friday)}
	if got, want := s.String(), "mo we fr"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	single := Schedule{Kind: ScheduleWeekdays, Weekdays: WeekdaysOf(time.Sunday)}
	if got, want := single.String(), "su"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
