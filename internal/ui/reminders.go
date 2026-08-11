package ui

import (
	"fmt"
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// reminderChrome is the number of non-list lines renderReminders always
// writes: title, blank, action row, blank, list header, blank, notes line.
const reminderChrome = 7

// sortedReminders returns the reminders Store.Reminders handed back,
// reordered for display: reminders still due (in date order, so an overdue
// one naturally leads) before reminders already completed. Every reader of
// the reminders list — rendering, item counting, activation — goes through
// this so "the Nth item" always means the same reminder everywhere.
func (m *Root) sortedReminders() []domain.Reminder {
	sorted := append([]domain.Reminder(nil), m.snap.Reminders...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.Completed != b.Completed {
			return !a.Completed // incomplete (due) before completed (done)
		}
		return a.Date.Before(b.Date)
	})
	return sorted
}

// renderReminders builds the reminders column: the "add reminder" action,
// the chronological upcoming list (windowed to fit height, done reminders
// de-emphasised below a "Done" divider), then the selected reminder's notes
// pinned to the bottom.
func (m *Root) renderReminders(width, height int) string {
	header := m.columnTitle(ColReminders, "Reminders")

	focusedActions := m.focus.Column == ColReminders && m.focus.Block == RemindersActions
	action := m.renderActionLine("Add reminder", focusedActions && m.focus.Item == 0)

	sorted := m.sortedReminders()
	focusedList := m.focus.Column == ColReminders && m.focus.Block == RemindersList

	listTitleStyle := m.theme.BlockTitle
	if focusedList {
		listTitleStyle = m.theme.BlockTitleOn
	}
	listHeader := listTitleStyle.Render(fmt.Sprintf("Upcoming  %d", countUpcoming(sorted)))

	body := m.theme.Muted.Render("No reminders yet.")
	notesText := ""
	if len(sorted) > 0 {
		lines, focusLine := m.reminderLines(width, sorted, focusedList)
		listHeight := max(height-reminderChrome, 1)
		body = strings.Join(windowAround(lines, listHeight, focusLine), "\n")
		if focusedList && m.focus.Item >= 0 && m.focus.Item < len(sorted) {
			notesText = flattenWhitespace(sorted[m.focus.Item].Notes)
		}
	}

	// reminderChrome budgets exactly one line for the note, but the note
	// comes from a huh multiline text field, so it can legitimately contain
	// newlines. MaxWidth truncates each line without joining them, so a
	// three-line note would render three rows and push the column two rows
	// past its budget — clipping the bottom border off the whole column.
	// flattenWhitespace above collapses it to a single line first.
	notesLine := m.theme.Dim.Render(" ")
	if notesText != "" {
		notesLine = m.theme.Dim.MaxWidth(width).Render("note: " + notesText)
	}

	return header + "\n\n" + action + "\n\n" + listHeader + "\n" + body + "\n\n" + notesLine
}

// countUpcoming is how many reminders in sorted are not yet completed — the
// number the "Upcoming" header must report, as opposed to the full list
// length (which also includes done reminders kept around so they can be
// un-toggled).
func countUpcoming(sorted []domain.Reminder) int {
	n := 0
	for _, r := range sorted {
		if !r.Completed {
			n++
		}
	}
	return n
}

// reminderLines renders one or more *physical* lines per reminder in sorted
// (inserting a single "Done" divider the first time it crosses from
// incomplete to completed), pre-wrapped to width via wrapPhysical so the
// caller windows on real screen rows, not logical reminders — see
// wrapPhysical's doc comment. It reports the absolute index of the first
// physical line of the focused reminder, or -1 if the list doesn't
// currently hold focus.
func (m *Root) reminderLines(width int, sorted []domain.Reminder, focusedList bool) (lines []string, focusLine int) {
	focusLine = -1
	doneHeaderShown := false
	for i, r := range sorted {
		if r.Completed && !doneHeaderShown {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, wrapPhysical(m.theme.Dim.Render("Done"), width)...)
			doneHeaderShown = true
		}
		selected := focusedList && m.focus.Item == i
		itemLines := m.renderReminderLines(width, r, selected)
		if selected {
			focusLine = len(lines)
		}
		lines = append(lines, itemLines...)
	}
	return lines, focusLine
}

// renderReminderLines renders one reminder row and wraps it to width,
// returning every physical line it occupies. A selected reminder is padded
// and reversed per physical line (rather than once over the whole, possibly
// multi-line, row) so the highlight covers the full width of every wrapped
// line, not just the first.
func (m *Root) renderReminderLines(width int, r domain.Reminder, selected bool) []string {
	check := theme.GlyphPending
	if r.Completed {
		check = theme.GlyphDone
	}

	note := ""
	if strings.TrimSpace(r.Notes) != "" {
		note = " " + theme.GlyphBullet
	}

	body := fmt.Sprintf("%s  %s%s", shortDate(r.Date), r.Title, note)

	if selected {
		line := fmt.Sprintf("%s [%s] %s", markerGlyph(true), check, body)
		rows := wrapPhysical(line, width)
		out := make([]string, len(rows))
		for i, row := range rows {
			out[i] = m.theme.ItemSelected.Render(padTo(row, width))
		}
		return out
	}

	checkStyle := m.theme.CheckPending
	if r.Completed {
		checkStyle = m.theme.CheckDone
	}
	status := m.reminderStatusStyle(r)
	composite := fmt.Sprintf("%s [%s] %s",
		theme.GlyphUnselected,
		checkStyle.Render(check),
		status.Render(body),
	)
	return wrapPhysical(composite, width)
}

func (m *Root) reminderStatusStyle(r domain.Reminder) lipgloss.Style {
	switch {
	case r.Completed:
		return m.theme.ReminderDone
	case r.Date.Before(m.today):
		return m.theme.ReminderOverdue
	case r.Date.Equal(m.today):
		return m.theme.ReminderDueSoon
	default:
		return m.theme.ReminderUpcoming
	}
}
