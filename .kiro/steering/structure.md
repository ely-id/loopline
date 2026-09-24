# Structure

## Layout

```
cmd/loopline/      # main entry point; the ONLY place that wires everything
internal/domain/   # pure business types & logic (Date, Habit, Schedule, stats)
internal/store/    # SQLite persistence; implements app.Store
internal/app/      # Service: orchestrates domain + store; defines ports/views
internal/theme/    # ANSI-16 lipgloss/huh styling
internal/ui/       # Bubble Tea models, layout, forms, navigation
docs/              # ARCHITECTURE.md (cross-package contract), QA.md
scripts/           # helper scripts (e.g. codex-review)
```

## Dependency direction (do not violate)

```
cmd/loopline ─> ui ──> app ──> domain
     │              └─> theme     ▲
     └─> store ───────────────────┘
```

- `internal/domain` imports **only the Go standard library**.
- `internal/app` never imports `ui`, `store`, or `theme` — it depends on the
  `Store` interface in `ports.go`, wired by `cmd/loopline`.
- `internal/store` is imported only by `cmd/loopline`.
- `internal/ui` never touches SQLite and never calls `time.Now()`; it talks to
  the backend only through the `Backend` interface declared in `internal/ui`.
- `internal/theme` imports no first-party package.

## Package roles

- **domain** — pure, no I/O. Schedules, active/scheduled windows, streaks, and
  daily summaries. `date.go` and `model.go` are frozen contract files (do not
  change exported signatures without agreement).
- **store** — migrations are an append-only ordered slice tracked in
  `schema_migrations`; never edit an applied migration. `completions` is the
  source of truth; `daily_summaries` is a derived write-through cache.
- **app** — `Service` with `Snapshot`, toggle/add/edit/delete methods.
  `ports.go` (Store) and `view.go` (read models) are seeded/frozen.
- **ui** — the Root model owns all navigation, focus, and ephemeral expansion
  state; child models are presentational (data in, string out). All backend
  calls run inside `tea.Cmd`s; after any mutation the UI refetches a `Snapshot`
  and replaces its data wholesale (no local mutation of view data).

## Testing conventions

- Unit tests for a package use plain names next to the code:
  `date_test.go`, `focus_test.go`, `service_test.go`.
- Spec/conformance tests are prefixed `spec_`: `spec_navigation_test.go`,
  `spec_persistence_test.go`. Keep new spec tests under this prefix.
- Use `store.OpenMemory()` for in-memory DB tests and `domain.FixedClock` for
  deterministic time.

`docs/ARCHITECTURE.md` is the authoritative cross-package contract; consult it
before changing package boundaries or exported signatures.
