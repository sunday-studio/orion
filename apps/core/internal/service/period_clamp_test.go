package service

import "testing"

func TestParsePeriodClampsHostileValues(t *testing.T) {
	for in, want := range map[string]int{
		"":                      90,
		"30d":                   30,
		"365d":                  365,
		"366d":                  365,
		"999999999d":            365,
		"99999999999999999999d": 365,
		"0d":                    90,
		"garbage":               90,
	} {
		if got := parsePeriod(in); got != want {
			t.Fatalf("parsePeriod(%q) = %d, want %d", in, got, want)
		}
	}
}
