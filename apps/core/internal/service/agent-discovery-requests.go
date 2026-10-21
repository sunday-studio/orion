package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrRequestInvalid       = errors.New("invalid request")
	ErrRequestNotFound      = errors.New("request not found")
	ErrRequestNotCancelable = errors.New("only a request the agent has not picked up can be cancelled")
)

// Request types the Console can send to an agent about discovery.
const (
	ReqRescan          = "rescan"
	ReqDismiss         = "dismiss"
	ReqRestore         = "restore"
	ReqIgnore          = "ignore"
	ReqUnignore        = "unignore"
	ReqSetScanning     = "set_scanning"
	ReqSetKind         = "set_kind"
	ReqTrackPath       = "track_storage"
	ReqCreateLogSource = "create_log_source"
)

// Request states: queued -> delivered -> applied | failed; or expired, unsupported, cancelled.
const (
	ReqQueued      = "queued"
	ReqDelivered   = "delivered"
	ReqApplied     = "applied"
	ReqFailed      = "failed"
	ReqExpired     = "expired"
	ReqUnsupported = "unsupported"
	ReqCancelled   = "cancelled"
)

const (
	requestLifetime      = 24 * time.Hour
	requestRetention     = 30 * 24 * time.Hour
	redeliverAfter       = 5 * time.Minute // a delivered request the agent never finished is offered again
	maxOpenRequests      = 20
	maxRequestIDs        = 100
	maxRequestPaths      = 20
	unsupportedAgentText = "This agent cannot take requests from the Console."
)

var (
	itemIDPattern = regexp.MustCompile(`^[a-z0-9]{2,20}:.{1,280}$`)
	pathPattern   = regexp.MustCompile(`^/[A-Za-z0-9_./ -]{0,250}$`)
	scanKinds     = map[string]bool{"services": true, "containers": true, "pm2": true, "ports": true, "disks": true, "logs": true, "storage": true}
)

// AgentDiscoveryRequestInput is what the Console sends.
type AgentDiscoveryRequestInput struct {
	Type    string         `json:"type" binding:"required" enums:"rescan,dismiss,restore,ignore,unignore,set_scanning,set_kind,track_storage,create_log_source"`
	Payload map[string]any `json:"payload,omitempty"`
}

// AgentDiscoveryRequestView is a request with its outcome.
type AgentDiscoveryRequestView struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Payload     map[string]any `json:"payload"`
	Status      string         `json:"status" enums:"queued,delivered,applied,failed,expired,unsupported,cancelled"`
	Result      string         `json:"result,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	DeliveredAt *time.Time     `json:"delivered_at,omitempty"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

func requestView(r db.AgentDiscoveryRequest) AgentDiscoveryRequestView {
	v := AgentDiscoveryRequestView{ID: r.ID, Type: r.Type, Payload: map[string]any{}, Status: r.Status, Result: r.Result, CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt, DeliveredAt: r.DeliveredAt, FinishedAt: r.FinishedAt, ExpiresAt: r.ExpiresAt}
	_ = json.Unmarshal([]byte(r.Payload), &v.Payload)
	return v
}

func stringList(p map[string]any, key string, max int, ok func(string) bool) ([]string, error) {
	raw, _ := p[key].([]any)
	if len(raw) == 0 || len(raw) > max {
		return nil, fmt.Errorf("%w: %s needs between 1 and %d entries", ErrRequestInvalid, key, max)
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, isStr := v.(string)
		s = strings.TrimSpace(s)
		if !isStr || !ok(s) {
			return nil, fmt.Errorf("%w: %s has an entry that is not valid", ErrRequestInvalid, key)
		}
		out = append(out, s)
	}
	return out, nil
}

// validateRequest checks the payload for the type and returns a cleaned copy.
func validateRequest(in AgentDiscoveryRequestInput) (map[string]any, error) {
	p := map[string]any{}
	switch in.Type {
	case ReqRescan:
	case ReqDismiss, ReqRestore, ReqIgnore, ReqUnignore:
		ids, err := stringList(in.Payload, "ids", maxRequestIDs, itemIDPattern.MatchString)
		if err != nil {
			return nil, err
		}
		p["ids"] = ids
	case ReqSetScanning:
		on, isBool := in.Payload["enabled"].(bool)
		if !isBool {
			return nil, fmt.Errorf("%w: enabled must be true or false", ErrRequestInvalid)
		}
		p["enabled"] = on
	case ReqSetKind:
		kind, _ := in.Payload["kind"].(string)
		on, isBool := in.Payload["enabled"].(bool)
		if !scanKinds[kind] || !isBool {
			return nil, fmt.Errorf("%w: kind must be one of services, containers, pm2, ports, disks, logs, storage and enabled true or false", ErrRequestInvalid)
		}
		p["kind"], p["enabled"] = kind, on
	case ReqTrackPath:
		paths, err := stringList(in.Payload, "paths", maxRequestPaths, func(s string) bool { return pathPattern.MatchString(s) && !strings.Contains(s, "..") })
		if err != nil {
			return nil, err
		}
		p["paths"] = paths
	case ReqCreateLogSource:
		return nil, fmt.Errorf("%w: creating log sources from the Console is not available yet", ErrRequestInvalid)
	default:
		return nil, fmt.Errorf("%w: unknown type %q", ErrRequestInvalid, in.Type)
	}
	return p, nil
}

func (s *AgentService) expireRequests(agentID string, now time.Time) {
	s.db.Model(&db.AgentDiscoveryRequest{}).
		Where("agent_id = ? AND status IN ? AND expires_at < ?", agentID, []string{ReqQueued, ReqDelivered}, now).
		Updates(map[string]any{"status": ReqExpired, "finished_at": now})
	// Finished requests are history for the Console, not an archive.
	s.db.Where("agent_id = ? AND finished_at IS NOT NULL AND finished_at < ?", agentID, now.Add(-requestRetention)).Delete(&db.AgentDiscoveryRequest{})
}

func (s *AgentService) agentCanTakeRequests(agentID string) bool {
	var row db.AgentDiscovery
	if err := s.db.Where("agent_id = ?", agentID).First(&row).Error; err != nil {
		return false
	}
	var caps []string
	_ = json.Unmarshal([]byte(row.Capabilities), &caps)
	for _, c := range caps {
		if c == "requests" {
			return true
		}
	}
	return false
}

// CreateDiscoveryRequest queues a request for the agent. An agent that has not
// said it can take requests gets it stored as unsupported, so the Console can say so.
func (s *AgentService) CreateDiscoveryRequest(agentID string, in AgentDiscoveryRequestInput, actor string) (*AgentDiscoveryRequestView, error) {
	payload, err := validateRequest(in)
	if err != nil {
		return nil, err
	}
	var agent db.Agent
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", agentID, time.Time{}).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	s.expireRequests(agentID, now)
	raw, _ := json.Marshal(payload)
	row := db.AgentDiscoveryRequest{ID: utils.GenerateID("discovery_req"), AgentID: agentID, Type: in.Type, Payload: string(raw), Status: ReqQueued,
		CreatedBy: actor, CreatedAt: now, ExpiresAt: now.Add(requestLifetime)}
	if !s.agentCanTakeRequests(agentID) {
		row.Status, row.Result, row.FinishedAt = ReqUnsupported, unsupportedAgentText, &now
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var open int64
		tx.Model(&db.AgentDiscoveryRequest{}).Where("agent_id = ? AND status IN ?", agentID, []string{ReqQueued, ReqDelivered}).Count(&open)
		if open >= maxOpenRequests {
			return fmt.Errorf("%w: %d requests are already waiting for this agent", ErrRequestInvalid, maxOpenRequests)
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return nil, err
	}
	v := requestView(row)
	return &v, nil
}

// ListDiscoveryRequests returns the agent's most recent requests, newest first.
func (s *AgentService) ListDiscoveryRequests(agentID string, limit int) ([]AgentDiscoveryRequestView, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	s.expireRequests(agentID, time.Now().UTC())
	var rows []db.AgentDiscoveryRequest
	if err := s.db.Where("agent_id = ?", agentID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AgentDiscoveryRequestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, requestView(r))
	}
	return out, nil
}

// CancelDiscoveryRequest withdraws a request the agent has not picked up.
func (s *AgentService) CancelDiscoveryRequest(agentID, requestID string) error {
	now := time.Now().UTC()
	res := s.db.Model(&db.AgentDiscoveryRequest{}).Where("id = ? AND agent_id = ? AND status = ?", requestID, agentID, ReqQueued).
		Updates(map[string]any{"status": ReqCancelled, "finished_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var n int64
		s.db.Model(&db.AgentDiscoveryRequest{}).Where("id = ? AND agent_id = ?", requestID, agentID).Count(&n)
		if n == 0 {
			return ErrRequestNotFound
		}
		return ErrRequestNotCancelable
	}
	return nil
}

// PullDiscoveryRequests hands the agent what is waiting for it. Requests move to delivered;
// one delivered more than redeliverAfter ago and never finished is offered again.
func (s *AgentService) PullDiscoveryRequests(agentID string) ([]AgentDiscoveryRequestView, error) {
	now := time.Now().UTC()
	s.expireRequests(agentID, now)
	var rows []db.AgentDiscoveryRequest
	err := s.db.Where("agent_id = ? AND (status = ? OR (status = ? AND delivered_at < ?))", agentID, ReqQueued, ReqDelivered, now.Add(-redeliverAfter)).
		Order("created_at ASC").Limit(maxOpenRequests).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]AgentDiscoveryRequestView, 0, len(rows))
	for _, r := range rows {
		// Conditional claim: a cancel or a concurrent pull that got here first wins.
		claim := s.db.Model(&db.AgentDiscoveryRequest{}).
			Where("id = ? AND (status = ? OR (status = ? AND delivered_at < ?))", r.ID, ReqQueued, ReqDelivered, now.Add(-redeliverAfter)).
			Updates(map[string]any{"status": ReqDelivered, "delivered_at": now})
		if claim.Error != nil || claim.RowsAffected != 1 {
			continue
		}
		r.Status, r.DeliveredAt = ReqDelivered, &now
		out = append(out, requestView(r))
	}
	return out, nil
}

// FinishDiscoveryRequest records the agent's outcome. result is the agent's own words.
func (s *AgentService) FinishDiscoveryRequest(agentID, requestID, status, result string) error {
	switch status {
	case ReqApplied, ReqFailed, ReqUnsupported:
	default:
		return fmt.Errorf("%w: status must be applied, failed or unsupported", ErrRequestInvalid)
	}
	now := time.Now().UTC()
	res := s.db.Model(&db.AgentDiscoveryRequest{}).Where("id = ? AND agent_id = ? AND status IN ?", requestID, agentID, []string{ReqQueued, ReqDelivered}).
		Updates(map[string]any{"status": status, "result": clipText(result), "finished_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRequestNotFound
	}
	return nil
}
