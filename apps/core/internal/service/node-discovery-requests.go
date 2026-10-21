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
	ErrRequestNotCancelable = errors.New("only a request the node has not picked up can be cancelled")
)

// Request types the Console can send to a node about discovery.
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
	requestLifetime     = 24 * time.Hour
	requestRetention    = 30 * 24 * time.Hour
	redeliverAfter      = 5 * time.Minute // a delivered request the node never finished is offered again
	maxOpenRequests     = 20
	maxRequestIDs       = 100
	maxRequestPaths     = 20
	unsupportedNodeText = "This node cannot take requests from the Console."
)

var (
	itemIDPattern = regexp.MustCompile(`^[a-z0-9]{2,20}:.{1,280}$`)
	pathPattern   = regexp.MustCompile(`^/[A-Za-z0-9_./ -]{0,250}$`)
	scanKinds     = map[string]bool{"services": true, "containers": true, "pm2": true, "ports": true, "disks": true, "logs": true, "storage": true}
)

// NodeDiscoveryRequestInput is what the Console sends.
type NodeDiscoveryRequestInput struct {
	Type    string         `json:"type" binding:"required" enums:"rescan,dismiss,restore,ignore,unignore,set_scanning,set_kind,track_storage,create_log_source"`
	Payload map[string]any `json:"payload,omitempty"`
}

// NodeDiscoveryRequestView is a request with its outcome.
type NodeDiscoveryRequestView struct {
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

func requestView(r db.NodeDiscoveryRequest) NodeDiscoveryRequestView {
	v := NodeDiscoveryRequestView{ID: r.ID, Type: r.Type, Payload: map[string]any{}, Status: r.Status, Result: r.Result, CreatedBy: r.CreatedBy,
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
func validateRequest(in NodeDiscoveryRequestInput) (map[string]any, error) {
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

func (s *NodeService) expireRequests(nodeID string, now time.Time) {
	s.db.Model(&db.NodeDiscoveryRequest{}).
		Where("node_id = ? AND status IN ? AND expires_at < ?", nodeID, []string{ReqQueued, ReqDelivered}, now).
		Updates(map[string]any{"status": ReqExpired, "finished_at": now})
	// Finished requests are history for the Console, not an archive.
	s.db.Where("node_id = ? AND finished_at IS NOT NULL AND finished_at < ?", nodeID, now.Add(-requestRetention)).Delete(&db.NodeDiscoveryRequest{})
}

func (s *NodeService) nodeCanTakeRequests(nodeID string) bool {
	var row db.NodeDiscovery
	if err := s.db.Where("node_id = ?", nodeID).First(&row).Error; err != nil {
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

// CreateDiscoveryRequest queues a request for the node. A node that has not
// said it can take requests gets it stored as unsupported, so the Console can say so.
func (s *NodeService) CreateDiscoveryRequest(nodeID string, in NodeDiscoveryRequestInput, actor string) (*NodeDiscoveryRequestView, error) {
	payload, err := validateRequest(in)
	if err != nil {
		return nil, err
	}
	var node db.Node
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	now := time.Now().UTC()
	s.expireRequests(nodeID, now)
	raw, _ := json.Marshal(payload)
	row := db.NodeDiscoveryRequest{ID: utils.GenerateID("discovery_req"), NodeID: nodeID, Type: in.Type, Payload: string(raw), Status: ReqQueued,
		CreatedBy: actor, CreatedAt: now, ExpiresAt: now.Add(requestLifetime)}
	if !s.nodeCanTakeRequests(nodeID) {
		row.Status, row.Result, row.FinishedAt = ReqUnsupported, unsupportedNodeText, &now
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var open int64
		tx.Model(&db.NodeDiscoveryRequest{}).Where("node_id = ? AND status IN ?", nodeID, []string{ReqQueued, ReqDelivered}).Count(&open)
		if open >= maxOpenRequests {
			return fmt.Errorf("%w: %d requests are already waiting for this node", ErrRequestInvalid, maxOpenRequests)
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		return nil, err
	}
	v := requestView(row)
	return &v, nil
}

// ListDiscoveryRequests returns the node's most recent requests, newest first.
func (s *NodeService) ListDiscoveryRequests(nodeID string, limit int) ([]NodeDiscoveryRequestView, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	s.expireRequests(nodeID, time.Now().UTC())
	var rows []db.NodeDiscoveryRequest
	if err := s.db.Where("node_id = ?", nodeID).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]NodeDiscoveryRequestView, 0, len(rows))
	for _, r := range rows {
		out = append(out, requestView(r))
	}
	return out, nil
}

// CancelDiscoveryRequest withdraws a request the node has not picked up.
func (s *NodeService) CancelDiscoveryRequest(nodeID, requestID string) error {
	now := time.Now().UTC()
	res := s.db.Model(&db.NodeDiscoveryRequest{}).Where("id = ? AND node_id = ? AND status = ?", requestID, nodeID, ReqQueued).
		Updates(map[string]any{"status": ReqCancelled, "finished_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var n int64
		s.db.Model(&db.NodeDiscoveryRequest{}).Where("id = ? AND node_id = ?", requestID, nodeID).Count(&n)
		if n == 0 {
			return ErrRequestNotFound
		}
		return ErrRequestNotCancelable
	}
	return nil
}

// PullDiscoveryRequests hands the node what is waiting for it. Requests move to delivered;
// one delivered more than redeliverAfter ago and never finished is offered again.
func (s *NodeService) PullDiscoveryRequests(nodeID string) ([]NodeDiscoveryRequestView, error) {
	now := time.Now().UTC()
	s.expireRequests(nodeID, now)
	var rows []db.NodeDiscoveryRequest
	err := s.db.Where("node_id = ? AND (status = ? OR (status = ? AND delivered_at < ?))", nodeID, ReqQueued, ReqDelivered, now.Add(-redeliverAfter)).
		Order("created_at ASC").Limit(maxOpenRequests).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]NodeDiscoveryRequestView, 0, len(rows))
	for _, r := range rows {
		// Conditional claim: a cancel or a concurrent pull that got here first wins.
		claim := s.db.Model(&db.NodeDiscoveryRequest{}).
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

// FinishDiscoveryRequest records the node's outcome. result is the node's own words.
func (s *NodeService) FinishDiscoveryRequest(nodeID, requestID, status, result string) error {
	switch status {
	case ReqApplied, ReqFailed, ReqUnsupported:
	default:
		return fmt.Errorf("%w: status must be applied, failed or unsupported", ErrRequestInvalid)
	}
	now := time.Now().UTC()
	res := s.db.Model(&db.NodeDiscoveryRequest{}).Where("id = ? AND node_id = ? AND status IN ?", requestID, nodeID, []string{ReqQueued, ReqDelivered}).
		Updates(map[string]any{"status": status, "result": clipText(result), "finished_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRequestNotFound
	}
	return nil
}
