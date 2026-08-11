# QA Conformance Report

Independent verification of SPEC.md against the built application, recorded
at final integration. Supersedes the pre-implementation version of this
document, which listed every row as `BLOCKED` because the vertical slice was
not yet wired.

Status legend: `PASS`, `FAIL`, `PARTIAL`, `N/A`.

Verified against a release gate of `go fmt ./...`, `go test ./...`,
`go vet ./...`, `go build ./cmd/routine` (all green) plus a manual pty run of
the built binary against a scratch database.

## 1. MVP definition of done (SPEC.md §"MVP definition of done")

| # | Requirement | Status | Evidence |
|---|---|---|---|
| 1 | Application starts successfully | PASS | manual pty run of `./routine -db <tmp>`; renders and exits cleanly on `q` |
| 2 | Three-column layout works | PASS | `internal/ui/layout_test.go`, `spec_render_test.go`; confirmed visually |
| 3 | Keyboard navigation works exactly as specified | PASS | `internal/ui/spec_navigation_test.go`, `focus_test.go` |
| 4 | Habits can be created | PASS | manual E2E via the huh form; row present in `habits` |
| 5 | Routines can be created | PASS | `internal/app/service_test.go` (`AddRoutine`), `internal/store/routines_test.go` |
| 6 | Habits can optionally belong to routines | PASS | `NewHabit.RoutineID` end-to-end; `internal/app/service_test.go` |
| 7 | Habits can be completed | PASS | manual E2E toggle; `completions` row written, column showed `1/1 100%` |
| 8 | State survives application restart | PASS | `internal/store/spec_persistence_test.go` (close + reopen) |
| 9 | Daily progress works | PASS | `internal/domain/summary_test.go`, `internal/ui/today.go` progress bar |
| 10 | Calendar displays completion state | PASS | `internal/app/service_test.go` calendar assertions; `overview.go:dayCellStyle` |
| 11 | Heatmap displays historical completion | PASS | `internal/ui/spec_heatmap_test.go`, `heatmap_range_test.go` |
| 12 | Reminders can be created | PASS | `internal/app/service_test.go`, `internal/store/reminders_test.go` |
| 13 | Reminder dates appear in calendar | PASS | `CalendarDay.HasReminder` wired; underline in `dayCellStyle` |
| 14 | Week/month/90d/year/all analytics filters work | PASS | `internal/ui/render_test.go` (cycling) + `heatmap_range_test.go` (long ranges are downsampled, not truncated) |
| 15 | `go test ./...` passes | PASS | release gate |
| 16 | `go vet ./...` passes | PASS | release gate |
| 17 | `go build ./cmd/routine` succeeds | PASS | release gate |

## 2. Navigation model (SPEC.md §"Navigation")

| Requirement | Status | Evidence |
|---|---|---|
| Left/Right moves between columns | PASS | `spec_navigation_test.go` |
| Tab/Shift+Tab moves between blocks in current column | PASS | `spec_navigation_test.go` |
| Up/Down moves between selectable items in current block | PASS | `spec_navigation_test.go` |
| Enter activates action / toggles habit / confirms forms | PASS | `spec_activation_test.go` |
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
| Column widths calculated dynamically | PASS | `columnWidths`, `internal/ui/layout_test.go` |
| Below minimum size, shows a clear message | PASS | `MinWidth` 96 / `MinHeight` 35, `renderMinSize`; pinned by `layout_test.go` |

## 5. Styling

| Requirement | Status | Evidence |
|---|---|---|
| No bundled/selected font | PASS | no font assets in the tree |
| ANSI 16-color palette | PASS | `internal/theme/theme.go` uses only `lipgloss.Color("0")`..`("15")` |
| No hardcoded RGB colors | PASS | no `#` colors anywhere in `internal/theme` |
| Terminal background remains terminal-controlled | PASS | no `Background(...)` call in `internal/theme` |
| Square/simple borders, no gradients, sparing Unicode | PASS | source inspection; confirmed visually |

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
4. For long heatmap ranges the block merges weeks into a single column
   (`k` weeks per column, shown in the title as `(Nw/col)`) because a
   53-week year cannot fit in a ~27-column pane. Full range is represented;
   per-week resolution is not.
