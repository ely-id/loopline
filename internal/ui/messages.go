package ui

import "loopline/internal/app"

// snapshotMsg carries a freshly loaded Snapshot. The model always replaces
// its data wholesale on receipt; it never mutates view data locally.
//
// reqID ties the result back to the fetchSnapshotCmd that produced it, so a
// slow response to an old request (e.g. a month or range the user has since
// navigated away from) can never overwrite a newer one.
type snapshotMsg struct {
	snap  app.Snapshot
	reqID int64
}

// snapshotErrMsg reports a failed Snapshot fetch. Same reqID discipline as
// snapshotMsg: a stale error must not clobber a status line set by a newer
// request.
type snapshotErrMsg struct {
	err   error
	reqID int64
}

// actionDoneMsg reports that a mutating Backend call succeeded; the model
// responds by requesting a fresh Snapshot.
type actionDoneMsg struct{}

// actionErrMsg reports that a mutating Backend call failed.
type actionErrMsg struct {
	err error
}

// tickMsg fires roughly once a minute so Root can notice the calendar date
// changing (e.g. the app left open across midnight) without any user input.
type tickMsg struct{}
