package state

import (
	"sort"
	"time"
)

type discoveryRecord struct {
	ID          string `gorm:"primaryKey;column:id"`
	Kind        string `gorm:"column:kind"`
	Name        string `gorm:"column:name"`
	Detail      string `gorm:"column:detail"`
	Confidence  string `gorm:"column:confidence"`
	Monitored   bool   `gorm:"column:monitored"`
	ManagedIn   string `gorm:"column:managed_in"`
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	New         bool `gorm:"column:is_new"` // seen by a scan, not yet reviewed by a person
	Dismissed   bool
	DismissedAt time.Time
}

func (discoveryRecord) TableName() string { return "discovery_items" }

// DiscoveryItem is one thing a scan found, with what the person did about it.
type DiscoveryItem struct {
	ID          string
	Kind        string
	Name        string
	Detail      string
	Confidence  string
	Monitored   bool
	ManagedIn   string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	New         bool
	Dismissed   bool
	DismissedAt time.Time
}

func (r discoveryRecord) item() DiscoveryItem {
	return DiscoveryItem{ID: r.ID, Kind: r.Kind, Name: r.Name, Detail: r.Detail, Confidence: r.Confidence, Monitored: r.Monitored,
		ManagedIn: r.ManagedIn, FirstSeenAt: r.FirstSeenAt, LastSeenAt: r.LastSeenAt, New: r.New, Dismissed: r.Dismissed, DismissedAt: r.DismissedAt}
}

// RecordScan stores a scan: new ids are inserted flagged New, known ids are
// refreshed, ids no longer present are removed unless the person dismissed
// them (a dismissal is remembered even while the thing is gone). baseline marks
// the first scan on a host: everything found then is the starting list, not
// "new on this server". It returns the ids that were new.
func (s *Store) RecordScan(items []DiscoveryItem, now time.Time, baseline bool) ([]string, error) {
	var existing []discoveryRecord
	if err := s.db.Find(&existing).Error; err != nil {
		return nil, err
	}
	known := map[string]discoveryRecord{}
	for _, r := range existing {
		known[r.ID] = r
	}
	seen := map[string]bool{}
	var fresh []string
	for _, it := range items {
		seen[it.ID] = true
		r, ok := known[it.ID]
		if !ok {
			r = discoveryRecord{ID: it.ID, FirstSeenAt: now, New: !baseline}
			if !baseline {
				fresh = append(fresh, it.ID)
			}
		}
		r.Kind, r.Name, r.Detail, r.Confidence, r.Monitored, r.ManagedIn, r.LastSeenAt = it.Kind, it.Name, it.Detail, it.Confidence, it.Monitored, it.ManagedIn, now
		if err := s.db.Save(&r).Error; err != nil {
			return nil, err
		}
	}
	for _, r := range existing {
		if !seen[r.ID] && !r.Dismissed {
			if err := s.db.Delete(&discoveryRecord{}, "id = ?", r.ID).Error; err != nil {
				return nil, err
			}
		}
	}
	// Remember that a baseline exists even if that first scan found nothing.
	if err := s.SetLogOffset(discoveryBaselineKey, LogOffset{Cursor: now.UTC().Format(time.RFC3339)}); err != nil {
		return nil, err
	}
	return fresh, nil
}

const discoveryBaselineKey = "__discovery_baseline__"

// ListDiscovery returns everything stored, newest first seen first.
func (s *Store) ListDiscovery() ([]DiscoveryItem, error) {
	var rs []discoveryRecord
	if err := s.db.Find(&rs).Error; err != nil {
		return nil, err
	}
	out := make([]DiscoveryItem, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.item())
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].FirstSeenAt.Equal(out[j].FirstSeenAt) {
			return out[i].FirstSeenAt.After(out[j].FirstSeenAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// SetDismissed dismisses or restores one suggestion. It reports whether the id existed.
func (s *Store) SetDismissed(id string, dismissed bool, now time.Time) (bool, error) {
	res := s.db.Model(&discoveryRecord{}).Where("id = ?", id).Updates(map[string]any{"dismissed": dismissed, "dismissed_at": now, "is_new": false})
	return res.RowsAffected > 0, res.Error
}

// MarkReviewed clears the "new" flag, for when a person has looked at the list.
func (s *Store) MarkReviewed() error {
	return s.db.Model(&discoveryRecord{}).Where("is_new = ?", true).Update("is_new", false).Error
}

// HasScanned reports whether a baseline scan was ever recorded.
func (s *Store) HasScanned() bool {
	_, ok, _ := s.GetLogOffset(discoveryBaselineKey)
	return ok
}
