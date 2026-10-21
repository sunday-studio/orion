package service

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"orion/core/internal/db"
	"orion/core/internal/utils"
)

// NodeListRow extends Node with list-only fields from joins/subqueries.
type NodeListRow struct {
	db.Node
	MonitorCount       int64       `json:"monitor_count"`
	IP                 *string     `json:"ip,omitempty"`
	Status             string      `json:"status"`
	AvailabilityHealth string      `json:"availability_health,omitempty"`
	MonitorHealth      string      `json:"monitor_health,omitempty"`
	StatusReason       string      `json:"status_reason,omitempty"`
	UptimeSeconds      *uint64     `json:"uptime_seconds,omitempty"`
	Metrics            NodeMetrics `json:"-"`
	StatusText         string      `json:"-"`
	StatusDetail       string      `json:"-"`
}

// parseListDuration parses "24h", "7d" into a duration. last_seen filter: nodes with last_seen >= now-duration.
func parseListDuration(s string) (time.Duration, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n <= 0 {
			return 0, false
		}
		return time.Duration(n) * 24 * time.Hour, true
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d, true
}

// parseListUptime parses "72h", "3d", "259200" into seconds. Uptime filter: nodes with uptime_seconds >= this.
func parseListUptime(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || n < 0 {
			return 0, false
		}
		return uint64(n) * 24 * 3600, true
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "h"))
		if err != nil || n < 0 {
			return 0, false
		}
		return uint64(n) * 3600, true
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

type NodeService struct {
	db     *gorm.DB
	logger *utils.Logger
}

const (
	NodeTokenStateActive  = "active"
	NodeTokenStateRevoked = "revoked"

	NodeTokenAuditActionRotated  = "node_token_rotated"
	NodeTokenAuditActionRevoked  = "node_token_revoked"
	NodeTokenAuditActionReissued = "node_token_reissued"
)

var (
	ErrNodeTokenRevoked         = errors.New("node_token_revoked")
	ErrNodeTokenReissueRequired = errors.New("node_token_reissue_required")
	ErrNodeTokenNotRevoked      = errors.New("node_token_not_revoked")
)

func NewNodeService(database *gorm.DB, logger *utils.Logger) *NodeService {
	return &NodeService{
		db:     database,
		logger: logger,
	}
}

type RegisterRequest struct {
	MachineId                string `json:"machine_id" binding:"required"`
	Name                     string `json:"name" binding:"required"`
	OS                       string `json:"os" binding:"required"`
	Arch                     string `json:"arch" binding:"required"`
	ReportingIntervalSeconds int    `json:"reporting_interval_seconds,omitempty"`
	Meta                     string `json:"meta,omitempty"`
}

type RegisterResponse struct {
	NodeID string `json:"node_id"`
	Token  string `json:"token"`
}

func (s *NodeService) RegisterNode(req *RegisterRequest) (*RegisterResponse, error) {
	var node db.Node

	if err := s.db.Where("machine_id = ?", req.MachineId).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return s.createNewNode(req)
		}
		s.logger.Error("Database error during node lookup", "error", err)
		return nil, err
	}

	// Node exists - handle reconnection/re-registration
	if node.TokenRevokedAt != nil {
		return nil, ErrNodeTokenRevoked
	}
	// Every node token is stored hashed, so Core cannot hand a token back on
	// re-registration; the operator reissues one (rotate/reissue endpoints).
	return nil, ErrNodeTokenReissueRequired
}

func (s *NodeService) createNewNode(req *RegisterRequest) (*RegisterResponse, error) {
	nodeID := utils.GenerateID("node")
	now := time.Now().UTC()

	token, err := utils.GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, err
	}

	node := db.Node{
		ID:                       nodeID,
		MachineId:                req.MachineId,
		Name:                     req.Name,
		OS:                       req.OS,
		Arch:                     req.Arch,
		Token:                    storedNodeTokenMarker(token),
		TokenHash:                hashNodeToken(token),
		TokenVersion:             1,
		TokenRotatedAt:           &now,
		ReportingIntervalSeconds: req.ReportingIntervalSeconds,
		Meta:                     req.Meta,
		CreatedAt:                now,
		LastSeen:                 now,
	}
	if node.ReportingIntervalSeconds <= 0 {
		node.ReportingIntervalSeconds = 60
	}

	if err := s.db.Create(&node).Error; err != nil {
		s.logger.Error("Failed to create node", "error", err)
		return nil, err
	}

	s.logger.Info("New node registered", "machine_id", req.MachineId, "node_id", node.ID, "name", node.Name)

	return &RegisterResponse{
		NodeID: nodeID,
		Token:  token,
	}, nil
}

func (s *NodeService) NodeTokenStatus(nodeID string) (*NodeTokenStatus, error) {
	var node db.Node
	if err := s.db.Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, err
	}
	status := nodeTokenStatus(node)
	return &status, nil
}

func (s *NodeService) UpdateLastSeen(nodeID string) error {
	if err := s.db.Model(&db.Node{}).Where("id = ?", nodeID).Update("last_seen", time.Now()).Error; err != nil {
		s.logger.Error("Failed to update last_seen", "node_id", nodeID, "error", err)
		return err
	}

	s.logger.Debug("Updated last_seen timestamp", "node_id", nodeID)
	return nil
}

type SetMaintenanceModeRequest struct {
	MaintenanceMode *bool `json:"maintenance_mode" binding:"required"`
	// DurationSeconds is how long maintenance lasts when enabling. Omitted means
	// DefaultMaintenanceDuration; values above MaxMaintenanceDuration are rejected.
	DurationSeconds *int `json:"duration_seconds,omitempty"`
}

const (
	DefaultMaintenanceDuration = 24 * time.Hour
	MaxMaintenanceDuration     = 30 * 24 * time.Hour
)

var ErrInvalidMaintenanceDuration = errors.New("duration_seconds must be between 1 and 2592000 (30 days)")

// SetMaintenanceMode enables maintenance until now+duration (default 24h) or
// disables it, and records an audit event so a silenced node is always explainable.
func (s *NodeService) SetMaintenanceMode(nodeID string, maintenanceMode bool, duration time.Duration, actor NodeTokenActionInput) error {
	now := time.Now().UTC()
	updates := map[string]any{"maintenance_mode": maintenanceMode, "maintenance_until": nil}
	action := NodeMaintenanceAuditDisabled
	metadata := map[string]any{}
	if maintenanceMode {
		if duration <= 0 {
			duration = DefaultMaintenanceDuration
		}
		if duration > MaxMaintenanceDuration {
			return ErrInvalidMaintenanceDuration
		}
		until := now.Add(duration)
		updates["maintenance_until"] = &until
		action = NodeMaintenanceAuditEnabled
		metadata["until"] = until.Format(time.RFC3339)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&db.Node{}).Where("id = ?", nodeID).Updates(updates)
		if result.Error != nil {
			s.logger.Error("Failed to set maintenance mode", "node_id", nodeID, "maintenance_mode", maintenanceMode, "error", result.Error)
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		s.logger.Info("Maintenance mode updated", "node_id", nodeID, "maintenance_mode", maintenanceMode)
		return recordMaintenanceAuditEvent(tx, action, nodeID, actor, metadata)
	})
}

// ExpireMaintenance ends every maintenance window whose end time has passed so
// alerting resumes, writing an audit event for each. It returns how many ended.
func (s *NodeService) ExpireMaintenance(now time.Time) (int, error) {
	var expired []db.Node
	if err := s.db.Where("maintenance_mode = ? AND maintenance_until IS NOT NULL AND maintenance_until <= ?", true, now.UTC()).Find(&expired).Error; err != nil {
		return 0, err
	}
	for _, node := range expired {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			result := tx.Model(&db.Node{}).
				Where("id = ? AND maintenance_mode = ? AND maintenance_until <= ?", node.ID, true, now.UTC()).
				Updates(map[string]any{"maintenance_mode": false, "maintenance_until": nil})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			return recordMaintenanceAuditEvent(tx, NodeMaintenanceAuditExpired, node.ID, NodeTokenActionInput{ActorType: "system", ActorID: "core"}, map[string]any{"ended_at": now.UTC().Format(time.RFC3339)})
		})
		if err != nil {
			return 0, err
		}
		s.logger.Info("Maintenance window expired; alerting resumed", "node_id", node.ID)
	}
	return len(expired), nil
}

func (s *NodeService) GetNode(nodeID string) (*db.Node, error) {
	var node db.Node
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
		s.logger.Error("Failed to get node", "node_id", nodeID, "error", err)
		return nil, err
	}
	return &node, nil
}

// ListNodesOpts holds query params for ListNodes.
type ListNodesOpts struct {
	Limit         int
	Offset        int
	Search        string // node name, machine_id, meta, or monitor name (LIKE)
	Status        string // up, down, degraded, unknown; applied as post-filter
	Maintenance   string // true, false, or empty
	StaleOnly     bool
	HasIncidents  bool
	IncludeSystem bool   // include Core's own node (Settings, System)
	LastSeen      string // e.g. 24h, 7d: last_seen >= now-duration
	Uptime        string // e.g. 72h, 3d: uptime_seconds >= parsed
	Sort          string // name, last_seen, created_at
	Order         string // asc, desc
}

func (s *NodeService) ListNodes(opts ListNodesOpts) ([]NodeListRow, int64, error) {
	query := s.db.Model(&db.Node{}).Where("deleted_at IS NULL OR deleted_at = ?", time.Time{})
	if !opts.IncludeSystem {
		query = query.Where("system = ?", false)
	}
	query = s.applyNodeListDatabaseFilters(query, opts)

	// Sort
	sortCol := "last_seen"
	switch opts.Sort {
	case "name":
		sortCol = "name"
	case "created_at":
		sortCol = "created_at"
	case "last_seen", "":
		sortCol = "last_seen"
	}
	order := "DESC"
	if strings.ToLower(opts.Order) == "asc" {
		order = "ASC"
	}
	query = query.Order(sortCol + " " + order)

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	var nodes []db.Node
	if err := query.Find(&nodes).Error; err != nil {
		s.logger.Error("Failed to list nodes", "error", err)
		return nil, 0, err
	}

	ids := make([]string, 0, len(nodes))
	for _, a := range nodes {
		ids = append(ids, a.ID)
	}

	// monitor_count per node
	monitorCounts := make(map[string]int64)
	if len(ids) > 0 {
		var counts []struct {
			NodeID string
			C      int64
		}
		s.db.Model(&db.Monitor{}).Where("node_id IN ? AND lifecycle = ?", ids, "active").Select("node_id, COUNT(*) as c").Group("node_id").Find(&counts)
		for _, c := range counts {
			monitorCounts[c.NodeID] = c.C
		}
	}

	// Latest report per node: uptime_seconds, Location.IP (N+1 acceptable for now)
	uptimeMap := make(map[string]uint64)
	ipMap := make(map[string]string)
	metricsMap := make(map[string]NodeMetrics)
	for _, id := range ids {
		var reports []db.NodeReport
		if err := s.db.Where("node_id = ?", id).Order("created_at DESC").Limit(nodeSeriesPoints).Find(&reports).Error; err == nil && len(reports) > 0 {
			m := metricsFromReports(reports)
			metricsMap[id] = m
			uptimeMap[id] = reports[0].UptimeSeconds
			if m.IP != "" {
				ipMap[id] = m.IP
			}
		}
	}

	// Status filter: compute health per node, keep only matching
	statusFilter := strings.ToLower(strings.TrimSpace(opts.Status))
	uptimeThreshold, hasUptime := parseListUptime(opts.Uptime)

	healthSvc := NewHealthService(s.db, s.logger)
	cfg := DefaultHealthConfig()

	rows := make([]NodeListRow, 0, len(nodes))
	for _, a := range nodes {
		snapshot := NodeHealthSnapshot{
			OverallHealth: "unknown",
			NodeHealth:    "unknown",
			MonitorHealth: "unknown",
			Reason:        "health has not been computed yet",
		}
		if computed, err := healthSvc.ComputeNodeHealthSnapshot(a.ID, cfg); err == nil {
			snapshot = computed
		}
		if opts.StaleOnly && snapshot.OverallHealth != "stale" {
			continue
		}
		if statusFilter != "" {
			if snapshot.OverallHealth != statusFilter {
				continue
			}
		}
		u := uptimeMap[a.ID]
		if hasUptime && u < uptimeThreshold {
			continue
		}

		row := NodeListRow{
			Node:               a,
			MonitorCount:       monitorCounts[a.ID],
			Status:             snapshot.OverallHealth,
			AvailabilityHealth: snapshot.NodeHealth,
			MonitorHealth:      snapshot.MonitorHealth,
			StatusReason:       snapshot.Reason,
			Metrics:            metricsMap[a.ID],
		}
		row.StatusText = NodeStatusText(snapshot.OverallHealth, a.LastSeen, row.Metrics, time.Now().UTC())
		row.StatusDetail = NodeStatusDetail(snapshot.OverallHealth, row.Metrics, snapshot.Reason)
		if v, ok := uptimeMap[a.ID]; ok {
			row.UptimeSeconds = &v
		}
		if v, ok := ipMap[a.ID]; ok {
			row.IP = &v
		}
		rows = append(rows, row)
	}

	count := int64(len(rows))
	if offset >= len(rows) {
		return []NodeListRow{}, count, nil
	}

	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}

	return rows[offset:end], count, nil
}

func (s *NodeService) applyNodeListDatabaseFilters(query *gorm.DB, opts ListNodesOpts) *gorm.DB {
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where(
			"name LIKE ? OR machine_id LIKE ? OR meta LIKE ? OR id IN (?)",
			like,
			like,
			like,
			s.db.Model(&db.Monitor{}).Select("node_id").Where("name LIKE ? AND lifecycle = ?", like, "active"),
		)
	}

	switch strings.ToLower(strings.TrimSpace(opts.Maintenance)) {
	case "true":
		query = query.Where("maintenance_mode = ?", true)
	case "false":
		query = query.Where("maintenance_mode = ?", false)
	}

	if opts.HasIncidents {
		query = query.Where(
			"id IN (?)",
			s.db.Model(&db.Incident{}).Select("node_id").Where("status IN ?", activeIncidentStatuses()),
		)
	}

	if opts.LastSeen != "" {
		if d, ok := parseListDuration(opts.LastSeen); ok {
			query = query.Where("last_seen >= ?", time.Now().Add(-d))
		}
	}

	return query
}
