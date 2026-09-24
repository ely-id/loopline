# Product

loopline is a keyboard-first terminal UI (TUI) application for tracking daily
habits, routines, and reminders. It runs entirely in the terminal and stores
everything locally in a single SQLite database under the user's XDG data
directory.

## Core concepts

- **Habit** — a recurring task with a schedule (daily or specific weekdays).
  Habits can belong to a routine. Habits are archived, never hard-deleted, so
  historical completion counts stay stable.
- **Routine** — a named group of habits. Deleting a routine keeps its habits as
  ungrouped (children survive).
- **Completion** — one record per (habit, date) marking a habit done that day.
  The `completions` table is the single source of truth for what happened.
- **Reminder** — a one-off dated note that can be marked complete.
- **Streaks / summaries** — analytics (current streak, best streak, completion
  heatmap) derived from completions plus the habit set.

## Interface shape

Three content panels rendered left-to-right as Habits, Reminders, Overview,
with a responsive identity/navigation rail or header. Navigation is global
(pick a panel) then local (traverse items inside it). All create/edit/delete
flows use in-terminal modal forms; destructive actions require confirmation.

The product is offline, single-user, and has no network or account features.
