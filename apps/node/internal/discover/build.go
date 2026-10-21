package discover

import (
	"fmt"
	"strings"

	"orion/node/internal/config"
)

// Select returns the candidates whose IDs are listed, or whose confidence is in
// the given set when ids contains "high", "medium", "low" or "all".
func Select(all []Candidate, ids []string) ([]Candidate, error) {
	byID := map[string]Candidate{}
	for _, c := range all {
		byID[c.ID] = c
	}
	var out []Candidate
	seen := map[string]bool{}
	take := func(c Candidate) {
		if !seen[c.ID] {
			seen[c.ID] = true
			out = append(out, c)
		}
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		switch id {
		case "all":
			for _, c := range all {
				take(c)
			}
		case "high", "medium", "low":
			for _, c := range all {
				if string(c.Confidence) == id {
					take(c)
				}
			}
		default:
			c, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("%q was not discovered on this host (run `orion-node discover` to list ids)", id)
			}
			take(c)
		}
	}
	return out, nil
}

// BuildConfig returns base with the selected candidates' monitors added.
// Existing monitors with the same name are kept as they are, so re-running
// setup never overwrites hand edits.
func BuildConfig(base config.UserConfig, selected []Candidate) config.UserConfig {
	have := map[string]bool{}
	for _, m := range base.Monitors {
		have[m.Name] = true
	}
	for _, c := range selected {
		if c.StoragePath != "" {
			AddStoragePath(&base, c.StoragePath)
			continue
		}
		if c.Monitor == nil || have[c.Monitor.Name] {
			continue
		}
		have[c.Monitor.Name] = true
		base.Monitors = append(base.Monitors, *c.Monitor)
	}
	return base
}
