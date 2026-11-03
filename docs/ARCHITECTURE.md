# loopline — Architecture Contract

Authoritative for cross-package boundaries and the implementation constraints
that keep the product behavior consistent across packages.

## 1. Dependency direction

```
cmd/loopline ─> internal/ui ──> internal/app ──> internal/domain
     │                  │                            ▲
     │                  └──> internal/theme          │
     └──> internal/store ──────────────────────────-─┘
```

- `internal/domain` imports **only the Go standard library**.
- `internal/store` implements `app.Store`; nothing imports `store` except `cmd/loopline`.
- `internal/app` never imports `ui`, `store` or `theme`.
- `internal/ui` never touches SQLite and never calls `time.Now()`.
- `cmd/loopline` wires everything: resolve XDG path → open SQLite → enable
  foreign keys → run migrations → build `app.Service` → build `ui.Root` → run
  the Bubble Tea program. Release builds use `go build ./cmd/loopline`.

## 2. File ownership (hard rule: do not edit files you do not own)

| Owner | Files |
|---|---|
| **data** | `internal/domain/**` (except the frozen files below), `internal/store/**` |
| **ui** | `internal/ui/**`, `internal/theme/**` |
| **qa** | test files named `spec_*_test.go` in any package; `internal/app/*_test.go`; `cmd/loopline/*_test.go` |
| **lead** | `internal/app/*.go` (non-test), `cmd/loopline/*.go`, `docs/**`, `go.mod` |

Frozen contract files, seeded by the lead. `data` owns them from now on but may
**not** change exported signatures without asking the lead first:
`internal/domain/date.go`, `internal/domain/model.go`.

`data` and `ui` write their own unit tests using **plain names**
(`date_test.go`, `focus_test.go`). `qa` only ever creates files prefixed
`spec_` so two agents can never collide on a filename.

Never run `go mod edit` / `go get`; the dependency set is fixed. Tell the lead
if you believe a new dependency is required.

## 3. Date and time policy (single source)

- `domain.Date` is the only calendar type crossing package boundaries.
- `domain.Clock` is the only source of "today". **No package other than
  `cmd/loopline` may call `time.Now()`.** `domain.SystemClock` is constructed in
  `cmd/loopline` and passed to `app.Service`. Tests use `domain.FixedClock`.
- Domain weeks are ISO 8601, Monday first. `Date.StartOfWeek()` returns Monday.
  Weekly Overview and the Reminder Calendar are presentation-specific
  Sunday-first views and derive their display boundary without changing the
  domain helper or the Year block's ISO week number.
- SQLite stores dates as `TEXT` in `YYYY-MM-DD` (`domain.DateLayout`), which
  sorts and compares lexicographically.
- Timestamps (`completed_at`, `created_at`) are stored as RFC3339 `TEXT`; they
  are informational only and never drive calculations.

## 4. History: single source of truth

- The `completions` table is the **source of truth** for what was done. One row
  per `(habit_id, date)`. Never mutated after the fact except by toggling.
- Habits are **archived, never hard-deleted** (`archived_on` date).
  A habit is scheduled on date D only if
  `created_on <= D && (archived_on == nil || D < archived_on) && schedule.AppliesOn(D)`.
  This keeps historical "scheduled" counts stable.
- `daily_summaries` is a **derived, write-through record**, required by SPEC's
  persistence list. Lifecycle rule: the summary for a date is rewritten inside
  the **same SQLite transaction** as the completion change that affects it
  (`Store.ToggleCompletion`), with the completed count recounted from committed
  table state rather than from a pre-read value. Mutations that change the
  habit set without touching completions (add habit, archive habit) call
  `Store.RefreshDailySummary`, which does the same recount in its own
  transaction. **All analytics read paths derive from `completions` + habits**,
  so the table is never load-bearing for correctness.
- Archiving is **write-once**: `Service.DeleteHabit` is a no-op on an
  already-archived habit, because moving `archived_on` forward would make the
  habit count as scheduled again for the dates in between and rewrite history.
- Deleting a routine (`DeleteRoutine`) is a hard delete of the routine row plus
  `routine_id -> NULL` on its habits, in one transaction, reinforced by
  `ON DELETE SET NULL`.

## 5. Package responsibilities

### internal/domain (owner: data)

Pure, no I/O. Already seeded: `date.go` (Date, Clock), `model.go` (Habit,
Routine, Completion, Reminder, Schedule, DailySummary).

To implement:

```go
// schedule.go
func (s Schedule) AppliesOn(d Date) bool
func (s Schedule) Validate() error
func (s Schedule) String() string          // "daily" | "mo we fr"
func (w WeekdaySet) Has(time.Weekday) bool
func (w WeekdaySet) With(time.Weekday) WeekdaySet
func WeekdaysOf(days ...time.Weekday) WeekdaySet

// habit.go
func (h Habit) ActiveOn(d Date) bool       // created_on..archived_on window
func (h Habit) ScheduledOn(d Date) bool    // ActiveOn && Schedule.AppliesOn
func (h Habit) Validate() error            // non-empty title, valid schedule

// summary.go — pure statistics over habits + completions
func (s DailySummary) Percent() int        // 0..100; 0 scheduled -> 0
func (s DailySummary) IsComplete() bool    // Scheduled > 0 && Completed >= Scheduled
func SummarizeDay(habits []Habit, done map[int64]bool, d Date) DailySummary
func SummarizeRange(habits []Habit, comps []Completion, from, to Date) []DailySummary
func CompletionIndex(comps []Completion) map[Date]map[int64]bool
func HabitStreak(h Habit, comps map[Date]map[int64]bool, upTo Date) int
func CurrentStreak(summaries []DailySummary, today Date) int
func BestStreak(summaries []DailySummary) int
```

Streak semantics (decided by lead, do not diverge): a streak counts backwards
from `upTo` over dates the habit is scheduled on; unscheduled dates are skipped
without breaking the streak; an incomplete scheduled date breaks it. Today
being incomplete does **not** break a streak that is otherwise intact —
counting starts at `upTo` and stops at the first incomplete scheduled day
strictly before it if `upTo` itself is incomplete.

`CurrentStreak`/`BestStreak` operate on day summaries: a day counts when
`IsComplete()`; days with `Scheduled == 0` are skipped, not breaks.

### internal/store (owner: data)

- `store.DefaultPath() (string, error)` — `$XDG_DATA_HOME/loopline/loopline.db`,
  falling back to `~/.local/share/loopline/loopline.db`; creates the directory
  with `0o700`.
- `store.Open(path string) (*store.SQLite, error)` — `modernc.org/sqlite`,
  driver name `sqlite`; sets `PRAGMA foreign_keys=ON`, `journal_mode=WAL`,
  `busy_timeout=5000`; runs migrations; `db.SetMaxOpenConns(1)`.
- `store.OpenMemory() (*store.SQLite, error)` — in-memory DB for tests.
- `*store.SQLite` implements every method of `app.Store` (see
  `internal/app/ports.go`). Compile-time assertion lives in `cmd/loopline`.
- Migrations: ordered slice of `{Version int, SQL string}` applied inside one
  transaction each, tracked in a `schema_migrations` table. Never edit an
  applied migration; append a new one.

Schema (v1):

```sql
CREATE TABLE routines (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  created_on TEXT NOT NULL,
  archived_on TEXT
);
CREATE TABLE habits (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  routine_id INTEGER REFERENCES routines(id) ON DELETE SET NULL,
  schedule_kind TEXT NOT NULL,
  schedule_weekdays INTEGER NOT NULL DEFAULT 127,
  position INTEGER NOT NULL DEFAULT 0,
  created_on TEXT NOT NULL,
  archived_on TEXT,
  notes TEXT NOT NULL DEFAULT ''
);
CREATE TABLE completions (
  habit_id INTEGER NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
  date TEXT NOT NULL,
  completed_at TEXT NOT NULL,
  PRIMARY KEY (habit_id, date)
);
CREATE INDEX idx_completions_date ON completions(date);
CREATE TABLE reminders (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  title TEXT NOT NULL,
  date TEXT NOT NULL,
  notes TEXT NOT NULL DEFAULT '',
  completed INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_reminders_date ON reminders(date);
CREATE TABLE daily_summaries (
  date TEXT PRIMARY KEY,
  scheduled INTEGER NOT NULL,
  completed INTEGER NOT NULL
);
```

### internal/app (owner: lead)

`ports.go` (Store interface) and `view.go` (read models) are seeded and frozen.
`service.go` implements:

```go
func New(store Store, clock domain.Clock) *Service
func (s *Service) Today() domain.Date
func (s *Service) Snapshot(ctx, SnapshotRequest) (Snapshot, error)
func (s *Service) ToggleHabit(ctx, habitID int64) error
func (s *Service) AddHabit(ctx, NewHabit) error
func (s *Service) AddRoutine(ctx, NewRoutine) error
func (s *Service) AddReminder(ctx, NewReminder) error
func (s *Service) EditHabit(ctx, id int64, NewHabit) error
func (s *Service) EditRoutine(ctx, id int64, NewRoutine) error
func (s *Service) EditReminder(ctx, id int64, NewReminder) error
func (s *Service) ToggleReminder(ctx, id int64) error
func (s *Service) DeleteHabit(ctx, id int64) error
func (s *Service) DeleteRoutine(ctx, id int64) error
func (s *Service) DeleteReminder(ctx, id int64) error
```

### internal/theme (owner: ui)

ANSI-16 only. Use `lipgloss.Color("0")`..`lipgloss.Color("15")`; never set a
background on the page body so the terminal background shows through. No RGB
hex unless justified in a comment. Exposes a `theme.Theme` value built by
`theme.New()` holding named `lipgloss.Style` fields, including the discrete
completion heatmap scale. Every active day uses exact U+F0C8; ANSI colour is
primary, with faint no-data and bold/underlined current-day treatment as quiet
redundant cues.

This package also owns the form palette: `theme.HuhTheme()` returns a
`huh.Theme` built from the same ANSI-16 colours, applied to every modal in
`internal/ui/forms.go`. Without it huh falls back to its stock `ThemeCharm`,
which hardcodes 24-bit RGB and 256-colour indices and so violates SPEC's
styling rules. This is why `internal/theme` imports `charm.land/huh/v2` as
well as lipgloss; the dependency direction is unaffected (theme still imports
no first-party package).

### internal/ui (owner: ui)

Bubble Tea **v2** (`charm.land/bubbletea/v2`): `Update(tea.Msg) (tea.Model, tea.Cmd)`,
`View() tea.View`, key messages are `tea.KeyPressMsg` (`msg.String()`).

Root model owns *all* navigation, focus and ephemeral expansion state:

```go
type Focus struct {
    Column int // 0 overview, 1 today, 2 reminders
    Block  int // index within the column's block list
    Item   int // index within the focused block's selectable items
}
```

`NavigationMode` is either global navigation or panel. In global mode
`Focus.Column` selects the corresponding horizontal-header or vertical-rail
entry. Enter switches to panel mode. In panel mode a
flattened list of every selectable `(Block, Item)` pair provides one continuous
Up/Down surface; empty blocks contribute no targets. Esc returns to global mode
without changing `Focus.Column`.

Modal state is a separate field on the root; when a modal is open it consumes
every key except `esc`. Child models are **presentational**: they receive
`Focus` + data and return a string; they hold no independent focus state.
Routine collapsed IDs and the one expanded reminder ID also live on Root and
are never persisted.

Content panels render left-to-right as Habits, Reminders, Overview. Their
logical IDs remain stable and their blocks are:

- Column 0 (overview): `0` lifetime history, `1` bounded current-week habit
  matrix, `2` monthly completion heatmap. Completion is bottom anchored.
- Column 1 (habits): `0` Add habit/Add routine, then one block per habit group —
  ungrouped first (only when non-empty), followed by every active routine in
  position order, including `0/0` routines. A routine header is item 0; expanded
  children follow it.
- Column 2 (reminders): `0` Add reminder, `1` conventional calendar, `2`
  upcoming reminders list. Non-interactive Year progress is bottom anchored.

Keys: global `tab/shift+tab` panel selection, `enter` enters
the selected panel, panel `up/down` traverses all local targets, `esc` returns
to global navigation, `enter` activates/toggles, `e` edits the selected entity, `t`
expand/collapse structured items, physical `Delete` opens a destructive
confirmation, `</>` (and `[/]`) previous/next month on either calendar,
`esc` close modal, `?` help, `q` quit when no modal is open. `h/j/k/l` may be
added as aliases.

Responsive: compute all region widths from `tea.WindowSizeMsg`. At 120 cells
and wider, use a measured ASCII identity/navigation rail followed by Habits,
Reminders and Overview panels; extra panel width is distributed 1:2:2 so Habits
stays compact. From `ui.MinWidth = 96` through 119 cells, retain
the established three-panel composition and horizontal header. Below 96 cells
or `ui.MinHeight = 33`, render a single centred minimum-size message and nothing
else. The compact header uses the embedded four-row ASCII mark only when width
and height are comfortable, then falls back to textual `loopline`, then the
minimal identity mark. Fixed lower blocks are reserved before variable
list/viewport heights are calculated.
Height budgeting must measure **wrapped** height at the real usable column
width, not logical string count.

The UI talks to the backend only through this interface, declared in
`internal/ui` so tests can fake it:

```go
type Backend interface {
    Today() domain.Date
    Snapshot(ctx context.Context, req app.SnapshotRequest) (app.Snapshot, error)
    ToggleHabit(ctx context.Context, habitID int64) error
    AddHabit(ctx context.Context, in app.NewHabit) error
    AddRoutine(ctx context.Context, in app.NewRoutine) error
    AddReminder(ctx context.Context, in app.NewReminder) error
    EditHabit(ctx context.Context, id int64, in app.NewHabit) error
    EditRoutine(ctx context.Context, id int64, in app.NewRoutine) error
    EditReminder(ctx context.Context, id int64, in app.NewReminder) error
    ToggleReminder(ctx context.Context, id int64) error
    DeleteHabit(ctx context.Context, id int64) error
    DeleteRoutine(ctx context.Context, id int64) error
    DeleteReminder(ctx context.Context, id int64) error
}
```

All backend calls happen inside `tea.Cmd`s that return a message; `Update`
never blocks on I/O. After any mutation the UI requests a fresh `Snapshot` and
replaces its data wholesale — no local mutation of view data.

Add/edit forms and destructive confirmations reuse `huh/v2` inside the modal.
Edit forms are pre-populated and update stable IDs in place. Confirmations
default to cancel; routine deletion explicitly states that children survive as
ungrouped habits. `huh.Form.Update` returns `(huh.Model, tea.Cmd)` and
`Form.View() string`.

## 6. Working agreements

- After meaningful changes run: `go fmt ./...`, `go test ./...`, `go vet ./...`,
  `go build ./cmd/loopline`. The tree may not compile during phase 1 while
  another agent's package is still stubbed — that is expected; make sure *your*
  package compiles (`go build ./internal/<yours>/...`).
- Prefer the standard library. No new abstractions, no ORM, no DI framework.
- Keep commits/checkpoints small and coherent. Do not commit unless the lead
  asks.
- Message the lead when blocked, when you need a frozen signature changed, or
  when you finish a task.
