// Package domain holds entities and pure calculations for routine-tui.
//
// This package depends only on the Go standard library. It must never import
// storage, UI or application packages.
package domain

import (
	"fmt"
	"time"
)

// Date is a timezone-independent local calendar date. It is the single date
// representation used across the whole application: storage, statistics and
// rendering all speak Date, never time.Time.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateLayout is the canonical textual form of a Date, also used by SQLite.
const DateLayout = "2006-01-02"

// NewDate builds a normalised Date (out-of-range components roll over, e.g.
// NewDate(2026, time.January, 32) is 2026-02-01).
func NewDate(year int, month time.Month, day int) Date {
	return DateFromTime(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

// DateFromTime takes the calendar date of t in t's own location.
func DateFromTime(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate parses a date in DateLayout form.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return DateFromTime(t), nil
}

// MustParseDate is ParseDate for tests and constants.
func MustParseDate(s string) Date {
	d, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return d
}

// String renders the date in DateLayout form.
func (d Date) String() string {
	return d.time().Format(DateLayout)
}

// IsZero reports whether the date is unset.
func (d Date) IsZero() bool { return d == Date{} }

// time returns midnight UTC for the date. UTC keeps arithmetic free of DST
// discontinuities; the calendar result is identical in any location.
func (d Date) time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// Time returns midnight UTC for the date, for callers that need a time.Time.
func (d Date) Time() time.Time { return d.time() }

// Weekday returns the day of the week.
func (d Date) Weekday() time.Weekday { return d.time().Weekday() }

// AddDays returns the date n days later (n may be negative).
func (d Date) AddDays(n int) Date { return DateFromTime(d.time().AddDate(0, 0, n)) }

// AddMonths returns the date n months later, normalised (Jan 31 + 1 month is
// March 3 or 2 depending on the year, matching time.Time semantics).
func (d Date) AddMonths(n int) Date { return DateFromTime(d.time().AddDate(0, n, 0)) }

// Compare orders two dates: -1 if d is earlier, 0 if equal, +1 if later.
func (d Date) Compare(o Date) int { return d.time().Compare(o.time()) }

// Before reports whether d is earlier than o.
func (d Date) Before(o Date) bool { return d.Compare(o) < 0 }

// After reports whether d is later than o.
func (d Date) After(o Date) bool { return d.Compare(o) > 0 }

// Equal reports whether the two dates are the same calendar day.
func (d Date) Equal(o Date) bool { return d == o }

// DaysSince returns the whole number of days from o to d.
func (d Date) DaysSince(o Date) int {
	return int(d.time().Sub(o.time()).Hours() / 24)
}

// ISOWeek returns the ISO 8601 year and week number.
func (d Date) ISOWeek() (year, week int) { return d.time().ISOWeek() }

// StartOfWeek returns the Monday of the ISO week containing d.
func (d Date) StartOfWeek() Date {
	offset := (int(d.Weekday()) + 6) % 7 // Monday == 0
	return d.AddDays(-offset)
}

// StartOfMonth returns the first day of the month containing d.
func (d Date) StartOfMonth() Date { return Date{Year: d.Year, Month: d.Month, Day: 1} }

// EndOfMonth returns the last day of the month containing d.
func (d Date) EndOfMonth() Date { return d.StartOfMonth().AddMonths(1).AddDays(-1) }

// DateRange lists every date from start to end inclusive. It returns nil when
// end is before start.
func DateRange(start, end Date) []Date {
	if end.Before(start) {
		return nil
	}
	out := make([]Date, 0, end.DaysSince(start)+1)
	for d := start; !d.After(end); d = d.AddDays(1) {
		out = append(out, d)
	}
	return out
}

// Clock provides the application's notion of "now". Every date-dependent
// decision flows through a Clock; packages must not call time.Now directly.
type Clock interface {
	// Today is the current local calendar date.
	Today() Date
	// Now is the current local wall-clock time, for completion timestamps.
	Now() time.Time
}

// SystemClock reads the host clock in the machine's local timezone.
type SystemClock struct{}

// Today implements Clock.
func (SystemClock) Today() Date { return DateFromTime(time.Now().Local()) }

// Now implements Clock.
func (SystemClock) Now() time.Time { return time.Now().Local() }

// FixedClock is a deterministic Clock for tests.
type FixedClock struct {
	Date Date
	At   time.Time
}

// NewFixedClock builds a FixedClock pinned to midday on the given date.
func NewFixedClock(d Date) *FixedClock {
	return &FixedClock{Date: d, At: time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC)}
}

// Today implements Clock.
func (c *FixedClock) Today() Date { return c.Date }

// Now implements Clock.
func (c *FixedClock) Now() time.Time { return c.At }

// Advance moves the clock forward by n days.
func (c *FixedClock) Advance(days int) {
	c.Date = c.Date.AddDays(days)
	c.At = c.At.AddDate(0, 0, days)
}
