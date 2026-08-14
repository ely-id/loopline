package ui

import (
	"testing"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// rootWithGroups builds a bare Root (no backend, no bubbletea) with a
// snapshot holding n habit groups of the given sizes, for exercising focus
// math in isolation.
func rootWithGroups(sizes ...int) *Root {
	r := &Root{theme: theme.New()}
	groups := make([]app.HabitGroup, len(sizes))
	for i, n := range sizes {
		items := make([]app.HabitItem, n)
		for j := range items {
			items[j] = app.HabitItem{Habit: domain.Habit{ID: int64(i*100 + j), Title: "h"}}
		}
		groups[i] = app.HabitGroup{Title: "g", Items: items}
	}
	r.snap.Today.Groups = groups
	return r
}

func TestBlockCountOverviewIsFixed(t *testing.T) {
	r := rootWithGroups()
	if got := r.blockCount(ColOverview); got != numOverviewBlocks {
		t.Fatalf("blockCount(overview) = %d, want %d", got, numOverviewBlocks)
	}
	if got := r.blockCount(ColReminders); got != numReminderBlocks {
		t.Fatalf("blockCount(reminders) = %d, want %d", got, numReminderBlocks)
	}
}

func TestBlockCountTodayTracksGroups(t *testing.T) {
	r := rootWithGroups(3, 0, 2)
	if got := r.blockCount(ColToday); got != 4 {
		t.Fatalf("blockCount(today) = %d, want 4 (Add + 3 groups)", got)
	}
	if got := r.itemCount(ColToday, TodayActions); got != 2 {
		t.Fatalf("itemCount(today,Add) = %d, want 2", got)
	}
	if got := r.itemCount(ColToday, 1); got != 3 {
		t.Fatalf("itemCount(today,1) = %d, want 3", got)
	}
	if got := r.itemCount(ColToday, 2); got != 0 {
		t.Fatalf("itemCount(today,2) = %d, want 0", got)
	}
}

func TestClampFocusHandlesTodayWithNoHabitGroups(t *testing.T) {
	r := rootWithGroups() // Add block remains
	r.focus = Focus{Column: ColToday, Block: 5, Item: 5}
	r.clampFocus()
	if r.focus.Block != TodayActions || r.focus.Item != ActionAddRoutine {
		t.Fatalf("clampFocus with only Add block = %+v, want Add Routine", r.focus)
	}
}

func TestClampFocusPullsBackIntoRange(t *testing.T) {
	r := rootWithGroups(2, 4)
	r.focus = Focus{Column: ColToday, Block: 9, Item: 9}
	r.clampFocus()
	if r.focus.Block != 2 {
		t.Fatalf("Block = %d, want 2 (last valid group block)", r.focus.Block)
	}
	if r.focus.Item != 3 {
		t.Fatalf("Item = %d, want 3 (last valid item)", r.focus.Item)
	}
}

func TestMoveColumnWrapsAndResets(t *testing.T) {
	r := rootWithGroups(2)
	r.focus = Focus{Column: ColOverview, Block: 2, Item: 1}

	r.moveColumn(-1) // wrap left from overview -> reminders
	if r.focus.Column != ColReminders {
		t.Fatalf("Column = %d, want ColReminders", r.focus.Column)
	}
	if r.focus.Block != 0 || r.focus.Item != 0 {
		t.Fatalf("moveColumn did not reset block/item: %+v", r.focus)
	}

	r.moveColumn(1)
	if r.focus.Column != ColOverview {
		t.Fatalf("Column = %d, want ColOverview after wrap", r.focus.Column)
	}
}

func TestMoveBlockWrapsWithinColumn(t *testing.T) {
	r := rootWithGroups()
	r.focus.Column = ColOverview

	r.moveBlock(-1) // from block 0, wrap to last block
	if r.focus.Block != numOverviewBlocks-1 {
		t.Fatalf("Block = %d, want %d", r.focus.Block, numOverviewBlocks-1)
	}

	r.moveBlock(1)
	if r.focus.Block != 0 {
		t.Fatalf("Block = %d, want 0 after wrapping forward", r.focus.Block)
	}
}

func TestMoveItemClampsWithoutWrap(t *testing.T) {
	r := rootWithGroups(3)
	r.focus = Focus{Column: ColToday, Block: 1, Item: 0}

	r.moveItem(-1)
	if r.focus.Item != 0 {
		t.Fatalf("Item = %d, want 0 (clamped, no wrap)", r.focus.Item)
	}

	r.moveItem(1)
	r.moveItem(1)
	r.moveItem(1) // one past the end
	if r.focus.Item != 2 {
		t.Fatalf("Item = %d, want 2 (clamped at last index)", r.focus.Item)
	}
}

func TestWrapHelper(t *testing.T) {
	cases := []struct{ i, n, want int }{
		{0, 3, 0}, {2, 3, 2}, {3, 3, 0}, {-1, 3, 2}, {-4, 3, 2}, {5, 3, 2},
	}
	for _, c := range cases {
		if got := wrap(c.i, c.n); got != c.want {
			t.Errorf("wrap(%d,%d) = %d, want %d", c.i, c.n, got, c.want)
		}
	}
}
