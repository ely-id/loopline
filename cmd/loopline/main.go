// Command loopline is the executable entry point for the keyboard-first
// daily habit, routine and reminder manager.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"loopline/internal/app"
	"loopline/internal/demo"
	"loopline/internal/domain"
	"loopline/internal/store"
	"loopline/internal/ui"
)

var _ app.Store = (*store.SQLite)(nil)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "loopline:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "database file (default: XDG data dir)")
	demoMode := flag.Bool("demo", false, "seed an empty database with sample data before starting")
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
	if *demoMode {
		if err := demo.Seed(ctx, db, domain.SystemClock{}.Today()); err != nil {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}
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
