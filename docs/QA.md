# QA Conformance Report

Verification of the current SPEC.md against the evolved `loopline` interface.
The v0.1 tag remains the historical MVP baseline; this report describes the
working tree after the monthly-calendar redesign.

Status legend: `PASS`, `FAIL`, `PARTIAL`, `N/A`.

Verified against a release gate of `go fmt ./...`, `go test ./...`,
`go vet ./...`, `go build -o loopline ./cmd/routine` (all green) plus a manual pty run of
the built binary against a scratch database.

## 1. Current definition of done (SPEC.md §"Current definition of done")

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Application starts successfully | PASS | manual 96x33 pty run of `./loopline -db <tmp>`; renders and exits cleanly on `q` |
| 2 | Three-column layout works | PASS | `internal/ui/layout_test.go`, `spec_render_test.go`; confirmed visually |
| 3 | Keyboard navigation works exactly as specified | PASS | `internal/ui/spec_navigation_test.go`, `focus_test.go` |
| 4 | Habits can be created | PASS | manual E2E via the huh form; row present in `habits` |
| 5 | Routines can be created | PASS | `internal/app/service_test.go` (`AddRoutine`), `internal/store/routines_test.go` |
| 6 | Habits can optionally belong to routines | PASS | `NewHabit.RoutineID` end-to-end; `internal/app/service_test.go` |
| 7 | Habits can be completed | PASS | manual E2E toggle; `completions` row written, column showed `1/1 100%` |
| 8 | State survives application restart | PASS | `internal/store/spec_persistence_test.go` (close + reopen) |
| 9 | Daily progress works | PASS | `internal/domain/summary_test.go`; shared semantic color stage in `internal/ui/today.go` |
| 10 | Monthly calendar displays authoritative historical completion | PASS | `service_test.go:TestServiceCalendarDerivesHistoryFromHabitsAndCompletions`; semantic boundary/color coverage in `completion_calendar_test.go` |
| 11 | Conventional calendar displays today and reminder dates | PASS | `completion_calendar_test.go:TestReminderCalendarStylePreservesTodayAndReminderDates` |
| 12 | Reminders can be created | PASS | `internal/app/service_test.go`, `internal/store/reminders_test.go` |
| 13 | Month navigation works from either calendar | PASS | `internal/ui/spec_navigation_test.go`, including Jan/Mar boundaries and shared Reminders month |
| 14 | Three-column layout works comfortably at approximately 96 x 33 | PASS | `layout_test.go`, `column_fit_test.go`, `height_budget_test.go`; manual 96x33 pty render |
| 15 | Habits, routines and reminders can be edited/deleted safely | PASS | service preservation tests plus modal-driven `crud_test.go` |
| 16 | `go test ./...` passes | PASS | release gate |
| 17 | `go vet ./...` passes | PASS | release gate |
| 18 | `go build -o loopline ./cmd/routine` succeeds | PASS | release gate; output verified executable |

### Release-candidate polish acceptance

| Requirement | Status | Evidence |
|---|---|---|
| Permanent completion legend removed | PASS | `TestCompletionCalendarHasNoPermanentLegend` |
| Year Progress handles day-of-year, leap years and ISO weeks | PASS | `year_progress_test.go`; seeded 96x33 pty run |
| Routine `t` expand/collapse and focus correction | PASS | `expansion_test.go:TestRoutineExpandCollapseCorrectsFocusAndHidesChildren`; manual pty interaction |
| Reminder `t` details, completed rows, missing notes and long wrapping | PASS | `expansion_test.go`, `column_fit_test.go`; seeded 96x33 pty interaction |
| `e` edit and physical Delete confirmation | PASS | `crud_test.go`; in-place app/store semantics in `service_test.go` and persistence tests |
| Shared ANSI-16 completion semantics in calendar and Today | PASS | `completion_calendar_test.go`; seeded 0/low/medium/high/complete month and 100% Today pty run |
| `loopline` identity and executable | PASS | fixed heading width test; real `./loopline` launch; no release-facing legacy command |

## 2. Navigation model (SPEC.md §"Navigation")

| Requirement | Status | Evidence |
|---|---|---|
| Left/Right moves between columns | PASS | `spec_navigation_test.go` |
| Tab/Shift+Tab moves between blocks in current column | PASS | `spec_navigation_test.go` |
| Up/Down moves between selectable items in current block | PASS | `spec_navigation_test.go` |
| Enter activates action / toggles habit / confirms forms | PASS | `spec_activation_test.go` |
| `e` edits the selected habit, routine or reminder | PASS | `crud_test.go`; pre-population and cancellation coverage |
| `t` expands/collapses a focused routine or reminder | PASS | `expansion_test.go`; help and contextual footer coverage |
| Delete confirms before deleting the selected entity | PASS | `crud_test.go`; default-cancel and focus/expansion cleanup coverage |
| `<`/`>` and `[`/`]` move the focused calendar month | PASS | `spec_navigation_test.go` |
| Esc closes modal or goes back | PASS | `spec_navigation_test.go`; `Root.Update` intercepts `esc` ahead of the form |
| `?` opens help | PASS | `spec_navigation_test.go` |
| `q` quits only when no modal is active | PASS | `spec_navigation_test.go`; modal and help paths both swallow `q` |
| Current column/block/item is visually obvious | PASS | focused column border, `BlockTitleOn`, `ItemSelected`; confirmed visually |

## 3. Persistence (SPEC.md §"Persistence")

| Requirement | Status | Evidence |
|---|---|---|
| Uses SQLite | PASS | `modernc.org/sqlite`, `internal/store` |
| Follows XDG paths | PASS | `store.DefaultPath`, `internal/store/store_test.go` |
| Preserves habits / routines / habit schedule | PASS | `spec_persistence_test.go` |
| Preserves completions | PASS | `spec_persistence_test.go`; confirmed in a real DB after a manual run |
| Preserves reminders | PASS | `spec_persistence_test.go` |
| Preserves daily completion snapshots | PASS | `daily_summaries` written through in the toggle transaction; see debt note below |
| Deleting a routine does not silently destroy its habits | PASS | `DeleteRoutine` nulls `routine_id`; `ON DELETE SET NULL` |
| Migrations are idempotent | PASS | `spec_persistence_test.go` (reopen applies nothing) |
| Foreign keys enforced | PASS | `spec_persistence_test.go`; pragma set per-connection via DSN |

## 4. Responsive behaviour

| Requirement | Status | Evidence |
|---|---|---|
| Column widths calculated dynamically | PASS | `columnWidths`, including 31/34/29 minimum and 1:2:1 growth; `layout_test.go` |
| Below minimum size, shows a clear message | PASS | `MinWidth` 96 / `MinHeight` 33, `renderMinSize`; pinned by `layout_test.go` |
| Six-week calendar, expansions and long lists do not clip at 96x33 | PASS | `layout_test.go`, `column_fit_test.go`, `height_budget_test.go`; manual seeded run |

## 5. Styling

| Requirement | Status | Evidence |
|---|---|---|
| No bundled/selected font | PASS | no font assets in the tree |
| ANSI 16-color palette | PASS | `internal/theme/theme.go` uses only `lipgloss.Color("0")`..`("15")` |
| No hardcoded RGB colors | PASS | style constructors use ANSI indices only; RGB literals appear only in explanatory comments about overridden defaults |
| Terminal background remains terminal-controlled | PASS | no full-page/column background; only local focused/blurred form buttons use ANSI backgrounds |
| Square/simple borders, no gradients, selective fixed Nerd Font icons | PASS | fixed icon width test; source inspection; confirmed visually |
| Completion intensity is semantic in color and remains legible without it | PASS | none/red/yellow/white/green/bright-green scale plus `·░▒▓▆█`; `completion_calendar_test.go` |

## 6. Core domain

| Requirement | Status | Evidence |
|---|---|---|
| Habit, Routine, Completion, Reminder, DailySummary exist | PASS | `internal/domain/model.go` |
| Habits belong to zero or one routine | PASS | `domain.Habit.RoutineID *int64` |
| Deleting a routine does not destroy its habits | PASS | see persistence section |

## 7. Known technical debt (accepted, not release-blocking)

1. `daily_summaries` is a derived write-through cache, not a complete
   record. It is written on every mutation and at startup (`EnsureToday`),
   but a day crossed while the app sits open with no interaction gets no
   row — as does any day the app is never launched. Harmless: every
   analytics and calendar read path derives from `completions` + habits, so
   the table is never load-bearing for correctness.
2. `Store.SetCompletion` writes `completions` without refreshing
   `daily_summaries`. It is reachable only from tests; the production path
   uses the atomic `ToggleCompletion`, which rewrites the summary inside the
   same transaction.
3. UI backend commands use `context.Background()` rather than the Bubble Tea
   program context, so an in-flight SQLite call is not cancelled on quit.
   Bounded by the 5s `busy_timeout`.
