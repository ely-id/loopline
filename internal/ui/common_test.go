package ui

import "testing"

func seqLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = string(rune('a' + i))
	}
	return lines
}

func TestWindowAroundReturnsAllLinesWhenTheyFit(t *testing.T) {
	lines := []string{"a", "b", "c"}
	got := windowAround(lines, 5, 1)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want all 3 unclipped", len(got))
	}
}

func TestWindowAroundKeepsFocusVisibleWhenClipped(t *testing.T) {
	lines := seqLines(20)
	focus := 15

	got := windowAround(lines, 5, focus)
	if len(got) != 5 {
		t.Fatalf("got %d lines, want 5", len(got))
	}
	found := false
	for _, l := range got {
		if l == lines[focus] {
			found = true
		}
	}
	if !found {
		t.Fatalf("windowed lines %v do not include the focused line %q", got, lines[focus])
	}
}

func TestWindowAroundClampsAtEitherEnd(t *testing.T) {
	lines := seqLines(10)

	got := windowAround(lines, 4, 0) // focus near the start must not run off the top
	if got[0] != lines[0] {
		t.Fatalf("window near start = %v, want to start at lines[0]", got)
	}

	got = windowAround(lines, 4, 9) // focus near the end must not run off the bottom
	if got[len(got)-1] != lines[9] {
		t.Fatalf("window near end = %v, want to end at lines[9]", got)
	}
}

func TestWindowAroundNoFocusStartsAtTop(t *testing.T) {
	lines := seqLines(10)
	got := windowAround(lines, 3, -1)
	if got[0] != lines[0] {
		t.Fatalf("no-anchor window = %v, want to start at top", got)
	}
}

func TestWindowAroundNonPositiveHeightReturnsNil(t *testing.T) {
	if got := windowAround(seqLines(5), 0, 2); got != nil {
		t.Fatalf("height 0 = %v, want nil", got)
	}
}

func TestWindowAroundRangeKeepsExpandedItemVisible(t *testing.T) {
	lines := seqLines(20)
	got := windowAroundRange(lines, 8, 9, 14)
	if got[0] > lines[9] || got[len(got)-1] < lines[13] {
		t.Fatalf("window %v does not contain full selected span %v", got, lines[9:14])
	}
}

func TestWindowAroundRangeAnchorsOversizedExpansionAtSelection(t *testing.T) {
	lines := seqLines(20)
	got := windowAroundRange(lines, 5, 9, 18)
	if got[0] != lines[9] {
		t.Fatalf("oversized expansion starts at %q, want selected row %q", got[0], lines[9])
	}
}
