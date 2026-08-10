// Command routine is the routine-tui entry point: a keyboard-first daily
// habit, routine and reminder manager for the terminal.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"routine-tui/internal/app"
	"routine-tui/internal/domain"
	"routine-tui/internal/store"
	"routine-tui/internal/ui"
)

// compile-time proof that the SQLite store satisfies the application's
// persistence contract.
var _ app.Store = (*store.SQLite)(nil)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "routine:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "database file (default: XDG data dir)")
	flag.Parse()

	path := *dbPath
	if path == "" {
		p, err := store.DefaultPath()
		if err != nil {
			return fmt.Errorf("resolve database path: %w", err)
		}
		path = p
	}

	db, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	service := app.New(db, domain.SystemClock{})
	if err := service.EnsureToday(ctx); err != nil {
		return fmt.Errorf("prepare today: %w", err)
	}

	program := tea.NewProgram(ui.New(service), tea.WithContext(ctx))
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run ui: %w", err)
	}
	return nil
}
