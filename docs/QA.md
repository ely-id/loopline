# QA Conformance Report

Owner: qa. This document tracks independent verification of SPEC.md against
the built application. It is filled in as part of Task #8, after the
suites in Task #7 pass and the vertical slice (`ui` + `data` + `app`) is
wired together by the lead.

Status legend: `PASS`, `FAIL`, `PARTIAL`, `BLOCKED` (cannot test yet),
`N/A`.

## 1. MVP definition of done (SPEC.md §"MVP definition of done")

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Application starts successfully | BLOCKED | needs `cmd/routine` (Task #6) |
| 2 | Three-column layout works | BLOCKED | needs `ui` root model (Task #4) |
| 3 | Keyboard navigation works exactly as specified | BLOCKED | needs `ui` root model (Task #4); covered by `internal/ui/spec_navigation_test.go` |
| 4 | Habits can be created | BLOCKED | needs `app.Service.AddHabit` (Task #6) + UI form (Task #5) |
| 5 | Routines can be created | BLOCKED | needs `app.Service.AddRoutine` (Task #6) + UI form (Task #5) |
| 6 | Habits can optionally belong to routines | BLOCKED | needs `NewHabit.RoutineID` path end-to-end |
| 7 | Habits can be completed | BLOCKED | needs `app.Service.ToggleHabit` (Task #6) |
| 8 | State survives application restart | BLOCKED | needs `internal/store` (Task #2); covered by `internal/store/spec_persistence_test.go` (Close+reopen) |
| 9 | Daily progress works | BLOCKED | needs `domain.SummarizeDay` + `app.Service.Snapshot` |
| 10 | Calendar displays completion state | BLOCKED | needs `app.Snapshot.Calendar` populated end-to-end |
| 11 | Heatmap displays historical completion | BLOCKED | needs `app.Snapshot.Heatmap` populated end-to-end |
| 12 | Reminders can be created | BLOCKED | needs `app.Service.AddReminder` (Task #6) + UI form (Task #5) |
| 13 | Reminder dates appear in calendar | BLOCKED | needs `CalendarDay.HasReminder` wired |
| 14 | Week/month/90d/year/all analytics filters work | BLOCKED | needs `app.StatsRanges` cycling in UI + `Heatmap` builder |
| 15 | `go test ./...` passes | BLOCKED | run at Task #8 |
| 16 | `go vet ./...` passes | BLOCKED | run at Task #8 |
| 17 | `go build ./cmd/routine` succeeds | BLOCKED | run at Task #8 |

## 2. Navigation model (SPEC.md §"Navigation")

| Requirement | Status | Evidence |
|---|---|---|
| Left/Right moves between columns | BLOCKED | `spec_navigation_test.go` |
| Tab/Shift+Tab moves between blocks in current column | BLOCKED | `spec_navigation_test.go` |
| Up/Down moves between selectable items in current block | BLOCKED | `spec_navigation_test.go` |
| Enter activates selected action / toggles habit / opens item / confirms forms | BLOCKED | `spec_navigation_test.go` |
| Esc closes modal or goes back | BLOCKED | `spec_navigation_test.go` |
| `?` opens help | BLOCKED | `spec_navigation_test.go` |
| `q` quits only when no modal is active | BLOCKED | `spec_navigation_test.go` |
| Current column/block/item is visually obvious | BLOCKED | manual/visual check, not test-automatable |

## 3. Persistence (SPEC.md §"Persistence")

| Requirement | Status | Evidence |
|---|---|---|
| Uses SQLite | BLOCKED | `internal/store` |
| Follows XDG paths (`$XDG_DATA_HOME/routine-tui/routine.db`, fallback `~/.local/share/routine-tui/routine.db`) | BLOCKED | `store.DefaultPath` test |
| Preserves habits | BLOCKED | `spec_persistence_test.go` |
| Preserves routines | BLOCKED | `spec_persistence_test.go` |
| Preserves habit schedule | BLOCKED | `spec_persistence_test.go` |
| Preserves completions | BLOCKED | `spec_persistence_test.go` |
| Preserves reminders | BLOCKED | `spec_persistence_test.go` |
| Preserves daily completion snapshots | BLOCKED | `spec_persistence_test.go` |
| Deleting a routine does not silently destroy its habits | BLOCKED | `spec_persistence_test.go` (`DeleteRoutine` keeps habits, nulls `routine_id`) |
| Migrations are idempotent (safe to open the same DB twice) | BLOCKED | `spec_persistence_test.go` |
| Foreign keys enforced | BLOCKED | `spec_persistence_test.go` |

## 4. Responsive behaviour (SPEC.md §"Responsive behaviour")

| Requirement | Status | Evidence |
|---|---|---|
| Column widths are calculated dynamically from terminal size | BLOCKED | `spec_navigation_test.go` (WindowSizeMsg handling) |
| Below minimum useful size, shows a clear minimum-size message instead of corrupting the UI | BLOCKED | `spec_navigation_test.go` (below `ui.MinWidth`/`ui.MinHeight`) |

## 5. Styling (SPEC.md §"Styling")

| Requirement | Status | Evidence |
|---|---|---|
| No bundled/selected font | N/A (source inspection) | grep for font embedding — should find none |
| ANSI 16-color palette preferred | BLOCKED | manual inspection of `internal/theme` for `lipgloss.Color("0")`..`"15"` |
| No hardcoded RGB colors unless justified | BLOCKED | manual inspection of `internal/theme` |
| Terminal background remains terminal-controlled (no background paint) | BLOCKED | manual inspection of `internal/theme` |
| Square/simple borders | BLOCKED | manual inspection |
| Dense but readable TUI | BLOCKED | manual/visual check |
| No decorative gradients | BLOCKED | manual inspection |
| Unicode icons used sparingly | BLOCKED | manual inspection |

## 6. Core domain (SPEC.md §"Core domain")

| Requirement | Status | Evidence |
|---|---|---|
| Habit, Routine, Completion, Reminder, DailySummary entities exist | PASS | `internal/domain/model.go` |
| Habits can belong to zero or one routine | PASS | `domain.Habit.RoutineID *int64` |
| Deleting a routine must not silently destroy its habits | BLOCKED | see persistence section |

## 7. Build/test commands

| Command | Status | Notes |
|---|---|---|
| `go fmt ./...` | BLOCKED | run at Task #8 |
| `go test ./...` | BLOCKED | run at Task #8 |
| `go vet ./...` | BLOCKED | run at Task #8 |
| `go build ./cmd/routine` | BLOCKED | run at Task #8 |

## 8. Defects found

Filled in as discovered. Each entry: file, description, reproduction,
who it was reported to, resolution status.

_None yet — vertical slice not wired._
