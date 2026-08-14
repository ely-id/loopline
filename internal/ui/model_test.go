package ui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"loopline/internal/domain"
)

// The real Root polls Backend.Today() once a minute (see commands.go's
// tickMidnightCmd) so a session left open notices midnight roll over. That
// poll is a self-perpetuating tea.Cmd exactly like bubbles/v2's cursor
// blink: step() below must never chase its rearm command, and its base
// interval must not be real wall-clock minutes or every test that calls
// Init() (via newReadyRoot) would block for that long. init() shrinks it
// once for the whole package-ui test binary.
func init() {
	midnightTickInterval = time.Nanosecond
}

// step drives the same message/command loop tea.Program would: run cmd,
// feed the resulting message back into Update, and repeat. tea.BatchMsg is
// unwrapped so every sub-command also gets its turn.
//
// The one thing it must never do is chase a bubbles/v2 cursor blink message.
// Model.Blink() returns a Cmd that genuinely blocks on a real timer
// (`<-ctx.Done()` after BlinkSpeed) and, on firing, calls Blink() again to
// rearm itself — that's an intentional forever-loop in the real async
// runtime (it just waits for the next tick in the background), but replayed
// synchronously here it would block for BlinkSpeed on every hop, forever.
// Every other message chain in this app (ours or huh's own field-advance
// messages) is finite, so those are always safe to chase to completion.
func step(t *testing.T, m *Root, cmd tea.Cmd) *Root {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = step(t, m, c)
		}
		return m
	}
	next, nextCmd := m.Update(msg)
	m = next.(*Root)
	if isCursorBlinkMsg(msg) {
		return m
	}
	if _, ok := msg.(tickMsg); ok {
		return m // tickMsg rearms itself every time it fires; never chase that forever
	}
	return step(t, m, nextCmd)
}

// isCursorBlinkMsg reports whether msg is one of
// charm.land/bubbles/v2/cursor's blink-cycle messages. Two of the three
// (initialBlinkMsg, blinkCanceled) are unexported, so this matches by
// package path and type name via reflection rather than a type switch.
func isCursorBlinkMsg(msg tea.Msg) bool {
	t := reflect.TypeOf(msg)
	if t == nil {
		return false
	}
	if t.PkgPath() != "charm.land/bubbles/v2/cursor" {
		return false
	}
	switch t.Name() {
	case "BlinkMsg", "initialBlinkMsg", "blinkCanceled":
		return true
	}
	return false
}

func send(t *testing.T, m *Root, msg tea.Msg) *Root {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(*Root)
	return step(t, m, cmd)
}

// keyMsg, pressKey and newReadyRoot are the shared test driver for package
// ui: spec_navigation_test.go (qa) builds on these instead of redefining
// them.
func keyMsg(s string) tea.KeyPressMsg {
	switch s {
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	default:
		r := []rune(s)
		return tea.KeyPressMsg{Code: r[0], Text: s}
	}
}

func pressKey(t *testing.T, m *Root, s string) *Root {
	t.Helper()
	return send(t, m, keyMsg(s))
}

func newReadyRoot(t *testing.T, backend *fakeBackend) *Root {
	t.Helper()
	m := New(backend)
	m = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = step(t, m, m.Init())
	if !m.ready {
		t.Fatal("root never became ready after Init")
	}
	return m
}

func TestInitLoadsSnapshot(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Drink water"})
	m := newReadyRoot(t, backend)

	if len(m.snap.Today.Groups) != 1 || len(m.snap.Today.Groups[0].Items) != 1 {
		t.Fatalf("Today.Groups = %+v, want one group with one habit", m.snap.Today.Groups)
	}
	if m.snap.Today.Groups[0].Items[0].Habit.Title != "Drink water" {
		t.Fatalf("habit title = %q", m.snap.Today.Groups[0].Items[0].Habit.Title)
	}
}

func TestWindowSizeBelowMinimumRendersMinSizeMessage(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	m := New(newFakeBackend(today))
	m = send(t, m, tea.WindowSizeMsg{Width: 60, Height: 15})

	view := m.View()
	if got := view.Content; !strings.Contains(got, "loopline needs a little more room") {
		t.Fatalf("View at 60x15 = %q, want min-size message", got)
	}
}

func TestWindowSizeAtMinimumRendersColumns(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	m := newReadyRoot(t, backend)
	m.width, m.height = MinWidth, MinHeight

	view := m.View()
	if strings.Contains(view.Content, "loopline needs a little more room") {
		t.Fatal("View at exactly MinWidth x MinHeight should render columns, not the min-size message")
	}
	if !strings.Contains(view.Content, "Overview") || !strings.Contains(view.Content, "Reminders") {
		t.Fatalf("View missing expected column titles: %q", view.Content)
	}
}

func TestArrowKeysNavigateColumnsAndWrap(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	m := newReadyRoot(t, newFakeBackend(today))

	if m.focus.Column != ColOverview {
		t.Fatalf("initial column = %d, want ColOverview", m.focus.Column)
	}
	m = pressKey(t, m, "left") // wraps to reminders
	if m.focus.Column != ColReminders {
		t.Fatalf("column after left = %d, want ColReminders", m.focus.Column)
	}
	m = pressKey(t, m, "right")
	if m.focus.Column != ColOverview {
		t.Fatalf("column after right = %d, want ColOverview", m.focus.Column)
	}
	// h/l aliases
	m = pressKey(t, m, "l")
	if m.focus.Column != ColToday {
		t.Fatalf("column after 'l' = %d, want ColToday", m.focus.Column)
	}
	m = pressKey(t, m, "h")
	if m.focus.Column != ColOverview {
		t.Fatalf("column after 'h' = %d, want ColOverview", m.focus.Column)
	}
}

func TestEnterTogglesHabit(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Stretch"})
	m := newReadyRoot(t, backend)

	m = pressKey(t, m, "right") // overview -> today
	m = pressKey(t, m, "tab")   // Add actions -> first habit group
	if m.focus.Column != ColToday {
		t.Fatalf("column = %d, want ColToday", m.focus.Column)
	}
	if m.snap.Today.Groups[0].Items[0].Done {
		t.Fatal("habit should start incomplete")
	}

	m = pressKey(t, m, "enter")
	if !m.snap.Today.Groups[0].Items[0].Done {
		t.Fatal("habit should be complete after enter toggles it and the snapshot refreshes")
	}

	m = pressKey(t, m, "enter")
	if m.snap.Today.Groups[0].Items[0].Done {
		t.Fatal("second enter should toggle the habit back to incomplete")
	}
}

func TestAddHabitFormEndToEnd(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	m := newReadyRoot(t, backend)

	// Add Habit is the first item in Today's Add block.
	m = pressKey(t, m, "right")
	m = pressKey(t, m, "enter")
	if m.modal == nil {
		t.Fatal("enter on 'Add habit' should open a modal")
	}

	for _, r := range "Read" {
		m = send(t, m, keyMsg(string(r)))
	}
	// Advance through Title -> Routine -> Schedule -> Weekdays -> Notes -> submit.
	for i := 0; i < 5; i++ {
		m = pressKey(t, m, "enter")
	}

	if m.modal != nil {
		t.Fatal("modal should close once the form completes")
	}
	if len(backend.habits) != 1 || backend.habits[0].Title != "Read" {
		t.Fatalf("backend.habits = %+v, want one habit titled Read", backend.habits)
	}
	if len(m.snap.Today.Groups) != 1 || m.snap.Today.Groups[0].Items[0].Habit.Title != "Read" {
		t.Fatalf("snapshot wasn't refreshed with the new habit: %+v", m.snap.Today.Groups)
	}
}

func TestEscClosesModalWithoutSubmitting(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	m := newReadyRoot(t, backend)

	m = pressKey(t, m, "right")
	m = pressKey(t, m, "enter") // open Add habit
	if m.modal == nil {
		t.Fatal("modal should be open")
	}
	m = pressKey(t, m, "esc")
	if m.modal != nil {
		t.Fatal("esc should close the modal")
	}
	if len(backend.habits) != 0 {
		t.Fatalf("esc should discard the form, but backend.habits = %+v", backend.habits)
	}
}

func TestHelpOverlayTogglesAndBlocksOtherKeys(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	m := newReadyRoot(t, newFakeBackend(today))

	m = pressKey(t, m, "?")
	if !m.help {
		t.Fatal("? should open help")
	}
	m = pressKey(t, m, "right") // should be swallowed by the help overlay
	if m.focus.Column != ColOverview {
		t.Fatalf("column changed while help was open: %d", m.focus.Column)
	}
	m = pressKey(t, m, "esc")
	if m.help {
		t.Fatal("esc should close help")
	}
}

func TestQuitOnlyWhenNoModalOpen(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	m := newReadyRoot(t, newFakeBackend(today))

	m = pressKey(t, m, "right")
	m = pressKey(t, m, "enter") // open Add habit modal
	if m.modal == nil {
		t.Fatal("modal should be open")
	}
	_, cmd := m.Update(keyMsg("q"))
	if cmd != nil && isQuit(cmd) {
		t.Fatal("q should not quit while a modal is open (it's text input)")
	}

	m2 := newReadyRoot(t, newFakeBackend(today))
	_, cmd2 := m2.Update(keyMsg("q"))
	if cmd2 == nil || !isQuit(cmd2) {
		t.Fatal("q should quit when no modal is open")
	}
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestReminderToggleAndAddReminderForm(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today)
	m := newReadyRoot(t, backend)

	m = pressKey(t, m, "left")  // to reminders column
	m = pressKey(t, m, "enter") // open Add reminder
	if m.modal == nil {
		t.Fatal("modal should open for Add reminder")
	}
	for _, r := range "Exam" {
		m = send(t, m, keyMsg(string(r)))
	}
	for i := 0; i < 3; i++ { // Title -> Date -> Notes -> submit
		m = pressKey(t, m, "enter")
	}
	if m.modal != nil {
		t.Fatal("modal should close after submit")
	}
	if len(backend.reminders) != 1 {
		t.Fatalf("backend.reminders = %+v, want one reminder", backend.reminders)
	}

	m = pressKey(t, m, "tab") // actions block -> calendar
	m = pressKey(t, m, "tab") // calendar -> list
	if m.focus.Block != RemindersList {
		t.Fatalf("block = %d, want RemindersList", m.focus.Block)
	}
	if m.snap.Reminders[0].Completed {
		t.Fatal("reminder should start incomplete")
	}
	m = pressKey(t, m, "enter")
	if !m.snap.Reminders[0].Completed {
		t.Fatal("enter should toggle the reminder complete")
	}
}
