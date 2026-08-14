package ui

import (
	"fmt"
	"sort"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// reminderChrome is the number of non-list rows at a given month height,
// including the bottom-anchored three-line Year block.
func reminderChrome(calendarWeeks int) int { return 11 + calendarWeeks }

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

// renderReminders builds the time/events column: action, conventional month
// calendar, and a chronological list with optional inline details.
func (m *Root) renderReminders(width, height int) string {
	focusedActions := m.panelFocused(ColReminders) && m.focus.Block == RemindersActions
	action := m.renderActionLine("Add reminder", focusedActions && m.focus.Item == 0)

	sorted := m.sortedReminders()
	focusedList := m.panelFocused(ColReminders) && m.focus.Block == RemindersList

	listTitleStyle := m.theme.BlockTitle
	if focusedList {
		listTitleStyle = m.theme.BlockTitleOn
	}
	listHeader := listTitleStyle.Render(fmt.Sprintf("Upcoming  %d", countUpcoming(sorted)))

	listHeight := max(height-reminderChrome(len(m.snap.Calendar.Weeks)), 1)
	body := m.theme.Muted.Render("No reminders yet.")
	if len(sorted) > 0 {
		lines, focusStart, focusEnd := m.reminderLines(width, sorted, focusedList)
		body = strings.Join(windowAroundRange(lines, listHeight, focusStart, focusEnd), "\n")
	}
	body = padLinesTo(body, listHeight)

	return action + "\n\n" + m.renderReminderCalendar(width) + "\n\n" + listHeader + "\n" + body + "\n\n" + m.renderYearProgress(width)
}

func (m *Root) renderReminderCalendar(width int) string {
	title := m.theme.BlockTitle
	if m.panelFocused(ColReminders) && m.focus.Block == RemindersCalendar {
		title = m.theme.BlockTitleOn
	}

	cal := m.snap.Calendar
	lines := []string{
		title.Render(theme.IconCalendar + " Calendar"),
		m.theme.Dim.Render(centeredLine(calendarMonthLabel(cal), width)),
		m.theme.Dim.Render(calendarGridRow(width, 2, []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"})),
	}
	for _, week := range cal.Weeks {
		cells := make([]string, len(week))
		for i, day := range week {
			cells[i] = m.reminderDayCell(day)
		}
		lines = append(lines, calendarGridRow(width, 2, cells))
	}
	return strings.Join(lines, "\n")
}

func (m *Root) reminderDayCell(day app.CalendarDay) string {
	return m.reminderDayStyle(day).Render(fmt.Sprintf("%2d", day.Date.Day))
}

func (m *Root) reminderDayStyle(day app.CalendarDay) lipgloss.Style {
	style := m.theme.CalInMonth
	if !day.InMonth {
		style = m.theme.CalOutMonth
	} else if day.IsFuture {
		style = m.theme.CalFuture
	}
	if day.HasReminder {
		style = style.Foreground(m.theme.Warning).Underline(true)
	}
	if day.IsToday {
		style = style.Faint(false).Bold(true).Foreground(m.theme.Accent)
	}
	return style
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
// wrapPhysical's doc comment. It reports the focused reminder's half-open
// physical-line span so its complete expansion can remain visible when it
// fits the viewport.
func (m *Root) reminderLines(width int, sorted []domain.Reminder, focusedList bool) (lines []string, focusStart, focusEnd int) {
	focusStart, focusEnd = -1, -1
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
		expanded := r.ID == m.expandedReminder
		itemStart := len(lines)
		itemLines := m.renderReminderLines(width, r, selected, expanded)
		if selected {
			focusStart = itemStart
		}
		lines = append(lines, itemLines...)
		if expanded {
			lines = append(lines, m.renderReminderDetails(width, r)...)
		}
		if selected {
			focusEnd = len(lines)
		}
	}
	return lines, focusStart, focusEnd
}

// renderReminderLines renders exactly one compact row. Long content is clipped
// here; expanding the reminder exposes the full title and notes below it.
func (m *Root) renderReminderLines(width int, r domain.Reminder, selected, expanded bool) []string {
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
		rows := []string{lipgloss.NewStyle().MaxWidth(width).Render(line)}
		if expanded {
			rows = wrapPhysical(line, width)
		}
		for i, row := range rows {
			rows[i] = m.theme.ItemSelected.Render(padTo(row, width))
		}
		return rows
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
	if expanded {
		return wrapPhysical(composite, width)
	}
	return []string{lipgloss.NewStyle().MaxWidth(width).Render(composite)}
}

func (m *Root) renderReminderDetails(width int, r domain.Reminder) []string {
	notes := flattenWhitespace(r.Notes)
	if notes == "" {
		notes = "—"
	}

	var lines []string
	lines = append(lines, m.renderReminderDetailField(width, "Date", r.Date.Time().Format("Mon, Jan 2, 2006"), m.theme.Text)...)
	lines = append(lines, m.renderReminderDetailField(width, "Status", m.reminderStatusLabel(r), m.reminderStatusStyle(r))...)
	lines = append(lines, m.renderReminderDetailField(width, "Notes", notes, m.theme.Text)...)
	return lines
}

func (m *Root) renderReminderDetailField(width int, label, value string, valueStyle lipgloss.Style) []string {
	const indent = "  "
	prefix := fmt.Sprintf("%-8s", label)
	field := m.theme.Dim.Render(prefix) + valueStyle.Render(value)
	lines := wrapPhysical(field, max(width-lipgloss.Width(indent), 1))
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return lines
}

func (m *Root) reminderStatusLabel(r domain.Reminder) string {
	switch {
	case r.Completed:
		return "Completed"
	case r.Date.Before(m.today):
		return "Overdue"
	case r.Date.Equal(m.today):
		return "Due today"
	default:
		return "Upcoming"
	}
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
