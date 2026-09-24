# Tech

## Language & build

- **Go** (module `loopline`, Go 1.26.5). Standard `go` toolchain, no Makefile.
- Prefer the **standard library**. No ORM, no DI framework, no new abstractions.
- The dependency set is **fixed**. Do not run `go get` or `go mod edit`; if a
  new dependency seems necessary, raise it rather than adding it.

## Stack

- **Bubble Tea v2** (`charm.land/bubbletea/v2`) — TUI runtime. `Update` returns
  `(tea.Model, tea.Cmd)`; `View()` returns `tea.View`; key events are
  `tea.KeyPressMsg` (use `msg.String()`).
- **Bubbles v2**, **Lipgloss v2**, **Huh v2** (all `charm.land/*/v2`) — widgets,
  styling, and modal forms.
- **modernc.org/sqlite** — pure-Go SQLite driver, registered as driver name
  `sqlite` (CGo-free).

Note: dependencies use the `charm.land/...` module paths, not
`github.com/charmbracelet/...`.

## Styling constraints

- **ANSI-16 only.** Use `lipgloss.Color("0")`..`lipgloss.Color("15")`. No RGB
  hex unless justified in a comment. Never set a page-body background so the
  terminal background shows through.
- Modal forms use `theme.HuhTheme()` (built from the same ANSI-16 palette), not
  huh's stock RGB theme.

## Time & data rules

- `domain.Date` is the only calendar type crossing package boundaries; SQLite
  stores dates as `TEXT` in `YYYY-MM-DD` (`domain.DateLayout`).
- Only `cmd/loopline` may call `time.Now()` (via `domain.SystemClock`). Every
  other package takes a `domain.Clock`; tests use `domain.FixedClock`.
- Weeks are ISO 8601 (Monday first) in the domain; Sunday-first is a
  presentation-only concern.

## Common commands

Run these after meaningful changes:

```sh
go fmt ./...
go vet ./...
go test ./...
go build ./cmd/loopline      # release build / entry point
```

Scope a build to one package while the tree is mid-change:

```sh
go build ./internal/<pkg>/...
```

Run the app against a scratch database:

```sh
go run ./cmd/loopline --db /tmp/loopline.db
```

`scripts/codex-review` runs an external read-only code review (requires the
`codex` CLI).
