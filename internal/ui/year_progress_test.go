package ui

import (
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/domain"
)

func TestCalculateYearProgressCommonAndLeapYears(t *testing.T) {
	cases := []struct {
		date                     string
		day, total, pct, isoWeek int
	}{
		{"2025-12-31", 365, 365, 100, 1},
		{"2024-02-29", 60, 366, 16, 9},
		{"2024-12-31", 366, 366, 100, 1},
		{"2026-08-14", 226, 365, 62, 33},
	}
	for _, tc := range cases {
		got := calculateYearProgress(domain.MustParseDate(tc.date))
		if got.Day != tc.day || got.Total != tc.total || got.Percent != tc.pct || got.ISOWeek != tc.isoWeek {
			t.Errorf("calculateYearProgress(%s) = %+v, want day=%d total=%d pct=%d week=%d", tc.date, got, tc.day, tc.total, tc.pct, tc.isoWeek)
		}
	}
}

func TestYearProgressDoesNotRoundToCompleteBeforeLastDay(t *testing.T) {
	got := calculateYearProgress(domain.MustParseDate("2025-12-30"))
	if got.Percent != 99 {
		t.Fatalf("Dec 30 percent = %d, want 99 rather than a premature 100", got.Percent)
	}
}

func TestYearProgressFitsMinimumOverviewWidth(t *testing.T) {
	root := rootWithGroups()
	root.today = domain.MustParseDate("2024-12-31")
	rendered := stripANSI(root.renderYearProgress(27))
	for i, line := range strings.Split(rendered, "\n") {
		if width := lipgloss.Width(line); width > 27 {
			t.Fatalf("year row %d is %d cells wide, want <= 27: %q", i, width, line)
		}
	}
	for _, want := range []string{"366 / 366 days", "Week 1", "100%"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("year progress missing %q: %s", want, rendered)
		}
	}
}

func TestYearIsBottomAnchoredOnlyInReminders(t *testing.T) {
	root := newReadyRoot(t, newRichBackend(domain.MustParseDate("2026-08-10")))
	const width, height = 25, 29
	reminders := stripANSI(root.renderReminders(width, height))
	lines := strings.Split(reminders, "\n")
	if len(lines) != height {
		t.Fatalf("Reminders height = %d, want %d", len(lines), height)
	}
	wantYear := strings.Split(stripANSI(root.renderYearProgress(width)), "\n")
	for i := range wantYear {
		if lines[height-len(wantYear)+i] != wantYear[i] {
			t.Fatalf("Year is not bottom anchored: tail=%q want=%q", lines[height-len(wantYear):], wantYear)
		}
	}
	if strings.Contains(stripANSI(root.renderOverview(27, 29)), " Year") {
		t.Fatal("Year remained duplicated in Overview")
	}
}
