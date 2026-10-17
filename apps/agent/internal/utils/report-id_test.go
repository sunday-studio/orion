package utils

import (
	"regexp"
	"testing"
)

func TestNewReportIDIsUniqueUUIDv4(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 1000 {
		id := NewReportID()
		if !re.MatchString(id) {
			t.Fatalf("NewReportID() = %q, not a UUIDv4", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
