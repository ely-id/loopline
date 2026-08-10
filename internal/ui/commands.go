package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/app"
)

// midnightTickInterval controls how often Root polls Backend.Today() for a
// date change. A package variable rather than a Root field or constructor
// argument, so tests can shrink it without changing New's signature; see
// model_test.go's init(), which sets it to a near-zero duration so the
// self-perpetuating tick command never actually blocks a test.
var midnightTickInterval = time.Minute

// doAction wraps a mutating Backend call as a tea.Cmd. Update never blocks on
// I/O: the call runs on the command goroutine and reports back as a message.
func doAction(fn func(context.Context) error) tea.Cmd {
	return func() tea.Msg {
		if err := fn(context.Background()); err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{}
	}
}

// fetchSnapshotCmd requests a fresh Snapshot for the current month/range
// selection, stamped with a new monotonic request ID so Update can discard
// it if a newer request finishes first.
func (m *Root) fetchSnapshotCmd() tea.Cmd {
	backend := m.backend
	req := app.SnapshotRequest{Month: m.calMonth, Range: m.statsRange}
	m.snapReqID++
	id := m.snapReqID
	return func() tea.Msg {
		snap, err := backend.Snapshot(context.Background(), req)
		if err != nil {
			return snapshotErrMsg{err: err, reqID: id}
		}
		return snapshotMsg{snap: snap, reqID: id}
	}
}

// tickMidnightCmd schedules the next once-a-minute check. It rearms itself
// every time it fires (see Update's tickMsg case), so Root keeps polling for
// as long as the program runs.
func (m *Root) tickMidnightCmd() tea.Cmd {
	return tea.Tick(midnightTickInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// mutateCmd marks a mutation in flight and wraps fn as a tea.Cmd. Update
// clears the pending flag when the resulting actionDoneMsg/actionErrMsg
// arrives (see model.go), so a second activate key pressed before that
// response lands is ignored rather than firing a second call.
func (m *Root) mutateCmd(fn func(context.Context) error) tea.Cmd {
	m.pending = true
	return doAction(fn)
}

func (m *Root) toggleHabitCmd(id int64) tea.Cmd {
	backend := m.backend
	return m.mutateCmd(func(ctx context.Context) error { return backend.ToggleHabit(ctx, id) })
}

func (m *Root) toggleReminderCmd(id int64) tea.Cmd {
	backend := m.backend
	return m.mutateCmd(func(ctx context.Context) error { return backend.ToggleReminder(ctx, id) })
}

func addHabitCmd(backend Backend, in app.NewHabit) tea.Cmd {
	return doAction(func(ctx context.Context) error { return backend.AddHabit(ctx, in) })
}

func addRoutineCmd(backend Backend, in app.NewRoutine) tea.Cmd {
	return doAction(func(ctx context.Context) error { return backend.AddRoutine(ctx, in) })
}

func addReminderCmd(backend Backend, in app.NewReminder) tea.Cmd {
	return doAction(func(ctx context.Context) error { return backend.AddReminder(ctx, in) })
}
