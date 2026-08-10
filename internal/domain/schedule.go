package domain

import (
	"fmt"
	"strings"
	"time"
)

// weekdayAbbrev maps time.Weekday order (Sunday=0..Saturday=6) to the
// two-letter abbreviation used by Schedule.String.
var weekdayAbbrev = [7]string{"su", "mo", "tu", "we", "th", "fr", "sa"}

// WeekdaysOf builds a WeekdaySet containing exactly the given weekdays.
func WeekdaysOf(days ...time.Weekday) WeekdaySet {
	var w WeekdaySet
	for _, d := range days {
		w = w.With(d)
	}
	return w
}

// Has reports whether the set contains the given weekday.
func (w WeekdaySet) Has(d time.Weekday) bool {
	return w&(1<<uint(d)) != 0
}

// With returns a copy of the set with the given weekday added.
func (w WeekdaySet) With(d time.Weekday) WeekdaySet {
	return w | (1 << uint(d))
}

// AppliesOn reports whether the schedule applies on the given calendar date.
func (s Schedule) AppliesOn(d Date) bool {
	switch s.Kind {
	case ScheduleDaily:
		return true
	case ScheduleWeekdays:
		return s.Weekdays.Has(d.Weekday())
	default:
		return false
	}
}

// Validate reports whether the schedule is well-formed.
func (s Schedule) Validate() error {
	switch s.Kind {
	case ScheduleDaily:
		return nil
	case ScheduleWeekdays:
		if s.Weekdays == 0 {
			return fmt.Errorf("weekday schedule must select at least one weekday")
		}
		if s.Weekdays&^AllWeekdays != 0 {
			return fmt.Errorf("weekday schedule has invalid weekday bits")
		}
		return nil
	default:
		return fmt.Errorf("unknown schedule kind %q", s.Kind)
	}
}

// String renders the schedule as "daily" or a space-separated list of
// two-letter weekday abbreviations in week order, e.g. "mo we fr".
func (s Schedule) String() string {
	if s.Kind == ScheduleDaily {
		return "daily"
	}
	var parts []string
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		if s.Weekdays.Has(wd) {
			parts = append(parts, weekdayAbbrev[int(wd)])
		}
	}
	return strings.Join(parts, " ")
}
