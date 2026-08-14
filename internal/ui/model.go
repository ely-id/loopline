package ui

import (
	tea "charm.land/bubbletea/v2"

	"loopline/internal/app"
	"loopline/internal/domain"
	"loopline/internal/theme"
)

// Root is loopline's Bubble Tea v2 root model. It owns every piece
// of interaction state (focus, modal, help, transient status) and renders
// the whole three-column layout from the last Snapshot it was given.
type Root struct {
	backend Backend
	theme   theme.Theme

	width, height int

	today domain.Date
	snap  app.Snapshot
	ready bool // at least one Snapshot has been loaded

	focus Focus
	modal *modalState
	help  bool

	calMonth domain.Date // zero means "the month containing today"

	// Expansion is deliberately ephemeral presentation state. Routines are
	// expanded by default; the map only records collapsed routine IDs. At
	// most one reminder is expanded to keep its narrow column predictable.
	collapsedRoutines map[int64]bool
	expandedReminder  int64

	snapReqID int64 // monotonic; bumped by every fetchSnapshotCmd
	pending   bool  // a mutating Backend call is in flight

	status string // transient status/error line shown in the footer
}

// New builds the root model. It performs no I/O itself; Init kicks off the
// first Snapshot fetch as a tea.Cmd.
func New(backend Backend) *Root {
	return &Root{
		backend:           backend,
		theme:             theme.New(),
		today:             backend.Today(), // bootstrap default until the first Snapshot lands
		collapsedRoutines: make(map[int64]bool),
	}
}

// Init implements tea.Model.
func (m *Root) Init() tea.Cmd {
	return tea.Batch(m.fetchSnapshotCmd(), m.tickMidnightCmd())
}

// Update implements tea.Model.
func (m *Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.modal != nil {
			return m.resizeModal()
		}
		return m, nil
	case snapshotMsg:
		if msg.reqID != m.snapReqID {
			return m, nil // superseded by a newer request
		}
		m.applySnapshot(msg.snap)
		return m, nil
	case snapshotErrMsg:
		if msg.reqID != m.snapReqID {
			return m, nil
		}
		m.status = msg.err.Error()
		return m, nil
	case actionErrMsg:
		m.pending = false
		m.status = msg.err.Error()
		return m, nil
	case actionDoneMsg:
		m.pending = false
		return m, m.fetchSnapshotCmd()
	case tickMsg:
		cmds := []tea.Cmd{m.tickMidnightCmd()}
		if m.backend.Today() != m.today {
			cmds = append(cmds, m.fetchSnapshotCmd())
		}
		return m, tea.Batch(cmds...)
	}

	if m.modal != nil {
		if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == "esc" {
			m.modal = nil
			return m, nil
		}
		return m.updateModal(msg)
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		return m.handleKey(key)
	}
	return m, nil
}

// applySnapshot replaces view data wholesale, per the architecture contract,
// then re-clamps focus so it can never point past the end of a list that
// just got shorter. m.today is derived from the snapshot's own notion of
// "today" so there is exactly one source of truth once the app is ready;
// see tickMsg in Update for what keeps that value fresh across midnight.
func (m *Root) applySnapshot(snap app.Snapshot) {
	m.snap = snap
	m.today = snap.Today.Date
	m.ready = true
	m.status = ""
	m.reconcileExpansion()
	m.clampFocus()
}

func (m *Root) reconcileExpansion() {
	if m.expandedReminder != 0 {
		found := false
		for _, reminder := range m.snap.Reminders {
			if reminder.ID == m.expandedReminder {
				found = true
				break
			}
		}
		if !found {
			m.expandedReminder = 0
		}
	}

	if len(m.collapsedRoutines) == 0 {
		return
	}
	active := make(map[int64]bool, len(m.snap.Today.Groups))
	for _, group := range m.snap.Today.Groups {
		if group.RoutineID != nil {
			active[*group.RoutineID] = true
		}
	}
	for id := range m.collapsedRoutines {
		if !active[id] {
			delete(m.collapsedRoutines, id)
		}
	}
}
