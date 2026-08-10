package domain

import (
	"testing"
	"time"
)

func TestNewDateNormalises(t *testing.T) {
	got := NewDate(2026, time.January, 32)
	want := MustParseDate("2026-02-01")
	if got != want {
		t.Fatalf("NewDate(2026, Jan, 32) = %v, want %v", got, want)
	}
}

func TestParseDateRoundTrip(t *testing.T) {
	d, err := ParseDate("2026-08-10")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if d.String() != "2026-08-10" {
		t.Fatalf("String() = %q, want %q", d.String(), "2026-08-10")
	}
}

func TestParseDateInvalid(t *testing.T) {
	if _, err := ParseDate("not-a-date"); err == nil {
		t.Fatal("ParseDate: expected error for malformed input")
	}
}

func TestStartOfWeekIsMonday(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2026-08-10", "2026-08-10"}, // Monday
		{"2026-08-11", "2026-08-10"}, // Tuesday
		{"2026-08-16", "2026-08-10"}, // Sunday
	}
	for _, c := range cases {
		d := MustParseDate(c.date)
		got := d.StartOfWeek()
		want := MustParseDate(c.want)
		if got != want {
			t.Errorf("StartOfWeek(%s) = %s, want %s", c.date, got, want)
		}
		if got.Weekday() != time.Monday {
			t.Errorf("StartOfWeek(%s) weekday = %v, want Monday", c.date, got.Weekday())
		}
	}
}

func TestStartAndEndOfMonth(t *testing.T) {
	d := MustParseDate("2026-02-15")
	if got, want := d.StartOfMonth(), MustParseDate("2026-02-01"); got != want {
		t.Errorf("StartOfMonth() = %s, want %s", got, want)
	}
	if got, want := d.EndOfMonth(), MustParseDate("2026-02-28"); got != want {
		t.Errorf("EndOfMonth() = %s, want %s", got, want)
	}

	leap := MustParseDate("2028-02-01")
	if got, want := leap.EndOfMonth(), MustParseDate("2028-02-29"); got != want {
		t.Errorf("EndOfMonth() leap year = %s, want %s", got, want)
	}
}

func TestDateRangeInclusive(t *testing.T) {
	start := MustParseDate("2026-08-01")
	end := MustParseDate("2026-08-03")
	got := DateRange(start, end)
	want := []Date{
		MustParseDate("2026-08-01"),
		MustParseDate("2026-08-02"),
		MustParseDate("2026-08-03"),
	}
	if len(got) != len(want) {
		t.Fatalf("DateRange length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("DateRange[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestDateRangeEndBeforeStart(t *testing.T) {
	start := MustParseDate("2026-08-03")
	end := MustParseDate("2026-08-01")
	if got := DateRange(start, end); got != nil {
		t.Fatalf("DateRange(end before start) = %v, want nil", got)
	}
}

func TestCompareBeforeAfterEqual(t *testing.T) {
	a := MustParseDate("2026-08-01")
	b := MustParseDate("2026-08-02")
	if !a.Before(b) || a.After(b) {
		t.Errorf("expected a before b")
	}
	if !b.After(a) || b.Before(a) {
		t.Errorf("expected b after a")
	}
	if !a.Equal(a) || a.Compare(a) != 0 {
		t.Errorf("expected a equal to itself")
	}
}

func TestDaysSince(t *testing.T) {
	a := MustParseDate("2026-08-10")
	b := MustParseDate("2026-08-15")
	if got := b.DaysSince(a); got != 5 {
		t.Errorf("DaysSince = %d, want 5", got)
	}
	if got := a.DaysSince(b); got != -5 {
		t.Errorf("DaysSince (reverse) = %d, want -5", got)
	}
}

func TestFixedClockAdvance(t *testing.T) {
	c := NewFixedClock(MustParseDate("2026-08-10"))
	if c.Today() != MustParseDate("2026-08-10") {
		t.Fatalf("Today() = %s, want 2026-08-10", c.Today())
	}
	c.Advance(3)
	if c.Today() != MustParseDate("2026-08-13") {
		t.Fatalf("Today() after Advance(3) = %s, want 2026-08-13", c.Today())
	}
}

func TestIsZero(t *testing.T) {
	var d Date
	if !d.IsZero() {
		t.Error("zero Date should report IsZero")
	}
	if MustParseDate("2026-08-10").IsZero() {
		t.Error("non-zero Date should not report IsZero")
	}
}
