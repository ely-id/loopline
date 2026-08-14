package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *SQLite {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return s
}

func TestOpenMemoryRunsMigrations(t *testing.T) {
	s := newTestStore(t)
	var version int
	if err := s.db.QueryRow(`SELECT version FROM schema_migrations WHERE version = 1`).Scan(&version); err != nil {
		t.Fatalf("expected migration 1 recorded: %v", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	if err := migrate(context.Background(), s.db); err != nil {
		t.Fatalf("second migrate call should be a no-op, got: %v", err)
	}
}

func TestPragmasApplied(t *testing.T) {
	s := newTestStore(t)

	var fk int
	if err := s.db.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
		t.Fatalf("query foreign_keys pragma: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys pragma = %d, want 1", fk)
	}

	var busyTimeout int
	if err := s.db.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout pragma: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout pragma = %d, want 5000", busyTimeout)
	}
}

func TestOpenCreatesDatabaseFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "loopline.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected database file to exist: %v", err)
	}
}

func TestDefaultPathUsesXDGDataHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(dir, "loopline", "loopline.db")
	if path != want {
		t.Errorf("DefaultPath() = %q, want %q", path, want)
	}

	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("expected data directory to exist: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("data directory mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestDefaultPathFallsBackToHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", dir)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(dir, ".local", "share", "loopline", "loopline.db")
	if path != want {
		t.Errorf("DefaultPath() = %q, want %q", path, want)
	}
}
