package ui

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/theme"
)

// TestHuhThemeIsANSI16Only is the regression test for FIX 4: SPEC's Styling
// section requires "Prefer ANSI 16-color palette" and "Avoid hardcoded RGB
// colors unless absolutely necessary", but huh/v2 falls back to its stock
// ThemeCharm (24-bit RGB hex + 256-color indices) whenever a form is built
// without an explicit theme. This walks every lipgloss.Style embedded in
// theme.HuhTheme()'s constructed huh.Styles and asserts each Foreground and
// Background color is either unset or an ANSI 0-15 color — never a
// lipgloss.Color("#...") hex value and never a 256-color index above 15.
func TestHuhThemeIsANSI16Only(t *testing.T) {
	styles := theme.HuhTheme().Theme(true)
	if styles == nil {
		t.Fatal("theme.HuhTheme().Theme(true) returned a nil *huh.Styles")
	}
	walkStyles(t, "Styles", reflect.ValueOf(*styles))
}

// walkStyles recursively visits every exported struct field reachable from
// v. Any field whose dynamic type is lipgloss.Style is checked directly;
// any other struct field is recursed into so nested style groups (huh's
// FieldStyles, TextInputStyles, bubbles/v2's help.Styles, ...) are all
// covered without needing to name each of their types here.
func walkStyles(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return
	}
	if style, ok := v.Interface().(lipgloss.Style); ok {
		checkANSI16Style(t, path, style)
		return
	}
	vt := v.Type()
	for i := 0; i < vt.NumField(); i++ {
		f := vt.Field(i)
		if f.PkgPath != "" { // unexported field: not part of the public style tree
			continue
		}
		walkStyles(t, path+"."+f.Name, v.Field(i))
	}
}

// checkANSI16Style fails t if s's Foreground or Background is a hardcoded
// RGB color or a 256-color-palette index above 15. The only colors
// theme.HuhTheme() (or huh.ThemeBase, which it builds on) may produce are
// ansi.BasicColor (ANSI 0-15) or lipgloss.NoColor (unset, terminal default).
func checkANSI16Style(t *testing.T, path string, s lipgloss.Style) {
	t.Helper()
	for _, c := range []struct {
		field string
		col   color.Color
	}{
		{"Foreground", s.GetForeground()},
		{"Background", s.GetBackground()},
	} {
		if c.col == nil {
			continue
		}
		switch typeName := reflect.TypeOf(c.col).String(); typeName {
		case "ansi.BasicColor", "lipgloss.NoColor":
			// ANSI 0-15, or genuinely unset: both fine.
		default:
			t.Errorf("%s.%s uses a non-ANSI-16 color: type=%s value=%#v (RGB hex and 256-color indices above 15 are not allowed)",
				path, c.field, typeName, c.col)
		}
	}
}

// TestHuhThemeRenderedOutputHasNoRawHexMarker is a cheap, independent
// sanity check alongside TestHuhThemeIsANSI16Only: sample a handful of the
// theme's colored styles, render them, and confirm none of the ANSI escape
// sequences (or literal text) contains a "#", the tell-tale marker of a
// lipgloss.Color("#RRGGBB") hex value leaking through.
func TestHuhThemeRenderedOutputHasNoRawHexMarker(t *testing.T) {
	styles := theme.HuhTheme().Theme(true)
	samples := []lipgloss.Style{
		styles.Focused.Title,
		styles.Focused.Description,
		styles.Focused.SelectedOption,
		styles.Focused.UnselectedOption,
		styles.Focused.ErrorMessage,
		styles.Focused.FocusedButton,
		styles.Focused.BlurredButton,
		styles.Help.ShortKey,
		styles.Help.ShortDesc,
	}
	var rendered strings.Builder
	for _, s := range samples {
		rendered.WriteString(s.Render("sample"))
	}
	if strings.Contains(rendered.String(), "#") {
		t.Fatalf("rendered huh theme sample contains a raw '#': %q", rendered.String())
	}
}
