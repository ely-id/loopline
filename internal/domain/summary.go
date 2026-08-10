package domain

import (
	"math"
	"sort"
)

// Percent is the completion percentage for the day, 0..100. A day with
// nothing scheduled reports 0.
func (s DailySummary) Percent() int {
	if s.Scheduled <= 0 {
		return 0
	}
	return int(math.Round(float64(s.Completed) * 100 / float64(s.Scheduled)))
}

// IsComplete reports whether every scheduled habit was completed. A day with
// nothing scheduled is not complete.
func (s DailySummary) IsComplete() bool {
	return s.Scheduled > 0 && s.Completed >= s.Scheduled
}

// SummarizeDay computes the scheduled/completed counts for one date from the
// full habit list and a set of habit IDs completed on that date.
func SummarizeDay(habits []Habit, done map[int64]bool, d Date) DailySummary {
	var scheduled, completed int
	for _, h := range habits {
		if !h.ScheduledOn(d) {
			continue
		}
		scheduled++
		if done[h.ID] {
			completed++
		}
	}
	return DailySummary{Date: d, Scheduled: scheduled, Completed: completed}
}

// SummarizeRange computes one DailySummary per date in [from, to] inclusive.
func SummarizeRange(habits []Habit, comps []Completion, from, to Date) []DailySummary {
	index := CompletionIndex(comps)
	dates := DateRange(from, to)
	out := make([]DailySummary, 0, len(dates))
	for _, d := range dates {
		out = append(out, SummarizeDay(habits, index[d], d))
	}
	return out
}

// CompletionIndex groups completions by date, then by habit ID, for fast
// lookup by SummarizeRange and HabitStreak.
func CompletionIndex(comps []Completion) map[Date]map[int64]bool {
	idx := make(map[Date]map[int64]bool, len(comps))
	for _, c := range comps {
		m, ok := idx[c.Date]
		if !ok {
			m = make(map[int64]bool)
			idx[c.Date] = m
		}
		m[c.HabitID] = true
	}
	return idx
}

// HabitStreak counts consecutive scheduled days a single habit was completed,
// walking backwards from upTo. Dates the habit is not scheduled on are
// skipped without breaking the streak. An incomplete scheduled date breaks
// the streak, except upTo itself: if upTo is scheduled but incomplete, it is
// simply not counted and the walk continues, so a streak that is otherwise
// intact is not broken by today not being done yet.
func HabitStreak(h Habit, comps map[Date]map[int64]bool, upTo Date) int {
	streak := 0
	for d := upTo; !d.Before(h.CreatedOn); d = d.AddDays(-1) {
		if !h.ScheduledOn(d) {
			continue
		}
		if comps[d][h.ID] {
			streak++
			continue
		}
		if d.Equal(upTo) {
			continue
		}
		break
	}
	return streak
}

// CurrentStreak counts consecutive complete days walking backwards from
// today, over the given day summaries. Days with nothing scheduled are
// skipped without breaking the streak; an incomplete scheduled day breaks it,
// except today itself, matching HabitStreak's treatment of upTo.
func CurrentStreak(summaries []DailySummary, today Date) int {
	index := make(map[Date]DailySummary, len(summaries))
	var earliest Date
	has := false
	for _, s := range summaries {
		index[s.Date] = s
		if !has || s.Date.Before(earliest) {
			earliest = s.Date
			has = true
		}
	}
	if !has {
		return 0
	}
	streak := 0
	for d := today; !d.Before(earliest); d = d.AddDays(-1) {
		s, ok := index[d]
		if !ok || s.Scheduled == 0 {
			continue
		}
		if s.IsComplete() {
			streak++
			continue
		}
		if d.Equal(today) {
			continue
		}
		break
	}
	return streak
}

// BestStreak returns the longest run of complete days across all summaries,
// in any order. Days with nothing scheduled are skipped without breaking a
// run.
func BestStreak(summaries []DailySummary) int {
	sorted := make([]DailySummary, len(summaries))
	copy(sorted, summaries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	var best, current int
	for _, s := range sorted {
		if s.Scheduled == 0 {
			continue
		}
		if s.IsComplete() {
			current++
			if current > best {
				best = current
			}
			continue
		}
		current = 0
	}
	return best
}
