package ui

import (
	"testing"

	"loopline/internal/domain"
)

// countCalls returns how many times name appears in calls (fakeBackend
// records every Backend method it's asked to perform, in order).
func countCalls(calls []string, name string) int {
	n := 0
	for _, c := range calls {
		if c == name {
			n++
		}
	}
	return n
}

// TestSpecActivateRapidDoubleEnterTogglesHabitExactlyOnce exercises the
// in-flight mutation guard (model.go's m.pending, set by commands.go's
// mutateCmd and cleared when actionDoneMsg/actionErrMsg lands): two Enter
// presses that both land before the first toggle's response comes back must
// net to exactly one ToggleHabit call, not zero (both dropped) and not two
// (a duplicate toggle that would cancel itself out and silently do nothing,
// or worse, desync the UI from a real backend where toggling twice really
// does mean "on" then "off").
//
// This drives Update directly rather than through the shared pressKey/send
// helpers, which fully resolve a key press's command chain to completion
// before returning — exactly the interleaving this test needs to NOT happen
// for the first Enter, so the second Enter can observe the mutation still in
// flight.
func TestSpecActivateRapidDoubleEnterTogglesHabitExactlyOnce(t *testing.T) {
	today := domain.MustParseDate("2026-08-10")
	backend := newFakeBackend(today).withHabits(domain.Habit{Title: "Stretch"})
	root := newReadyRoot(t, backend)
	root = enterPanel(t, root, ColToday)
	root = pressKey(t, root, "down") // Add Habit -> Add Routine
	root = pressKey(t, root, "down") // first habit group

	if root.pending {
		t.Fatal("setup: pending should start false")
	}
	if root.snap.Today.Groups[0].Items[0].Done {
		t.Fatal("setup: habit should start incomplete")
	}

	next1, cmd1 := root.Update(keyMsg("enter"))
	root = next1.(*Root)
	if !root.pending {
		t.Fatal("first enter should mark a mutation as in flight")
	}
	if cmd1 == nil {
		t.Fatal("first enter should return the toggle command")
	}

	next2, cmd2 := root.Update(keyMsg("enter"))
	root = next2.(*Root)
	if cmd2 != nil {
		t.Fatal("second enter arriving while a mutation is in flight must be dropped (nil cmd), not fire a second toggle")
	}
	if !root.pending {
		t.Fatal("pending should still be true after the dropped second enter")
	}

	// Resolve the one and only in-flight mutation, then let its follow-up
	// snapshot refresh run to completion.
	root = send(t, root, cmd1())

	if got := countCalls(backend.calls, "ToggleHabit"); got != 1 {
		t.Fatalf("ToggleHabit called %d times across two rapid enters, want exactly 1", got)
	}
	if !root.snap.Today.Groups[0].Items[0].Done {
		t.Fatal("habit should be toggled to done after the single in-flight mutation resolves")
	}
	if root.pending {
		t.Fatal("pending should be cleared once the mutation resolves")
	}
}
