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

var ErrAppResourcesInvalid = errors.New("invalid app resources")

const (
	maxAppResourceSeries = 200
	maxAppResourceText   = 300
	maxAppResourceNotes  = 20
)

var appResourceKinds = map[string]bool{"process": true, "unit": true, "container": true, "path": true, "volume": true, "docker": true}

// AppResourceSeries is one measured thing. Rates are per second; a field is absent when the
// agent could not measure it (never zero by default).
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

// AppResourcesInput is the app_resources section of an agent report.
type AppResourcesInput struct {
	IntervalSeconds float64             `json:"interval_seconds"`
	Series          []AppResourceSeries `json:"series"`
	Notes           []string            `json:"notes,omitempty"`
}

// AgentAppResourcesView is the latest app-resources report for the Console.
type AgentAppResourcesView struct {
	ReportedAt      time.Time           `json:"reported_at"`
	IntervalSeconds float64             `json:"interval_seconds"`
	Series          []AppResourceSeries `json:"series"`
	Notes           []string            `json:"notes"`
}

func finiteOrNil(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 {
		return nil
	}
	return v
}

func cleanAppResources(in *AppResourcesInput) ([]AppResourceSeries, error) {
	if len(in.Series) > maxAppResourceSeries {
		return nil, ErrAppResourcesInvalid
	}
	seen := map[string]bool{}
	out := make([]AppResourceSeries, 0, len(in.Series))
	for _, s := range in.Series {
		if !appResourceKinds[s.Kind] || s.Key == "" || len(s.Key) > maxAppResourceText || seen[s.Key] {
			continue // one odd series never rejects the report
		}
		seen[s.Key] = true
		s.Name = clipRunes(s.Name, maxAppResourceText)
		s.CPUPercent, s.IOReadBps, s.IOWriteBps = finiteOrNil(s.CPUPercent), finiteOrNil(s.IOReadBps), finiteOrNil(s.IOWriteBps)
		s.NetRxBps, s.NetTxBps = finiteOrNil(s.NetRxBps), finiteOrNil(s.NetTxBps)
		if s.Pids != nil && *s.Pids < 0 {
			s.Pids = nil
		}
		out = append(out, s)
	}
	return out, nil
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// StoreAppResources replaces the agent's latest measurements with this report. A thing that is no
// longer reported disappears; Core keeps no history.
func (s *AgentService) StoreAppResources(agentID string, in *AppResourcesInput) error {
	if in == nil {
		return nil
	}
	series, err := cleanAppResources(in)
	if err != nil {
		return err
	}
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
		rows := make([]db.AgentAppResource, 0, len(series))
		for _, x := range series {
			rows = append(rows, db.AgentAppResource{AgentID: agentID, Key: x.Key, Kind: x.Kind, Name: x.Name, CPUPercent: x.CPUPercent,
				MemoryBytes: x.MemoryBytes, IOReadBytesPerSec: x.IOReadBps, IOWriteBytesPerSec: x.IOWriteBps, NetRxBytesPerSec: x.NetRxBps,
				NetTxBytesPerSec: x.NetTxBps, Pids: x.Pids, StorageBytes: x.StorageBytes, StorageCapped: x.StorageCapped, UpdatedAt: now})
		}
		// Replace the whole set: a field that is no longer measured must become absent, not keep its old value.
		if err := tx.Where("agent_id = ?", agentID).Delete(&db.AgentAppResource{}).Error; err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).
			Create(&db.AgentAppResourceReport{AgentID: agentID, IntervalSeconds: interval, Notes: string(rawNotes), ReportedAt: now}).Error
	})
}

// GetAppResources returns the latest report, or nil when the agent never sent one.
func (s *AgentService) GetAppResources(agentID string) (*AgentAppResourcesView, error) {
	var head db.AgentAppResourceReport
	if err := s.db.Where("agent_id = ?", agentID).First(&head).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var rows []db.AgentAppResource
	if err := s.db.Where("agent_id = ?", agentID).Find(&rows).Error; err != nil {
		return nil, err
	}
	view := &AgentAppResourcesView{ReportedAt: head.ReportedAt, IntervalSeconds: head.IntervalSeconds, Series: make([]AppResourceSeries, 0, len(rows)), Notes: []string{}}
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
