package ui

import (
	"testing"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

func rootWithGroups(sizes ...int) *Root {
	r := &Root{theme: theme.New(), navigation: NavigationPanel}
	groups := make([]app.HabitGroup, len(sizes))
	for i, n := range sizes {
		items := make([]app.HabitItem, n)
		for j := range items {
			items[j] = app.HabitItem{Habit: domain.Habit{ID: int64(i*100 + j + 1), Title: "h"}}
		}
		groups[i] = app.HabitGroup{Title: "g", Items: items}
	}
	r.snap.Today.Groups = groups
	return r
}

func TestBlockCountsIncludeWeeklyAndEveryHabitGroup(t *testing.T) {
	r := rootWithGroups(3, 0, 2)
	if got := r.blockCount(ColOverview); got != numOverviewBlocks {
		t.Fatalf("blockCount(overview) = %d, want %d", got, numOverviewBlocks)
	}
	if got := r.blockCount(ColToday); got != 4 {
		t.Fatalf("blockCount(habits) = %d, want Add + 3 groups", got)
	}
	if got := r.blockCount(ColReminders); got != numReminderBlocks {
		t.Fatalf("blockCount(reminders) = %d, want %d", got, numReminderBlocks)
	}
}

func TestRoutineHeaderIsAlwaysASelectableItem(t *testing.T) {
	r := rootWithGroups()
	id := int64(7)
	r.snap.Today.Groups = []app.HabitGroup{{RoutineID: &id, Title: "Morning"}}
	if got := r.itemCount(ColToday, 1); got != 1 {
		t.Fatalf("expanded empty routine item count = %d, want its header", got)
	}
	r.snap.Today.Groups[0].Items = []app.HabitItem{{Habit: domain.Habit{ID: 1, Title: "Water"}}}
	if got := r.itemCount(ColToday, 1); got != 2 {
		t.Fatalf("expanded routine item count = %d, want header + child", got)
	}
	r.collapsedRoutines = map[int64]bool{id: true}
	if got := r.itemCount(ColToday, 1); got != 1 {
		t.Fatalf("collapsed routine item count = %d, want header only", got)
	}
}

func TestClampFocusSkipsEmptyWeeklyBlock(t *testing.T) {
	r := rootWithGroups()
	r.focus = Focus{Column: ColOverview, Block: OverviewWeekly, Item: 8}
	r.clampFocus()
	if r.focus != (Focus{Column: ColOverview, Block: OverviewCompletion, Item: 0}) {
		t.Fatalf("focus = %+v, want next selectable Completion block", r.focus)
	}
}

func TestClampFocusPullsBackAfterListShrinks(t *testing.T) {
	r := rootWithGroups(2, 4)
	r.focus = Focus{Column: ColToday, Block: 9, Item: 9}
	r.clampFocus()
	if r.focus != (Focus{Column: ColToday, Block: 2, Item: 3}) {
		t.Fatalf("focus = %+v, want last valid local item", r.focus)
	}
}

func TestMoveGlobalWrapsAndResetsToFirstLocalTarget(t *testing.T) {
	r := rootWithGroups(2)
	r.navigation = NavigationHeader
	r.focus = Focus{Column: ColOverview, Block: OverviewCompletion}
	r.moveGlobal(-1)
	if r.focus != (Focus{Column: ColReminders, Block: RemindersActions, Item: 0}) {
		t.Fatalf("after reverse wrap focus = %+v", r.focus)
	}
	r.moveGlobal(1)
	if r.focus != (Focus{Column: ColOverview, Block: OverviewLifetime, Item: 0}) {
		t.Fatalf("after forward wrap focus = %+v", r.focus)
	}
}

func TestMoveLocalCrossesLogicalBlockBoundariesWithoutWrapping(t *testing.T) {
	r := rootWithGroups(2)
	r.focus = Focus{Column: ColToday, Block: TodayActions, Item: ActionAddRoutine}
	r.moveLocal(1)
	if r.focus != (Focus{Column: ColToday, Block: 1, Item: 0}) {
		t.Fatalf("down across Add/group boundary = %+v", r.focus)
	}
	r.moveLocal(1)
	if r.focus != (Focus{Column: ColToday, Block: 1, Item: 1}) {
		t.Fatalf("down within group = %+v", r.focus)
	}
	r.moveLocal(1)
	if r.focus.Item != 1 {
		t.Fatalf("down past panel end wrapped unexpectedly: %+v", r.focus)
	}
	r.moveLocal(-1)
	r.moveLocal(-1)
	if r.focus != (Focus{Column: ColToday, Block: TodayActions, Item: ActionAddRoutine}) {
		t.Fatalf("up across group/Add boundary = %+v", r.focus)
	}
}

func TestLocalPositionsSkipEmptyBlocks(t *testing.T) {
	r := rootWithGroups()
	r.snap.Weekly.Rows = nil
	positions := r.localPositions(ColOverview)
	want := []Focus{
		{Column: ColOverview, Block: OverviewLifetime},
		{Column: ColOverview, Block: OverviewCompletion},
	}
	if len(positions) != len(want) {
		t.Fatalf("positions = %+v, want %+v", positions, want)
	}
	for i := range want {
		if positions[i] != want[i] {
			t.Fatalf("positions[%d] = %+v, want %+v", i, positions[i], want[i])
		}
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
