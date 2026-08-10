package ui

import (
	"strings"
	"testing"

	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// TestSpecHeatmapLevelZeroRendersVisibleGlyphNotBlank is a regression guard
// for a real bug: HeatmapGlyphs[0] used to be a bare space, and a
// Foreground-only lipgloss style paints nothing over a blank rune — so every
// "nothing scheduled/done" cell (which, before any habit history exists, is
// every cell) rendered as invisible whitespace, making the entire
// contribution/heatmap block (SPEC.md's "Block 4") silently disappear. See
// theme.go's HeatmapGlyphs doc comment: level 0 must still be a visible
// glyph.
func TestSpecHeatmapLevelZeroRendersVisibleGlyphNotBlank(t *testing.T) {
	if theme.HeatmapGlyphs[0] == ' ' || theme.HeatmapGlyphs[0] == 0 {
		t.Fatalf("HeatmapGlyphs[0] = %q, want a visible non-space, non-zero glyph (a Foreground-only style paints nothing over a blank rune)", theme.HeatmapGlyphs[0])
	}

	th := theme.New()
	rendered := stripANSI(th.HeatmapLevels[0].Render(string(theme.HeatmapGlyphs[0])))
	wantGlyph := string(theme.HeatmapGlyphs[0])
	if rendered != wantGlyph {
		t.Fatalf("level-0 heatmap style rendered %q, want exactly %q", rendered, wantGlyph)
	}

	// End-to-end: a fresh backend with no habit history at all produces an
	// all-level-0 heatmap (every cell's Summary has Scheduled=0), and the
	// rendered overview column must still show the glyph, not blank cells
	// that only look like they rendered correctly because the terminal
	// background happens to be blank too.
	today := domain.MustParseDate("2026-08-10")
	root := newReadyRoot(t, newFakeBackend(today))
	for _, col := range root.snap.Heatmap.Columns {
		for _, cell := range col {
			if !cell.Date.IsZero() && cell.Level != 0 {
				t.Fatalf("fixture drifted: expected an all-level-0 heatmap with no habits, got level %d for %s", cell.Level, cell.Date)
			}
		}
	}

	hm := stripANSI(root.renderHeatmap(40))
	if !strings.ContainsRune(hm, theme.HeatmapGlyphs[0]) {
		t.Fatalf("rendered heatmap block has no visible level-0 glyph at all: %q", hm)
	}

	content := stripANSI(root.View().Content)
	if !strings.ContainsRune(content, theme.HeatmapGlyphs[0]) {
		t.Fatalf("full rendered view has no visible level-0 heatmap glyph: %q", content)
	}
}
