package ui

// Focus is the single source of truth for what's selected. It lives only on
// the root Model; every renderer is a pure function of Focus plus data.
type Focus struct {
	Column int // 0 overview, 1 today, 2 reminders
	Block  int // index within the column's block list
	Item   int // index within the focused block's selectable items
}

// Columns, in navigation order.
const (
	ColOverview = iota
	ColToday
	ColReminders
	numColumns
)

// Blocks within the overview column.
const (
	OverviewActions = iota
	OverviewLifetime
	OverviewWeek
	OverviewHeatmap
	numOverviewBlocks
)

// Blocks within the reminders column.
const (
	RemindersActions = iota
	RemindersList
	numReminderBlocks
)

// Items within the overview actions block.
const (
	ActionAddHabit = iota
	ActionAddRoutine
)

// blockCount returns how many blocks the given column currently has. Today's
// block count is dynamic: one per non-empty habit group.
func (m *Root) blockCount(col int) int {
	switch col {
	case ColOverview:
		return numOverviewBlocks
	case ColToday:
		return len(m.snap.Today.Groups)
	case ColReminders:
		return numReminderBlocks
	default:
		return 0
	}
}

// itemCount returns how many selectable items the given block holds.
func (m *Root) itemCount(col, block int) int {
	switch col {
	case ColOverview:
		if block == OverviewActions {
			return 2
		}
		return 1
	case ColToday:
		if block < 0 || block >= len(m.snap.Today.Groups) {
			return 0
		}
		return len(m.snap.Today.Groups[block].Items)
	case ColReminders:
		if block == RemindersActions {
			return 1
		}
		return len(m.snap.Reminders)
	default:
		return 0
	}
}

// clampFocus pulls Focus back into range after the underlying data changes
// (a new snapshot, a column/block switch). It never panics on an empty
// column or block.
func (m *Root) clampFocus() {
	bc := m.blockCount(m.focus.Column)
	if bc == 0 {
		m.focus.Block = 0
		m.focus.Item = 0
		return
	}
	if m.focus.Block >= bc {
		m.focus.Block = bc - 1
	}
	if m.focus.Block < 0 {
		m.focus.Block = 0
	}
	ic := m.itemCount(m.focus.Column, m.focus.Block)
	if m.focus.Item >= ic {
		m.focus.Item = ic - 1
	}
	if m.focus.Item < 0 {
		m.focus.Item = 0
	}
}

// moveColumn shifts focus to an adjacent column, wrapping around, and resets
// block/item since the two columns' block lists are unrelated.
func (m *Root) moveColumn(delta int) {
	m.focus.Column = wrap(m.focus.Column+delta, numColumns)
	m.focus.Block = 0
	m.focus.Item = 0
	m.clampFocus()
}

// moveBlock shifts focus to an adjacent block within the current column,
// wrapping around.
func (m *Root) moveBlock(delta int) {
	bc := m.blockCount(m.focus.Column)
	if bc == 0 {
		return
	}
	m.focus.Block = wrap(m.focus.Block+delta, bc)
	m.focus.Item = 0
	m.clampFocus()
}

// moveItem shifts focus to an adjacent item within the current block,
// clamped at the ends (no wrap: consistent with a simple list cursor).
func (m *Root) moveItem(delta int) {
	ic := m.itemCount(m.focus.Column, m.focus.Block)
	if ic == 0 {
		return
	}
	next := m.focus.Item + delta
	if next < 0 {
		next = 0
	}
	if next >= ic {
		next = ic - 1
	}
	m.focus.Item = next
}

func wrap(i, n int) int {
	if n <= 0 {
		return 0
	}
	i %= n
	if i < 0 {
		i += n
	}
	return i
}
