package ui

import (
	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
	"routine-tui/internal/theme"
)

// Root is the Bubble Tea v2 root model for routine-tui. It owns every piece
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

	calMonth   domain.Date // zero means "the month containing today"
	statsRange app.StatsRange

	snapReqID int64 // monotonic; bumped by every fetchSnapshotCmd
	pending   bool  // a mutating Backend call is in flight

	status string // transient status/error line shown in the footer
}

// New builds the root model. It performs no I/O itself; Init kicks off the
// first Snapshot fetch as a tea.Cmd.
func New(backend Backend) *Root {
	return &Root{
		backend:    backend,
		theme:      theme.New(),
		today:      backend.Today(), // bootstrap default until the first Snapshot lands
		statsRange: app.RangeMonth,
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
	m.clampFocus()
}
