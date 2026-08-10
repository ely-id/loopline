package domain

import "fmt"

// ActiveOn reports whether the habit exists on the given date: created on or
// before d, and not yet archived (or archived strictly after d).
func (h Habit) ActiveOn(d Date) bool {
	if d.Before(h.CreatedOn) {
		return false
	}
	if h.ArchivedOn != nil && !d.Before(*h.ArchivedOn) {
		return false
	}
	return true
}

// ScheduledOn reports whether the habit is scheduled on the given date: it
// must be active on d and its schedule must apply on d.
func (h Habit) ScheduledOn(d Date) bool {
	return h.ActiveOn(d) && h.Schedule.AppliesOn(d)
}

// Validate reports whether the habit has a non-empty title and a valid
// schedule.
func (h Habit) Validate() error {
	if h.Title == "" {
		return fmt.Errorf("habit title must not be empty")
	}
	if err := h.Schedule.Validate(); err != nil {
		return fmt.Errorf("habit schedule: %w", err)
	}
	return nil
}
