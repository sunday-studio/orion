package api

import (
	"testing"
	"time"
)

func TestParseLogTime(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
		ok   bool
	}{
		{"", time.Time{}, true},
		{"now", now, true},
		{"-1h", now.Add(-time.Hour), true},
		{"-7d", now.AddDate(0, 0, -7), true},
		{"2026-11-08T10:00:00Z", time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC), true},
		{"later", time.Time{}, false},
		{"-", time.Time{}, false},
	}
	for _, tc := range cases {
		got, ok := parseLogTime(tc.in, now)
		if ok != tc.ok || !got.Equal(tc.want) {
			t.Errorf("parseLogTime(%q) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
