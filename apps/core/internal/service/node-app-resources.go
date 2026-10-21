package service

import (
	"encoding/json"
	"errors"
	"math"
	"orion/core/internal/db"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxAppResourceSeries = 200
	maxAppResourceText   = 300
	maxAppResourceNotes  = 20
)

var appResourceKinds = map[string]bool{"process": true, "unit": true, "container": true, "path": true, "volume": true, "docker": true}

// AppResourceSeries is one measured thing. Rates are per second; a field is absent when the
// node could not measure it (never zero by default).
type AppResourceSeries struct {
	Kind          string   `json:"kind" enums:"process,unit,container,path,volume,docker"`
	Key           string   `json:"key"`
	Name          string   `json:"name"`
	CPUPercent    *float64 `json:"cpu_percent,omitempty"`
	MemoryBytes   *uint64  `json:"memory_bytes,omitempty"`
	IOReadBps     *float64 `json:"io_read_bytes_per_sec,omitempty"`
	IOWriteBps    *float64 `json:"io_write_bytes_per_sec,omitempty"`
	NetRxBps      *float64 `json:"net_rx_bytes_per_sec,omitempty"`
	NetTxBps      *float64 `json:"net_tx_bytes_per_sec,omitempty"`
	Pids          *int     `json:"pids,omitempty"`
	StorageBytes  *uint64  `json:"storage_bytes,omitempty"`
	StorageCapped bool     `json:"storage_capped,omitempty"`
}

// AppResourcesInput is the app_resources section of a node report.
type AppResourcesInput struct {
	IntervalSeconds float64             `json:"interval_seconds"`
	Series          []AppResourceSeries `json:"series"`
	Notes           []string            `json:"notes,omitempty"`
}

// NodeAppResourcesView is the latest app-resources report for the Console.
type NodeAppResourcesView struct {
	ReportedAt      time.Time           `json:"reported_at"`
	IntervalSeconds float64             `json:"interval_seconds"`
	Series          []AppResourceSeries `json:"series"`
	Notes           []string            `json:"notes"`
}

// leniently reads one number; anything else (a string, a negative, a huge or non-finite value) is "not measured".
func numberField(m map[string]json.RawMessage, key string) *float64 {
	raw, ok := m[key]
	if !ok {
		return nil
	}
	var v float64
	if json.Unmarshal(raw, &v) != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return nil
	}
	return &v
}

func countField(m map[string]json.RawMessage, key string) *uint64 {
	v := numberField(m, key)
	if v == nil {
		return nil
	}
	n := uint64(math.MaxInt64) // SQLite stores signed 64-bit integers; float64(MaxInt64) rounds up past it
	if *v < 1<<63 {
		n = uint64(*v)
	}
	return &n
}

func stringField(m map[string]json.RawMessage, key string) string {
	var v string
	_ = json.Unmarshal(m[key], &v)
	return v
}

// parseAppResources reads the section leniently: whatever cannot be read is dropped (a series) or
// treated as not measured (a field), so an odd section never costs the host report that carries it.
func parseAppResources(raw json.RawMessage) (*AppResourcesInput, bool) {
	var env struct {
		IntervalSeconds json.RawMessage   `json:"interval_seconds"`
		Series          []json.RawMessage `json:"series"`
		Notes           []json.RawMessage `json:"notes"`
	}
	if len(raw) == 0 || string(raw) == "null" || json.Unmarshal(raw, &env) != nil {
		return nil, false
	}
	in := &AppResourcesInput{}
	if env.IntervalSeconds != nil {
		if v := numberField(map[string]json.RawMessage{"v": env.IntervalSeconds}, "v"); v != nil {
			in.IntervalSeconds = *v
		}
	}
	for _, n := range env.Notes {
		var note string
		if json.Unmarshal(n, &note) == nil {
			in.Notes = append(in.Notes, note)
		}
	}
	for _, rs := range env.Series {
		if len(in.Series) >= maxAppResourceSeries {
			break // keep the first ones
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(rs, &m) != nil {
			continue
		}
		s := AppResourceSeries{Kind: stringField(m, "kind"), Key: stringField(m, "key"), Name: stringField(m, "name"),
			CPUPercent: numberField(m, "cpu_percent"), MemoryBytes: countField(m, "memory_bytes"),
			IOReadBps: numberField(m, "io_read_bytes_per_sec"), IOWriteBps: numberField(m, "io_write_bytes_per_sec"),
			NetRxBps: numberField(m, "net_rx_bytes_per_sec"), NetTxBps: numberField(m, "net_tx_bytes_per_sec"),
			StorageBytes: countField(m, "storage_bytes")}
		if p := numberField(m, "pids"); p != nil {
			n := int(math.Min(*p, math.MaxInt32))
			s.Pids = &n
		}
		_ = json.Unmarshal(m["storage_capped"], &s.StorageCapped)
		in.Series = append(in.Series, s)
	}
	return in, true
}

func cleanAppResources(in *AppResourcesInput) []AppResourceSeries {
	seen := map[string]bool{}
	out := make([]AppResourceSeries, 0, len(in.Series))
	for _, s := range in.Series {
		if !appResourceKinds[s.Kind] || s.Key == "" || len(s.Key) > maxAppResourceText || seen[s.Key] {
			continue // one odd series never rejects the report
		}
		seen[s.Key] = true
		s.Name = clipRunes(s.Name, maxAppResourceText)
		out = append(out, s)
	}
	return out
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// StoreAppResources replaces the node's latest measurements with this report. A thing that is no
// longer reported disappears; Core keeps no history.
func (s *NodeService) StoreAppResources(nodeID string, raw json.RawMessage) error {
	in, ok := parseAppResources(raw)
	if !ok {
		return nil
	}
	series := cleanAppResources(in)
	notes := make([]string, 0, len(in.Notes))
	for _, n := range in.Notes {
		if len(notes) < maxAppResourceNotes {
			notes = append(notes, clipRunes(n, maxAppResourceText))
		}
	}
	rawNotes, _ := json.Marshal(notes)
	interval := in.IntervalSeconds
	if math.IsNaN(interval) || math.IsInf(interval, 0) || interval < 0 {
		interval = 0
	}
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		rows := make([]db.NodeAppResource, 0, len(series))
		for _, x := range series {
			rows = append(rows, db.NodeAppResource{NodeID: nodeID, Key: x.Key, Kind: x.Kind, Name: x.Name, CPUPercent: x.CPUPercent,
				MemoryBytes: x.MemoryBytes, IOReadBytesPerSec: x.IOReadBps, IOWriteBytesPerSec: x.IOWriteBps, NetRxBytesPerSec: x.NetRxBps,
				NetTxBytesPerSec: x.NetTxBps, Pids: x.Pids, StorageBytes: x.StorageBytes, StorageCapped: x.StorageCapped, UpdatedAt: now})
		}
		// Replace the whole set: a field that is no longer measured must become absent, not keep its old value.
		if err := tx.Where("node_id = ?", nodeID).Delete(&db.NodeAppResource{}).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).
			Create(&db.NodeAppResourceReport{NodeID: nodeID, IntervalSeconds: interval, Notes: string(rawNotes), ReportedAt: now}).Error
	})
}

// GetAppResources returns the latest report, or nil when the node never sent one.
func (s *NodeService) GetAppResources(nodeID string) (*NodeAppResourcesView, error) {
	var head db.NodeAppResourceReport
	if err := s.db.Where("node_id = ?", nodeID).First(&head).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var rows []db.NodeAppResource
	if err := s.db.Where("node_id = ?", nodeID).Find(&rows).Error; err != nil {
		return nil, err
	}
	view := &NodeAppResourcesView{ReportedAt: head.ReportedAt, IntervalSeconds: head.IntervalSeconds, Series: make([]AppResourceSeries, 0, len(rows)), Notes: []string{}}
	_ = json.Unmarshal([]byte(head.Notes), &view.Notes)
	for _, r := range rows {
		view.Series = append(view.Series, AppResourceSeries{Kind: r.Kind, Key: r.Key, Name: r.Name, CPUPercent: r.CPUPercent, MemoryBytes: r.MemoryBytes,
			IOReadBps: r.IOReadBytesPerSec, IOWriteBps: r.IOWriteBytesPerSec, NetRxBps: r.NetRxBytesPerSec, NetTxBps: r.NetTxBytesPerSec,
			Pids: r.Pids, StorageBytes: r.StorageBytes, StorageCapped: r.StorageCapped})
	}
	sort.Slice(view.Series, func(i, j int) bool {
		if view.Series[i].Kind != view.Series[j].Kind {
			return view.Series[i].Kind < view.Series[j].Kind
		}
		return view.Series[i].Key < view.Series[j].Key
	})
	return view, nil
}
