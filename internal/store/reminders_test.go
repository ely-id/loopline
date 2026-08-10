package store

import (
	"context"
	"testing"

	"routine-tui/internal/domain"
)

func TestReminderCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateReminder(ctx, domain.Reminder{
		Title: "Pay rent",
		Date:  domain.MustParseDate("2026-09-01"),
		Notes: "before the 5th",
	})
	if err != nil {
		t.Fatalf("CreateReminder: %v", err)
	}

	reminders, err := s.Reminders(ctx)
	if err != nil {
		t.Fatalf("Reminders: %v", err)
	}
	if len(reminders) != 1 || reminders[0].ID != id || reminders[0].Completed {
		t.Fatalf("Reminders() = %+v", reminders)
	}

	updated := reminders[0]
	updated.Completed = true
	updated.Notes = "paid"
	if err := s.UpdateReminder(ctx, updated); err != nil {
		t.Fatalf("UpdateReminder: %v", err)
	}

	reminders, err = s.Reminders(ctx)
	if err != nil {
		t.Fatalf("Reminders: %v", err)
	}
	if !reminders[0].Completed || reminders[0].Notes != "paid" {
		t.Fatalf("Reminders()[0] after update = %+v", reminders[0])
	}

	if err := s.DeleteReminder(ctx, id); err != nil {
		t.Fatalf("DeleteReminder: %v", err)
	}
	reminders, err = s.Reminders(ctx)
	if err != nil {
		t.Fatalf("Reminders: %v", err)
	}
	if len(reminders) != 0 {
		t.Fatalf("expected reminder to be gone, got %+v", reminders)
	}
}

func TestRemindersOrderedByDateThenTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	specs := []domain.Reminder{
		{Title: "Zebra", Date: domain.MustParseDate("2026-09-01")},
		{Title: "Apple", Date: domain.MustParseDate("2026-09-01")},
		{Title: "Anything", Date: domain.MustParseDate("2026-08-01")},
	}
	for _, r := range specs {
		if _, err := s.CreateReminder(ctx, r); err != nil {
			t.Fatalf("CreateReminder(%s): %v", r.Title, err)
		}
	}

	reminders, err := s.Reminders(ctx)
	if err != nil {
		t.Fatalf("Reminders: %v", err)
	}
	want := []string{"Anything", "Apple", "Zebra"}
	if len(reminders) != len(want) {
		t.Fatalf("expected %d reminders, got %d", len(want), len(reminders))
	}
	for i, title := range want {
		if reminders[i].Title != title {
			t.Errorf("Reminders()[%d].Title = %q, want %q", i, reminders[i].Title, title)
		}
	}
}
