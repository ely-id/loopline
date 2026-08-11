package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// bigHeatmapBackend wraps fakeBackend and replaces the Heatmap it returns
// with exactly `weeks` week-columns (oldest first), so tests can exercise
// ranges (year/all) far longer than fakeBackend.Snapshot's built-in
// one-week window without needing a real app.Service. The Monday cell of
// the very first (oldest) week is given a full completion so tests can
// check whether that week survives rendering.
type bigHeatmapBackend struct {
	*fakeBackend
	weeks int
}

func (b *bigHeatmapBackend) Snapshot(ctx context.Context, req app.SnapshotRequest) (app.Snapshot, error) {
	snap, err := b.fakeBackend.Snapshot(ctx, req)
	if err != nil {
		return snap, err
	}

	today := b.Today()
	start := today.StartOfWeek().AddDays(-7 * (b.weeks - 1))
	cols := make([][]app.HeatmapCell, b.weeks)
	for w := 0; w < b.weeks; w++ {
		weekStart := start.AddDays(7 * w)
		col := make([]app.HeatmapCell, 7)
		for d := 0; d < 7; d++ {
			date := weekStart.AddDays(d)
			if date.After(today) {
				continue // zero-Date padding, same as fakeBackend leaves for out-of-range cells
			}
			var sum domain.DailySummary
			if w == 0 && d == 0 {
				sum = domain.DailySummary{Scheduled: 1, Completed: 1} // full completion, oldest week
			}
			col[d] = app.HeatmapCell{Date: date, Summary: sum, Level: fallbackLevel(sum)}
		}
		cols[w] = col
	}
	snap.Heatmap = app.Heatmap{Range: req.Range, Start: start, End: today, Columns: cols}
	return snap, nil
}

var _ Backend = (*bigHeatmapBackend)(nil)

// newReadyRootFromBackend is newReadyRoot (model_test.go), generalized to any
// Backend: newReadyRoot itself is typed to *fakeBackend, which
// bigHeatmapBackend deliberately isn't (it needs to override Snapshot).
func newReadyRootFromBackend(t *testing.T, backend Backend) *Root {
	t.Helper()
	m := New(backend)
	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = step(t, m, m.Init())
	if !m.ready {
		t.Fatal("root never became ready after Init")
	}
	return m
}

// heatmapCellLines returns just the 7 weekday-row lines from a rendered,
// ANSI-stripped renderHeatmap block: index 0 is the title, index 1 is the
// hint/blank line, and indexes 2..8 are the seven cell rows.
func heatmapCellLines(t *testing.T, rendered string) []string {
	t.Helper()
	lines := strings.Split(rendered, "\n")
	if len(lines) < 9 {
		t.Fatalf("rendered heatmap has %d lines, want at least 9 (title, hint, 7 rows): %q", len(lines), rendered)
	}
	return lines[2:9]
}

// TestHeatmapDownsamplePreservesOldestWeekAtWidth is the regression test for
// defect 1: with a range whose week count exceeds the columns the block has
// room for (a year, 53 weeks, at the width the overview column gets at a
// typical terminal size), the block must downsample rather than truncate,
// so every rendered row stays within budget yet the oldest week (which the
// old truncating code silently dropped) is still represented.
func TestHeatmapDownsamplePreservesOldestWeekAtWidth(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := &bigHeatmapBackend{fakeBackend: newFakeBackend(today), weeks: 53}
	root := newReadyRootFromBackend(t, backend) // sizes the window to 120x40

	const width = 35 // renderOverview's text width at 120 columns wide
	maxCols := clampInt(width-2, 1, len(root.snap.Heatmap.Columns))
	if maxCols >= len(root.snap.Heatmap.Columns) {
		t.Fatalf("fixture drifted: maxCols=%d >= %d week-columns, this test needs a range that overflows the block", maxCols, len(root.snap.Heatmap.Columns))
	}

	raw := root.renderHeatmap(width)
	rendered := stripANSI(raw)

	for i, line := range heatmapCellLines(t, rendered) {
		if n := len([]rune(line)); n > maxCols {
			t.Fatalf("rendered heatmap row %d has %d cells, want at most maxCols=%d: %q", i, n, maxCols, line)
		}
	}

	// fallbackLevel({Scheduled:1, Completed:1}) is the top bucket (level 4);
	// with nothing else in the fixture completed, that glyph can only come
	// from the aggregated oldest-week cell. The old truncating
	// implementation kept only the newest maxCols columns and would drop
	// week 0 entirely, so this glyph would be absent.
	wantGlyph := theme.HeatmapGlyphs[4]
	if !strings.ContainsRune(rendered, wantGlyph) {
		t.Fatalf("rendered heatmap has no level-4 glyph %q; the oldest week's completion was dropped instead of downsampled:\n%s", wantGlyph, rendered)
	}

	if !strings.Contains(raw, "w/col)") {
		t.Fatalf("title should note the compression ratio when columns are merged, got: %q", stripANSI(raw))
	}
}

// TestHeatmapNoDownsampleWhenRangeFitsColumns checks the k==1 path (range
// fits in the available columns) is untouched: no "(Nw/col)" suffix, and the
// number of rendered columns matches the number of week-columns in the
// snapshot exactly (nothing merged, nothing truncated).
func TestHeatmapNoDownsampleWhenRangeFitsColumns(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	const width = 35

	// fakeBackend's built-in Snapshot always returns exactly one week of
	// heatmap columns, well within budget for any range selector.
	for _, rng := range []app.StatsRange{app.RangeWeek, app.RangeMonth} {
		backend := newFakeBackend(today)
		root := newReadyRoot(t, backend)
		root.statsRange = rng

		rendered := stripANSI(root.renderHeatmap(width))
		if strings.Contains(rendered, "w/col") {
			t.Fatalf("range %s: title should have no compression suffix when the range fits, got: %q", rng, rendered)
		}

		row0 := heatmapCellLines(t, rendered)[0]
		if got, want := len([]rune(row0)), len(root.snap.Heatmap.Columns); got != want {
			t.Fatalf("range %s: rendered %d columns, want %d (== len(hm.Columns), unmodified)", rng, got, want)
		}
	}

	// Boundary case: weeks exactly equal to maxCols must also take the
	// unmodified path (n <= maxCols, not n < maxCols).
	maxCols := clampInt(width-2, 1, 1<<30)
	backend := &bigHeatmapBackend{fakeBackend: newFakeBackend(today), weeks: maxCols}
	root := newReadyRootFromBackend(t, backend)

	rendered := stripANSI(root.renderHeatmap(width))
	if strings.Contains(rendered, "w/col") {
		t.Fatalf("boundary case n == maxCols should not compress, got: %q", rendered)
	}
	row0 := heatmapCellLines(t, rendered)[0]
	if got, want := len([]rune(row0)), maxCols; got != want {
		t.Fatalf("boundary case: rendered %d columns on the first cell row, want %d: %q", got, want, rendered)
	}
}

var errBoom = errors.New("boom: unreadable database")

// TestNotReadyWithStatusRendersErrorNotLoading is the regression test for
// defect 2: a snapshotErrMsg sets m.status but leaves m.ready false. Before
// the fix, View() always rendered "Loading…" whenever !ready, so a failed
// startup fetch (e.g. an unreadable database) hid the real error behind a
// permanent loading screen.
func TestNotReadyWithStatusRendersErrorNotLoading(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	backend.snapshotErr = errBoom

	m := New(backend)
	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = step(t, m, m.Init())

	if m.ready {
		t.Fatal("setup: root should not become ready when Snapshot fails")
	}
	if m.status == "" {
		t.Fatal("setup: m.status should be set after a failed Snapshot")
	}

	content := m.View().Content
	if strings.Contains(content, "Loading") {
		t.Fatalf("View() with !ready and a status should not show Loading…, got: %q", content)
	}
	if !strings.Contains(content, m.status) {
		t.Fatalf("View() with !ready and a status should show the error %q, got: %q", m.status, content)
	}
}
