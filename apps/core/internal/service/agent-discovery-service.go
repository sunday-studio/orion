package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrDiscoveryInvalid = errors.New("invalid discovery report")

const (
	maxDiscoveryItems = 500
	maxDiscoveryKinds = 20
	maxDiscoveryText  = 300
)

// AgentDiscoveryItem is one thing the agent found on its host.
type AgentDiscoveryItem struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Detail     string `json:"detail,omitempty"`
	Confidence string `json:"confidence" enums:"high,medium,low"`
	// Status is suggested (nothing watches it yet) or monitored (a monitor already covers it).
	Status    string `json:"status" enums:"suggested,monitored"`
	ManagedIn string `json:"managed_in,omitempty"`
	New       bool   `json:"new,omitempty"`
	Dismissed bool   `json:"dismissed,omitempty"`
}

// AgentDiscoveryKind says how scanning one kind of thing went on the host.
type AgentDiscoveryKind struct {
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
	Why     string `json:"why,omitempty"`
}

// AgentDiscoveryInput is what an agent reports.
type AgentDiscoveryInput struct {
	ScannedAt time.Time            `json:"scanned_at"`
	Enabled   bool                 `json:"enabled"`
	Kinds     []AgentDiscoveryKind `json:"kinds"`
	Items     []AgentDiscoveryItem `json:"items"`
}

// AgentDiscoveryView is the stored scan with the numbers a panel needs.
type AgentDiscoveryView struct {
	AgentDiscoveryInput
	ReceivedAt time.Time `json:"received_at"`
	Things     int       `json:"things"`
	Suggested  int       `json:"suggested"`
	Monitored  int       `json:"monitored"`
	NewItems   int       `json:"new_items"`
}

func clipText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxDiscoveryText {
		return string(r[:maxDiscoveryText])
	}
	return s
}

func validateDiscovery(in AgentDiscoveryInput) (AgentDiscoveryInput, error) {
	if len(in.Items) > maxDiscoveryItems {
		return in, fmt.Errorf("%w: at most %d items", ErrDiscoveryInvalid, maxDiscoveryItems)
	}
	if len(in.Kinds) > maxDiscoveryKinds {
		return in, fmt.Errorf("%w: at most %d kinds", ErrDiscoveryInvalid, maxDiscoveryKinds)
	}
	for i := range in.Items {
		it := &in.Items[i]
		it.ID, it.Kind, it.Name, it.Detail, it.ManagedIn = clipText(it.ID), clipText(it.Kind), clipText(it.Name), clipText(it.Detail), clipText(it.ManagedIn)
		if it.ID == "" || it.Kind == "" || it.Name == "" {
			return in, fmt.Errorf("%w: every item needs id, kind and name", ErrDiscoveryInvalid)
		}
		switch it.Confidence {
		case "high", "medium", "low":
		default:
			return in, fmt.Errorf("%w: confidence must be high, medium or low", ErrDiscoveryInvalid)
		}
		switch it.Status {
		case "suggested", "monitored":
		default:
			return in, fmt.Errorf("%w: status must be suggested or monitored", ErrDiscoveryInvalid)
		}
	}
	for i := range in.Kinds {
		k := &in.Kinds[i]
		k.Kind, k.State, k.Message, k.Fix, k.Why = clipText(k.Kind), clipText(k.State), clipText(k.Message), clipText(k.Fix), clipText(k.Why)
		if k.Kind == "" || k.State == "" {
			return in, fmt.Errorf("%w: every kind needs kind and state", ErrDiscoveryInvalid)
		}
	}
	if in.ScannedAt.IsZero() || in.ScannedAt.After(time.Now().Add(5*time.Minute)) {
		in.ScannedAt = time.Now().UTC()
	}
	return in, nil
}

// SaveAgentDiscovery replaces the agent's stored scan with the one it just reported.
func (s *AgentService) SaveAgentDiscovery(agentID string, in AgentDiscoveryInput) error {
	in, err := validateDiscovery(in)
	if err != nil {
		return err
	}
	kinds, _ := json.Marshal(in.Kinds)
	items, _ := json.Marshal(in.Items)
	scanned := in.ScannedAt.UTC()
	row := db.AgentDiscovery{AgentID: agentID, ScannedAt: &scanned, Enabled: in.Enabled, Kinds: string(kinds), Items: string(items), ReceivedAt: time.Now().UTC()}
	return s.db.Save(&row).Error
}

// GetAgentDiscovery returns the agent's latest scan, or nil when it has not reported one.
func (s *AgentService) GetAgentDiscovery(agentID string) (*AgentDiscoveryView, error) {
	var row db.AgentDiscovery
	if err := s.db.Where("agent_id = ?", agentID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	v := &AgentDiscoveryView{ReceivedAt: row.ReceivedAt}
	v.Enabled = row.Enabled
	if row.ScannedAt != nil {
		v.ScannedAt = *row.ScannedAt
	}
	_ = json.Unmarshal([]byte(row.Kinds), &v.Kinds)
	_ = json.Unmarshal([]byte(row.Items), &v.Items)
	if v.Kinds == nil {
		v.Kinds = []AgentDiscoveryKind{}
	}
	if v.Items == nil {
		v.Items = []AgentDiscoveryItem{}
	}
	for _, it := range v.Items {
		if it.Dismissed {
			continue
		}
		v.Things++
		switch it.Status {
		case "monitored":
			v.Monitored++
		default:
			v.Suggested++
		}
		if it.New {
			v.NewItems++
		}
	}
	return v, nil
}
