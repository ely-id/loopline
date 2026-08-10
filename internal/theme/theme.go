// Package theme holds the visual language of routine-tui: an ANSI-16 lipgloss
// palette and the named styles every renderer in internal/ui draws from.
//
// Only lipgloss.Color("0")..lipgloss.Color("15") are used, so the look
// adapts to whatever palette the user's terminal defines. Nothing here ever
// sets a background on a full-page or full-column region; the terminal's own
// background always shows through. Borders are square (lipgloss.NormalBorder).
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// ANSI 16-color palette. Named by their conventional meaning, not their
// index, so callers never have to remember "6 means cyan".
var (
	colBlack        = lipgloss.Color("0")
	colRed          = lipgloss.Color("1")
	colGreen        = lipgloss.Color("2")
	colYellow       = lipgloss.Color("3")
	colBlue         = lipgloss.Color("4")
	colMagenta      = lipgloss.Color("5")
	colCyan         = lipgloss.Color("6")
	colWhite        = lipgloss.Color("7")
	colBrightBlack  = lipgloss.Color("8")
	colBrightGreen  = lipgloss.Color("10")
	colBrightYellow = lipgloss.Color("11")
)

// Glyphs used across block renderers. Centralised here so the visual
// vocabulary of the app (what a "done" habit or a "busy" heatmap day looks
// like) stays in one place.
const (
	GlyphDone       = "x"
	GlyphPending    = " "
	GlyphSelected   = "▸"
	GlyphUnselected = " "
	GlyphReminder   = "•"
	GlyphBullet     = "·"
	ProgressFilled  = "█"
	ProgressEmpty   = "░"
)

// HeatmapGlyphs are the five density levels of a contribution cell, from
// "nothing scheduled/done" to "fully complete". Colour alone can't carry five
// steps on a 16-color palette, so density does the work and color reinforces
// it. Level 0 must still be a visible glyph (never a bare space): on an ANSI
// terminal, a Foreground-only style paints nothing over a blank rune.
var HeatmapGlyphs = [5]rune{'░', '▒', '▓', '█', '█'}

// Theme is the full set of named styles the UI renders with. Values, not
// pointers: cheap to build, safe to share, and copy-on-write per render.
type Theme struct {
	// Base colors, exposed for the rare case a block renderer needs to
	// compose a style combination theme.New doesn't already provide.
	FgDim       color.Color
	Accent      color.Color
	AccentAlt   color.Color
	Success     color.Color
	SuccessAlt  color.Color
	Warning     color.Color
	Danger      color.Color
	Border      color.Color
	BorderFocus color.Color

	// Column chrome (column = one of the three top-level regions).
	Column         lipgloss.Style
	ColumnFocused  lipgloss.Style
	ColumnTitle    lipgloss.Style
	ColumnTitleOn  lipgloss.Style
	MinSizeMessage lipgloss.Style

	// Block chrome (block = a titled group inside a column).
	BlockTitle   lipgloss.Style
	BlockTitleOn lipgloss.Style

	// Generic text.
	Text  lipgloss.Style
	Dim   lipgloss.Style
	Bold  lipgloss.Style
	Muted lipgloss.Style

	// Selectable rows.
	Item         lipgloss.Style
	ItemSelected lipgloss.Style
	Marker       lipgloss.Style

	// Habit checkboxes.
	CheckDone    lipgloss.Style
	CheckPending lipgloss.Style
	HabitDone    lipgloss.Style // completed habit title text

	// Progress bar.
	ProgressBarFilled lipgloss.Style
	ProgressBarEmpty  lipgloss.Style
	ProgressLabel     lipgloss.Style

	// Reminders.
	ReminderOverdue  lipgloss.Style
	ReminderDueSoon  lipgloss.Style
	ReminderUpcoming lipgloss.Style
	ReminderDone     lipgloss.Style
	ReminderDot      lipgloss.Style

	// Calendar / week grid.
	CalToday    lipgloss.Style
	CalInMonth  lipgloss.Style
	CalOutMonth lipgloss.Style
	CalFuture   lipgloss.Style
	CalComplete lipgloss.Style
	CalPartial  lipgloss.Style
	CalSelected lipgloss.Style

	// Heatmap cell colors, indexed 0 (none) .. 4 (fully complete).
	HeatmapLevels [5]lipgloss.Style

	// Modals (huh/v2 forms live inside this frame).
	ModalBorder lipgloss.Style
	ModalTitle  lipgloss.Style

	// Help overlay and footer key hints.
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	Footer   lipgloss.Style
}

// New builds the theme. Called once by internal/ui at startup.
func New() Theme {
	t := Theme{
		FgDim:       colBrightBlack,
		Accent:      colCyan,
		AccentAlt:   colBlue,
		Success:     colGreen,
		SuccessAlt:  colBrightGreen,
		Warning:     colYellow,
		Danger:      colRed,
		Border:      colBrightBlack,
		BorderFocus: colCyan,
	}

	border := lipgloss.NormalBorder()

	t.Column = lipgloss.NewStyle().
		Border(border).
		BorderForeground(t.Border).
		Padding(0, 1)
	t.ColumnFocused = lipgloss.NewStyle().
		Border(border).
		BorderForeground(t.BorderFocus).
		Padding(0, 1)
	t.ColumnTitle = lipgloss.NewStyle().Bold(true).Foreground(t.FgDim)
	t.ColumnTitleOn = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.MinSizeMessage = lipgloss.NewStyle().Bold(true).Foreground(t.Warning)

	t.BlockTitle = lipgloss.NewStyle().Bold(true).Foreground(t.FgDim)
	t.BlockTitleOn = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)

	t.Text = lipgloss.NewStyle()
	t.Dim = lipgloss.NewStyle().Faint(true)
	t.Bold = lipgloss.NewStyle().Bold(true)
	t.Muted = lipgloss.NewStyle().Foreground(t.FgDim)

	t.Item = lipgloss.NewStyle()
	t.ItemSelected = lipgloss.NewStyle().Bold(true).Reverse(true)
	t.Marker = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)

	t.CheckDone = lipgloss.NewStyle().Bold(true).Foreground(t.Success)
	t.CheckPending = lipgloss.NewStyle().Foreground(t.FgDim)
	t.HabitDone = lipgloss.NewStyle().Faint(true)

	t.ProgressBarFilled = lipgloss.NewStyle().Foreground(t.SuccessAlt)
	t.ProgressBarEmpty = lipgloss.NewStyle().Foreground(t.FgDim)
	t.ProgressLabel = lipgloss.NewStyle().Bold(true)

	t.ReminderOverdue = lipgloss.NewStyle().Bold(true).Foreground(t.Danger)
	t.ReminderDueSoon = lipgloss.NewStyle().Foreground(t.Warning)
	t.ReminderUpcoming = lipgloss.NewStyle()
	t.ReminderDone = lipgloss.NewStyle().Faint(true)
	t.ReminderDot = lipgloss.NewStyle().Foreground(t.Warning)

	t.CalToday = lipgloss.NewStyle().Bold(true).Underline(true)
	t.CalInMonth = lipgloss.NewStyle()
	t.CalOutMonth = lipgloss.NewStyle().Foreground(t.FgDim)
	t.CalFuture = lipgloss.NewStyle().Foreground(t.FgDim)
	t.CalComplete = lipgloss.NewStyle().Bold(true).Foreground(t.SuccessAlt)
	t.CalPartial = lipgloss.NewStyle().Foreground(t.Warning)
	t.CalSelected = lipgloss.NewStyle().Bold(true).Reverse(true)

	t.HeatmapLevels = [5]lipgloss.Style{
		lipgloss.NewStyle().Foreground(t.FgDim),
		lipgloss.NewStyle().Foreground(t.Success),
		lipgloss.NewStyle().Foreground(t.Success),
		lipgloss.NewStyle().Foreground(t.SuccessAlt),
		lipgloss.NewStyle().Bold(true).Foreground(t.SuccessAlt),
	}

	t.ModalBorder = lipgloss.NewStyle().
		Border(border).
		BorderForeground(t.BorderFocus).
		Padding(1, 2)
	t.ModalTitle = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)

	t.HelpKey = lipgloss.NewStyle().Bold(true).Foreground(t.Accent)
	t.HelpDesc = lipgloss.NewStyle().Foreground(t.FgDim)
	t.Footer = lipgloss.NewStyle().Foreground(t.FgDim)

	return t
}
