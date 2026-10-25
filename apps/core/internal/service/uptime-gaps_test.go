package service

import (
	"testing"
	"time"
)

func TestMissedChecks(t *testing.T) {
	day := time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC)
	cov := timeSpan{day, day.AddDate(0, 0, 1)}
	every := func(from time.Time, n int, step time.Duration) []time.Time {
		out := make([]time.Time, n)
		for i := range out {
			out[i] = from.Add(time.Duration(i) * step)
		}
		return out
	}
	minute := time.Minute

	t.Run("steady reports miss nothing", func(t *testing.T) {
		if got := missedChecks(every(day, 1440, minute), cov, minute, nil); got != 0 {
			t.Fatalf("missed = %d, want 0", got)
		}
	})
	t.Run("one late check is not downtime", func(t *testing.T) {
		reports := append(every(day, 100, minute), every(day.Add(103*minute), 1337, minute)...)
		if got := missedChecks(reports, cov, minute, nil); got != 0 {
			t.Fatalf("missed = %d, want 0 for a 3 minute silence", got)
		}
	})
	t.Run("a long silence counts the checks that did not run", func(t *testing.T) {
		// Reports until 09:59, then silence until 12:00 on a 1 minute monitor: checks 10:00..11:59 never ran.
		reports := append(every(day, 600, minute), every(day.Add(12*time.Hour), 720, minute)...)
		if got := missedChecks(reports, cov, minute, nil); got != 120 {
			t.Fatalf("missed = %d, want 120", got)
		}
	})
	t.Run("a day with no reports is fully missed", func(t *testing.T) {
		if got := missedChecks(nil, cov, minute, nil); got != 1439 {
			t.Fatalf("missed = %d, want 1439", got)
		}
	})
	t.Run("paused time is not downtime", func(t *testing.T) {
		paused := []timeSpan{{day.Add(10 * time.Hour), day.Add(12 * time.Hour)}}
		reports := append(every(day, 600, minute), every(day.Add(12*time.Hour), 720, minute)...)
		if got := missedChecks(reports, cov, minute, paused); got != 0 {
			t.Fatalf("missed = %d, want 0 while paused", got)
		}
	})
	t.Run("overlapping exclusions are not subtracted twice", func(t *testing.T) {
		excluded := []timeSpan{
			{day, day.Add(12 * time.Hour)},
			{day.Add(6 * time.Hour), day.Add(18 * time.Hour)},
		}
		// 24h silent, the union of the exclusions covers 00:00-18:00, leaving 6h of 1 minute checks.
		if got := missedChecks(nil, cov, minute, excluded); got != 360 {
			t.Fatalf("missed = %d, want 360", got)
		}
	})
	t.Run("slow monitors tolerate longer silences", func(t *testing.T) {
		hourly := time.Hour
		// 5 interval stale window = 5h; a 4h silence is not counted.
		reports := []time.Time{day, day.Add(4 * time.Hour), day.Add(8 * time.Hour), day.Add(12 * time.Hour), day.Add(16 * time.Hour), day.Add(20 * time.Hour)}
		if got := missedChecks(reports, cov, hourly, nil); got != 0 {
			t.Fatalf("missed = %d, want 0", got)
		}
	})
}
