package domain

import (
	"testing"
	"time"
)

// ISO week boundaries, verified independently against Python's
// datetime.isocalendar() rather than derived from the code under test:
//
//	2026-01-01 (Thu) -> ISO 2026, week 1
//	2024-12-30 (Mon) -> ISO 2025, week 1
//	2021-01-03 (Sun) -> ISO 2020, week 53
//	2020-12-31 (Thu) -> ISO 2020, week 53 (2020 has 53 ISO weeks)
//	2025-12-29 (Mon) -> ISO 2026, week 1
//	2026-01-04 (Sun) -> ISO 2026, week 1
//	2026-01-05 (Mon) -> ISO 2026, week 2
func TestSpecISOWeekBoundaries(t *testing.T) {
	cases := []struct {
		date     string
		wantYear int
		wantWeek int
	}{
		{"2026-01-01", 2026, 1},
		{"2024-12-30", 2025, 1},
		{"2021-01-03", 2020, 53},
		{"2020-12-31", 2020, 53},
		{"2025-12-29", 2026, 1},
		{"2026-01-04", 2026, 1},
		{"2026-01-05", 2026, 2},
	}
	for _, c := range cases {
		t.Run(c.date, func(t *testing.T) {
			d := MustParseDate(c.date)
			gotYear, gotWeek := d.ISOWeek()
			if gotYear != c.wantYear || gotWeek != c.wantWeek {
				t.Fatalf("ISOWeek(%s) = (%d, %d), want (%d, %d)", c.date, gotYear, gotWeek, c.wantYear, c.wantWeek)
			}
		})
	}
}

func TestSpecStartOfWeekAlwaysMonday(t *testing.T) {
	// One date for every weekday, plus the ISO-boundary dates above.
	dates := []string{
		"2026-08-10", "2026-08-11", "2026-08-12", "2026-08-13",
		"2026-08-14", "2026-08-15", "2026-08-16",
		"2026-01-01", "2024-12-30", "2021-01-03", "2020-12-31",
	}
	for _, ds := range dates {
		t.Run(ds, func(t *testing.T) {
			d := MustParseDate(ds)
			sow := d.StartOfWeek()
			if sow.Weekday() != time.Monday {
				t.Fatalf("StartOfWeek(%s) = %s, weekday %s, want Monday", ds, sow, sow.Weekday())
			}
			// StartOfWeek must fall within 6 days before d and be idempotent.
			if sow.After(d) {
				t.Fatalf("StartOfWeek(%s) = %s is after the input date", ds, sow)
			}
			if diff := d.DaysSince(sow); diff < 0 || diff > 6 {
				t.Fatalf("StartOfWeek(%s) = %s is %d days from input, want 0..6", ds, sow, diff)
			}
			if again := sow.StartOfWeek(); !again.Equal(sow) {
				t.Fatalf("StartOfWeek not idempotent: StartOfWeek(%s)=%s, StartOfWeek(that)=%s", ds, sow, again)
			}
		})
	}

	// Explicit known values.
	if got := MustParseDate("2026-01-01").StartOfWeek(); got != MustParseDate("2025-12-29") {
		t.Fatalf("StartOfWeek(2026-01-01) = %s, want 2025-12-29", got)
	}
	if got := MustParseDate("2021-01-03").StartOfWeek(); got != MustParseDate("2020-12-28") {
		t.Fatalf("StartOfWeek(2021-01-03) = %s, want 2020-12-28", got)
	}
	// A Monday is its own start of week.
	if got := MustParseDate("2026-01-05").StartOfWeek(); got != MustParseDate("2026-01-05") {
		t.Fatalf("StartOfWeek(Monday 2026-01-05) = %s, want itself", got)
	}
}

func TestSpecEndOfMonthFebruary(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2024-02-15", "2024-02-29"}, // leap year
		{"2025-02-01", "2025-02-28"}, // non-leap year
		{"2000-02-10", "2000-02-29"}, // divisible by 400 -> leap
		{"1900-02-10", "1900-02-28"}, // divisible by 100 but not 400 -> not leap
		{"2026-02-01", "2026-02-28"}, // non-leap year (current year context)
	}
	for _, c := range cases {
		t.Run(c.date, func(t *testing.T) {
			got := MustParseDate(c.date).EndOfMonth()
			want := MustParseDate(c.want)
			if got != want {
				t.Fatalf("EndOfMonth(%s) = %s, want %s", c.date, got, want)
			}
		})
	}
}

func TestSpecAddDaysNormalisation(t *testing.T) {
	cases := []struct {
		date string
		n    int
		want string
	}{
		{"2026-01-31", 1, "2026-02-01"},
		{"2026-02-28", 1, "2026-03-01"},  // non-leap Feb rollover
		{"2024-02-28", 1, "2024-02-29"},  // leap Feb
		{"2024-02-29", 1, "2024-03-01"},  // leap day rollover
		{"2026-12-31", 1, "2027-01-01"},  // year rollover forward
		{"2026-01-01", -1, "2025-12-31"}, // year rollover backward
		{"2026-08-10", 0, "2026-08-10"},  // identity
		{"2026-08-10", -365, "2025-08-10"},
	}
	for _, c := range cases {
		t.Run(c.date, func(t *testing.T) {
			got := MustParseDate(c.date).AddDays(c.n)
			want := MustParseDate(c.want)
			if got != want {
				t.Fatalf("AddDays(%s, %d) = %s, want %s", c.date, c.n, got, want)
			}
		})
	}
}

func TestSpecAddMonthsNormalisation(t *testing.T) {
	cases := []struct {
		date string
		n    int
		want string
	}{
		{"2026-01-31", 1, "2026-03-03"},  // Feb 2026 has 28 days: 31-28=3 -> Mar 3
		{"2024-01-31", 1, "2024-03-02"},  // Feb 2024 has 29 days: 31-29=2 -> Mar 2
		{"2026-03-31", -1, "2026-03-03"}, // Mar31 - 1 month = "Feb 31" -> Mar 3 (non-leap)
		{"2026-01-15", 12, "2027-01-15"}, // full year
		{"2026-08-10", -8, "2025-12-10"}, // year rollover backward
		{"2026-08-10", 0, "2026-08-10"},  // identity
	}
	for _, c := range cases {
		t.Run(c.date, func(t *testing.T) {
			got := MustParseDate(c.date).AddMonths(c.n)
			want := MustParseDate(c.want)
			if got != want {
				t.Fatalf("AddMonths(%s, %d) = %s, want %s", c.date, c.n, got, want)
			}
		})
	}
}

func TestSpecDaysSinceSymmetry(t *testing.T) {
	pairs := []struct{ a, b string }{
		{"2026-08-10", "2026-08-01"},
		{"2026-01-01", "2025-12-25"},
		{"2024-03-01", "2024-02-28"}, // spans leap day
		{"2026-08-10", "2026-08-10"}, // same date
	}
	for _, p := range pairs {
		t.Run(p.a+"_"+p.b, func(t *testing.T) {
			a, b := MustParseDate(p.a), MustParseDate(p.b)
			forward := a.DaysSince(b)
			backward := b.DaysSince(a)
			if forward != -backward {
				t.Fatalf("DaysSince not symmetric: %s.DaysSince(%s)=%d, %s.DaysSince(%s)=%d", p.a, p.b, forward, p.b, p.a, backward)
			}
			if b.AddDays(forward) != a {
				t.Fatalf("%s.AddDays(%d) = %s, want %s", p.b, forward, b.AddDays(forward), p.a)
			}
		})
	}
}

func TestSpecParseDateRoundTrip(t *testing.T) {
	dates := []string{
		"2026-08-10", "0001-01-01", "9999-12-31", "2024-02-29", "2026-01-01",
	}
	for _, ds := range dates {
		t.Run(ds, func(t *testing.T) {
			d, err := ParseDate(ds)
			if err != nil {
				t.Fatalf("ParseDate(%s) error: %v", ds, err)
			}
			if got := d.String(); got != ds {
				t.Fatalf("round trip: ParseDate(%s).String() = %s", ds, got)
			}
		})
	}
}

func TestSpecParseDateInvalid(t *testing.T) {
	invalid := []string{"", "not-a-date", "2026-13-01", "2026-02-30", "08/10/2026", "2026-8-10"}
	for _, s := range invalid {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseDate(s); err == nil {
				t.Fatalf("ParseDate(%q) expected an error, got none", s)
			}
		})
	}
}

func TestSpecDateRangeInclusive(t *testing.T) {
	start := MustParseDate("2026-08-08")
	end := MustParseDate("2026-08-10")
	got := DateRange(start, end)
	want := []Date{
		MustParseDate("2026-08-08"),
		MustParseDate("2026-08-09"),
		MustParseDate("2026-08-10"),
	}
	if len(got) != len(want) {
		t.Fatalf("DateRange length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DateRange[%d] = %s, want %s", i, got[i], want[i])
		}
	}

	// Single-day range.
	single := DateRange(start, start)
	if len(single) != 1 || single[0] != start {
		t.Fatalf("DateRange(start, start) = %v, want [%s]", single, start)
	}
}

func TestSpecDateRangeEmptyWhenEndBeforeStart(t *testing.T) {
	start := MustParseDate("2026-08-10")
	end := MustParseDate("2026-08-09")
	got := DateRange(start, end)
	if got != nil {
		t.Fatalf("DateRange(end < start) = %v, want nil", got)
	}
}

func TestSpecFixedClock(t *testing.T) {
	d := MustParseDate("2026-08-10")
	c := NewFixedClock(d)

	if got := c.Today(); got != d {
		t.Fatalf("Today() = %s, want %s", got, d)
	}
	if now := c.Now(); DateFromTime(now) != d {
		t.Fatalf("Now() date = %s, want %s", DateFromTime(now), d)
	}
	if now := c.Now(); now.Hour() != 12 || now.Minute() != 0 {
		t.Fatalf("NewFixedClock should pin midday, got %s", now)
	}

	// A Clock-typed reference must still work (interface satisfaction).
	var clock Clock = c
	if clock.Today() != d {
		t.Fatalf("Clock.Today() via interface = %s, want %s", clock.Today(), d)
	}

	c.Advance(1)
	want := d.AddDays(1)
	if c.Today() != want {
		t.Fatalf("after Advance(1), Today() = %s, want %s", c.Today(), want)
	}
	if DateFromTime(c.Now()) != want {
		t.Fatalf("after Advance(1), Now() date = %s, want %s", DateFromTime(c.Now()), want)
	}

	// Advance across a month/year boundary.
	c2 := NewFixedClock(MustParseDate("2026-12-31"))
	c2.Advance(1)
	if c2.Today() != MustParseDate("2027-01-01") {
		t.Fatalf("Advance across year boundary = %s, want 2027-01-01", c2.Today())
	}

	// Negative advance moves backward.
	c3 := NewFixedClock(MustParseDate("2026-08-10"))
	c3.Advance(-10)
	if c3.Today() != MustParseDate("2026-07-31") {
		t.Fatalf("Advance(-10) = %s, want 2026-07-31", c3.Today())
	}
}

func TestSpecSystemClockSatisfiesInterface(t *testing.T) {
	var clock Clock = SystemClock{}
	today := clock.Today()
	if today.IsZero() {
		t.Fatal("SystemClock.Today() returned zero date")
	}
	if clock.Now().IsZero() {
		t.Fatal("SystemClock.Now() returned zero time")
	}
}
