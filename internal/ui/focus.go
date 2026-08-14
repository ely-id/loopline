package ui

// NavigationMode separates global navigation (horizontal header or vertical
// identity rail) from the selected panel's local navigation surface.
type NavigationMode int

const (
	NavigationHeader NavigationMode = iota
	NavigationPanel
)

// Focus is the single source of truth for the selected panel and its local
// target. Block remains a rendering/layout coordinate, while Up/Down walks a
// flattened list of every selectable Block/Item pair in the panel.
type Focus struct {
	Column int // 0 overview, 1 today, 2 reminders
	Block  int // index within the column's block list
	Item   int // index within the focused block's selectable items
}

// Stable logical column IDs. See globalColumnOrder for presentation order.
const (
	ColOverview = iota
	ColToday
	ColReminders
	numColumns
)

// globalColumnOrder is the left-to-right display and Tab order. Logical
// column IDs remain stable so panel-specific state and backend data do not
// depend on presentation order.
var globalColumnOrder = [...]int{ColToday, ColReminders, ColOverview}

// Blocks within the overview column.
const (
	OverviewLifetime = iota
	OverviewWeekly
	OverviewCompletion
	numOverviewBlocks
)

// Blocks within the today column. Habit groups follow TodayActions in their
// snapshot order, so group index = block - 1.
const TodayActions = 0

// Blocks within the reminders column.
const (
	RemindersActions = iota
	RemindersCalendar
	RemindersList
	numReminderBlocks
)

// Items within Today's Add block.
const (
	ActionAddHabit = iota
	ActionAddRoutine
)

// blockCount returns how many rendering blocks the given column currently
// has. Local navigation does not stop at these boundaries.
func (m *Root) blockCount(col int) int {
	switch col {
	case ColOverview:
		return numOverviewBlocks
	case ColToday:
		return 1 + len(m.snap.Today.Groups)
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
		switch block {
		case OverviewLifetime, OverviewCompletion:
			return 1
		case OverviewWeekly:
			return len(m.snap.Weekly.Rows)
		}
		return 0
	case ColToday:
		if block == TodayActions {
			return 2
		}
		group := block - 1
		if group < 0 || group >= len(m.snap.Today.Groups) {
			return 0
		}
		g := m.snap.Today.Groups[group]
		if g.RoutineID == nil {
			return len(g.Items)
		}
		if m.routineCollapsed(g) {
			return 1
		}
		return 1 + len(g.Items) // routine header plus its visible children
	case ColReminders:
		if block == RemindersActions {
			return 1
		}
		if block == RemindersCalendar {
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
	m.focus.Column = clampInt(m.focus.Column, 0, numColumns-1)
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
	if ic == 0 {
		for block := m.focus.Block + 1; block < bc; block++ {
			if m.itemCount(m.focus.Column, block) > 0 {
				m.focus.Block, m.focus.Item = block, 0
				return
			}
		}
		for block := m.focus.Block - 1; block >= 0; block-- {
			if count := m.itemCount(m.focus.Column, block); count > 0 {
				m.focus.Block, m.focus.Item = block, count-1
				return
			}
		}
		m.focus.Block, m.focus.Item = 0, 0
		return
	}
	if m.focus.Item >= ic {
		m.focus.Item = ic - 1
	}
	if m.focus.Item < 0 {
		m.focus.Item = 0
	}
}

// moveGlobal shifts the selected global entry, wrapping around, and resets
// its local target to the first selectable item in that panel.
func (m *Root) moveGlobal(delta int) {
	position := 0
	for i, col := range globalColumnOrder {
		if col == m.focus.Column {
			position = i
			break
		}
	}
	m.focus.Column = globalColumnOrder[wrap(position+delta, len(globalColumnOrder))]
	m.focus.Block = 0
	m.focus.Item = 0
	m.clampFocus()
}

// localPositions flattens a panel's logical blocks into one vertical
// navigation surface. Empty blocks contribute no target and therefore never
// trap the cursor.
func (m *Root) localPositions(col int) []Focus {
	var positions []Focus
	for block := 0; block < m.blockCount(col); block++ {
		for item := 0; item < m.itemCount(col, block); item++ {
			positions = append(positions, Focus{Column: col, Block: block, Item: item})
		}
	}
	return positions
}

// moveLocal moves continuously through every selectable target in the
// current panel, clamping at the first and last target.
func (m *Root) moveLocal(delta int) {
	positions := m.localPositions(m.focus.Column)
	if len(positions) == 0 {
		return
	}
	current := 0
	for i, position := range positions {
		if position == m.focus {
			current = i
			break
		}
	}
	current = clampInt(current+delta, 0, len(positions)-1)
	m.focus = positions[current]
}

func (m *Root) panelFocused(col int) bool {
	return m.navigation == NavigationPanel && m.focus.Column == col
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
