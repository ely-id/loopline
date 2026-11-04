# QA Conformance Report

Verification of the current `loopline` interface against the behavior and
layout contracts covered by the repository's tests. The known-good starting
point for this round was `v0.2.0`; this report describes the global/local
navigation and information-architecture refinement built on that baseline.

Status legend: `PASS`, `FAIL`, `PARTIAL`, `N/A`.

Validated with `go fmt ./...`, `go test ./...`, `go vet ./...`, and
`go build ./cmd/loopline`, plus responsive render coverage at 96x33, 119x35,
120x35, 140x45, and 200x60.

## 1. Navigation and interaction

| Requirement | Status | Evidence |
|---|---|---|
| Global navigation starts as an understandable horizontal header or vertical rail | PASS | `spec_navigation_test.go`, `header_footer_test.go`, `layout_test.go` |
| Tab/Shift+Tab cycles Overview, Habits, and Reminders | PASS | `spec_navigation_test.go`; PTY Tab and reverse-Tab checks |
| Enter enters the selected panel | PASS | `spec_navigation_test.go`; PTY run |
| Esc returns to the corresponding global navigation entry | PASS | `spec_navigation_test.go`; PTY run |
| Up/Down forms one continuous local surface across blocks | PASS | `focus_test.go`, `spec_navigation_test.go`; PTY Add → routine header → habit and Add reminder → Calendar → reminder checks |
| Empty blocks and panels cannot trap or invalidate focus | PASS | `focus_test.go`, `spec_navigation_test.go` |
| Edit, toggle/expand, delete confirmation, and activation remain contextual | PASS | `crud_test.go`, `expansion_test.go`, `spec_activation_test.go`; PTY routine/reminder expansion and habit completion |
| Focus is reclamped after editing, deleting, collapsing, and snapshot refresh | PASS | `crud_test.go`, `expansion_test.go`, `spec_navigation_test.go` |

## 2. Information architecture and rendering

| Requirement | Status | Evidence |
|---|---|---|
| Responsive identity/global navigation uses a vertical rail when wide and the horizontal header when compact | PASS | `layout_test.go`, `header_footer_test.go`; exact ASCII rows, breakpoint, text fallback, and minimal fallback covered |
| Navigation and content retain the Habits → Reminders → Overview visual order in both responsive modes | PASS | `layout_test.go`, `spec_navigation_test.go` |
| Wide panel headings map vertical selectors to Overview/Habits/Reminders while compact interiors stay dense | PASS | `layout_test.go`, `header_footer_test.go` |
| Overview contains Lifetime, bounded Weekly Overview, and bottom-anchored Completion | PASS | `weekly_overview_test.go`, `height_budget_test.go`, `year_progress_test.go` |
| Weekly Overview derives only scheduled/completed states for the current Sunday-first display week | PASS | `service_test.go`, `weekly_overview_test.go` |
| Weekly selection stays visible without overwriting Completion | PASS | `weekly_overview_test.go`; viewport uses physical row budget and focused-row windowing |
| Completion uses exact U+F0C8 squares as a chronological, balanced, justified sequence with no weekday/date axis | PASS | `completion_calendar_test.go`; ANSI-aware glyph width pinned at one cell |
| Completion uses a discrete ANSI-16 semantic scale with muted no-data and green completion | PASS | `completion_calendar_test.go`, `theme.go` |
| Reminder Calendar is Sunday-first and retains numeric, adjacent-month, today, and reminder-date semantics | PASS | `completion_calendar_test.go`; PTY month navigation |
| Habits keeps daily progress bottom anchored | PASS | `height_budget_test.go`, `spec_render_test.go`; PTY 96x33 run |
| Reminders keeps Year progress bottom anchored | PASS | `year_progress_test.go`, `height_budget_test.go`; PTY 96x33 run |
| Empty routines remain visible, selectable, editable, collapsible, and deletable | PASS | `service_test.go`, `crud_test.go`, `focus_test.go`; PTY-created `Morning 0/0` routine |
| Footer is centered, one-line, keycap-styled, and context-aware | PASS | `header_footer_test.go`; PTY checks in header, action, calendar, and entity contexts |

## 3. Existing behavior and persistence

| Requirement | Status | Evidence |
|---|---|---|
| Habit, routine, and reminder CRUD remains correct | PASS | `internal/app/service_test.go`, `internal/store/*_test.go`, `internal/ui/crud_test.go`; PTY creation checks |
| Habit and reminder completion persists | PASS | store persistence suite; PTY habit toggle against scratch database |
| Routine and reminder expansion remains ephemeral and safe | PASS | `expansion_test.go`; PTY checks |
| Calendar highlighting and Year calculations remain correct | PASS | `completion_calendar_test.go`, `year_progress_test.go` |
| Database integrity, migrations, and XDG path behavior remain correct | PASS | `internal/store/spec_persistence_test.go`, `store_test.go` |
| Default storage uses the loopline identity | PASS | `store.DefaultPath` resolves `$XDG_DATA_HOME/loopline/loopline.db`; persistence tests pin the new path |

## 4. Responsive behavior and styling

| Requirement | Status | Evidence |
|---|---|---|
| Three dynamic content panels fit at 96x33; the measured rail and proportioned panels fit at 120+ | PASS | `layout_test.go`, `column_fit_test.go`, `height_budget_test.go` |
| Rail/header/footer budgets do not collide with panel borders | PASS | representative full-view and resize dimension tests |
| Below minimum size, a clear centered message replaces the layout | PASS | `layout_test.go`; PTY 80x24 run |
| Long lists and expanded details are windowed on physical terminal rows | PASS | `column_fit_test.go`, `height_budget_test.go`, `weekly_overview_test.go` |
| ANSI-16 only; no page background or truecolor requirement | PASS | `internal/theme/theme.go`, `huh_theme_test.go` |
| Nerd Font icons have ANSI-aware width coverage | PASS | `layout_test.go`, `completion_calendar_test.go` |

## 5. Naming

| Requirement | Status | Evidence |
|---|---|---|
| Normal command path is `cmd/loopline` | PASS | `go build ./cmd/loopline` |
| Obsolete `cmd/routine` source path is removed | PASS | tracked-file search |
| Legitimate Routine domain concepts are unchanged | PASS | routine service, store, UI, and regression suites remain green |
| Tracked application identity no longer uses the former project name | PASS | tracked-file search; the unchanged external Git remote is reported separately |

## 6. Known technical debt (unchanged, not release-blocking)

1. `daily_summaries` is a derived write-through cache. All analytics derive
   from completions plus habits, so missing cache rows are not load-bearing.
2. `Store.SetCompletion` is test-only and does not refresh `daily_summaries`;
   production uses the atomic `ToggleCompletion` path.
3. UI backend commands use `context.Background()` rather than the Bubble Tea
   program context and are bounded by SQLite's five-second busy timeout.
