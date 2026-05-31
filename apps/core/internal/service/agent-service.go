package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
)

// AgentListRow extends Agent with list-only fields from joins/subqueries.
type AgentListRow struct {
	db.Agent
	MonitorCount       int64   `json:"monitor_count"`
	IP                 *string `json:"ip,omitempty"`
	Status             string  `json:"status"`
	AvailabilityHealth string  `json:"availability_health,omitempty"`
	MonitorHealth      string  `json:"monitor_health,omitempty"`
	StatusReason       string  `json:"status_reason,omitempty"`
	UptimeSeconds      *uint64 `json:"uptime_seconds,omitempty"`
}

// parseListDuration parses "24h", "7d" into a duration. last_seen filter: agents with last_seen >= now-duration.
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

// parseListUptime parses "72h", "3d", "259200" into seconds. Uptime filter: agents with uptime_seconds >= this.
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

type AgentService struct {
	db     *gorm.DB
	logger *logging.Logger
}

const (
	AgentTokenStateActive  = "active"
	AgentTokenStateRevoked = "revoked"

	AgentTokenAuditActionRotated  = "agent_token_rotated"
	AgentTokenAuditActionRevoked  = "agent_token_revoked"
	AgentTokenAuditActionReissued = "agent_token_reissued"
)

var (
	ErrAgentTokenRevoked         = errors.New("agent_token_revoked")
	ErrAgentTokenReissueRequired = errors.New("agent_token_reissue_required")
	ErrAgentTokenNotRevoked      = errors.New("agent_token_not_revoked")
)

func NewAgentService(database *gorm.DB, logger *logging.Logger) *AgentService {
	return &AgentService{
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
	AgentID string `json:"agent_id"`
	Token   string `json:"token"`
}

type AgentTokenStatus struct {
	AgentID               string     `json:"agent_id"`
	State                 string     `json:"state"`
	TokenVersion          int        `json:"token_version"`
	TokenRotatedAt        *time.Time `json:"token_rotated_at,omitempty"`
	TokenRevokedAt        *time.Time `json:"token_revoked_at,omitempty"`
	TokenRevocationReason string     `json:"token_revocation_reason,omitempty"`
	TokenExists           bool       `json:"token_exists"`
}

type AgentTokenActionInput struct {
	ActorType string
	ActorID   string
	Reason    string
	RequestID string
}

type AgentTokenIssueResult struct {
	Token  string
	Status AgentTokenStatus
}

func (s *AgentService) RegisterAgent(req *RegisterRequest) (*RegisterResponse, error) {
	var agent db.Agent

	if err := s.db.Where("machine_id = ?", req.MachineId).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return s.createNewAgent(req)
		}
		s.logger.Error("Database error during agent lookup", "error", err)
		return nil, err
	}

	// Agent exists - handle reconnection/re-registration
	if agent.TokenRevokedAt != nil {
		return nil, ErrAgentTokenRevoked
	}
	if agent.TokenHash != "" {
		return nil, ErrAgentTokenReissueRequired
	}

	// Update metadata if it has changed (OS, arch, name may change after system updates)
	updates := make(map[string]any)
	updates["last_seen"] = time.Now()
	if req.ReportingIntervalSeconds > 0 && agent.ReportingIntervalSeconds != req.ReportingIntervalSeconds {
		updates["reporting_interval_seconds"] = req.ReportingIntervalSeconds
	}

	if agent.Name != req.Name {
		s.logger.Info("Agent name changed", "old_name", agent.Name, "new_name", req.Name, "agent_id", agent.ID)
		updates["name"] = req.Name
	}

	if agent.OS != req.OS {
		s.logger.Info("Agent OS changed", "old_os", agent.OS, "new_os", req.OS, "agent_id", agent.ID)
		updates["os"] = req.OS
	}

	if agent.Arch != req.Arch {
		s.logger.Info("Agent arch changed", "old_arch", agent.Arch, "new_arch", req.Arch, "agent_id", agent.ID)
		updates["arch"] = req.Arch
	}

	if req.Meta != "" {
		updates["meta"] = req.Meta
	}

	// Only update if there are changes
	if len(updates) > 0 {
		if err := s.db.Model(&agent).Updates(updates).Error; err != nil {
			s.logger.Error("Failed to update agent on reconnection", "error", err, "agent_id", agent.ID)
			// Don't fail registration if update fails, just log it
		} else {
			s.logger.Info("Agent metadata updated on reconnection", "agent_id", agent.ID, "updates", updates)
		}
	}

	s.logger.Info("Agent reconnected", "machine_id", req.MachineId, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agent.ID,
		Token:   agent.Token,
	}, nil
}

func (s *AgentService) createNewAgent(req *RegisterRequest) (*RegisterResponse, error) {
	agentID := utils.GenerateID("agent")
	now := time.Now().UTC()

	token, err := utils.GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, err
	}

	agent := db.Agent{
		ID:                       agentID,
		MachineId:                req.MachineId,
		Name:                     req.Name,
		OS:                       req.OS,
		Arch:                     req.Arch,
		Token:                    storedAgentTokenMarker(token),
		TokenHash:                hashAgentToken(token),
		TokenVersion:             1,
		TokenRotatedAt:           &now,
		ReportingIntervalSeconds: req.ReportingIntervalSeconds,
		Meta:                     req.Meta,
		CreatedAt:                now,
		LastSeen:                 now,
	}
	if agent.ReportingIntervalSeconds <= 0 {
		agent.ReportingIntervalSeconds = 60
	}

	if err := s.db.Create(&agent).Error; err != nil {
		s.logger.Error("Failed to create agent", "error", err)
		return nil, err
	}

	s.logger.Info("New agent registered", "machine_id", req.MachineId, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agentID,
		Token:   token,
	}, nil
}

func (s *AgentService) ValidateAgentToken(agentID string, token string) (*db.Agent, error) {
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Invalid token for missing agent", "agent_id", agentID)
			return nil, err
		}
		s.logger.Error("Database error during token validation", "error", err)
		return nil, err
	}
	if agent.TokenRevokedAt != nil {
		s.logger.Warn("Rejected revoked token for agent", "agent_id", agentID)
		return nil, ErrAgentTokenRevoked
	}
	if !agentTokenMatches(agent, token) {
		s.logger.Warn("Invalid token for agent", "agent_id", agentID)
		return nil, gorm.ErrRecordNotFound
	}

	s.logger.Debug("Token validated successfully", "agent_id", agentID, "agent_name", agent.Name)
	return &agent, nil
}

func (s *AgentService) AgentTokenStatus(agentID string) (*AgentTokenStatus, error) {
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return nil, err
	}
	status := agentTokenStatus(agent)
	return &status, nil
}

func (s *AgentService) RotateAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenIssueResult, error) {
	return s.issueAgentToken(agentID, input, AgentTokenAuditActionRotated, false)
}

func (s *AgentService) ReissueAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenIssueResult, error) {
	return s.issueAgentToken(agentID, input, AgentTokenAuditActionReissued, true)
}

func (s *AgentService) RevokeAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenStatus, error) {
	now := time.Now().UTC()
	reason := sanitizeAgentTokenReason(input.Reason)

	var status AgentTokenStatus
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var agent db.Agent
		if err := tx.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return err
		}
		if agent.TokenRevokedAt != nil {
			status = agentTokenStatus(agent)
			return nil
		}

		updates := map[string]any{
			"token":                   revokedAgentTokenMarker(agent.ID, agent.TokenVersion+1),
			"token_hash":              "",
			"token_version":           normalizedAgentTokenVersion(agent.TokenVersion) + 1,
			"token_revoked_at":        now,
			"token_revocation_reason": reason,
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return err
		}
		var updatedAgent db.Agent
		if err := tx.Where("id = ?", agent.ID).First(&updatedAgent).Error; err != nil {
			return err
		}
		status = agentTokenStatus(updatedAgent)
		return recordAgentTokenAuditEvent(tx, AgentTokenAuditActionRevoked, agent.ID, input, status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *AgentService) issueAgentToken(agentID string, input AgentTokenActionInput, auditAction string, requireRevoked bool) (*AgentTokenIssueResult, error) {
	token, err := utils.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	var result AgentTokenIssueResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var agent db.Agent
		if err := tx.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return err
		}
		if requireRevoked && agent.TokenRevokedAt == nil {
			return ErrAgentTokenNotRevoked
		}
		if !requireRevoked && agent.TokenRevokedAt != nil {
			return ErrAgentTokenRevoked
		}

		nextVersion := normalizedAgentTokenVersion(agent.TokenVersion) + 1
		updates := map[string]any{
			"token":                   storedAgentTokenMarker(token),
			"token_hash":              hashAgentToken(token),
			"token_version":           nextVersion,
			"token_rotated_at":        now,
			"token_revocation_reason": "",
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).UpdateColumn("token_revoked_at", gorm.Expr("NULL")).Error; err != nil {
			return err
		}
		var updatedAgent db.Agent
		if err := tx.Where("id = ?", agent.ID).First(&updatedAgent).Error; err != nil {
			return err
		}
		result = AgentTokenIssueResult{
			Token:  token,
			Status: agentTokenStatus(updatedAgent),
		}
		return recordAgentTokenAuditEvent(tx, auditAction, updatedAgent.ID, input, result.Status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func agentTokenMatches(agent db.Agent, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if agent.TokenHash != "" {
		return subtle.ConstantTimeCompare([]byte(agent.TokenHash), []byte(hashAgentToken(token))) == 1
	}
	return subtle.ConstantTimeCompare([]byte(agent.Token), []byte(token)) == 1
}

func agentTokenStatus(agent db.Agent) AgentTokenStatus {
	state := AgentTokenStateActive
	tokenExists := agent.TokenHash != "" || (agent.Token != "" && !strings.HasPrefix(agent.Token, "revoked:"))
	if agent.TokenRevokedAt != nil {
		state = AgentTokenStateRevoked
		tokenExists = false
	}
	return AgentTokenStatus{
		AgentID:               agent.ID,
		State:                 state,
		TokenVersion:          normalizedAgentTokenVersion(agent.TokenVersion),
		TokenRotatedAt:        agent.TokenRotatedAt,
		TokenRevokedAt:        agent.TokenRevokedAt,
		TokenRevocationReason: agent.TokenRevocationReason,
		TokenExists:           tokenExists,
	}
}

func normalizedAgentTokenVersion(version int) int {
	if version <= 0 {
		return 1
	}
	return version
}

func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func storedAgentTokenMarker(token string) string {
	return "sha256:" + hashAgentToken(token)
}

func revokedAgentTokenMarker(agentID string, version int) string {
	return fmt.Sprintf("revoked:%s:%d", agentID, version)
}

func sanitizeAgentTokenReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return reason
}

func recordAgentTokenAuditEvent(tx *gorm.DB, action string, agentID string, input AgentTokenActionInput, tokenVersion int) error {
	actorType := strings.TrimSpace(input.ActorType)
	if actorType == "" {
		actorType = "user"
	}
	actorID := strings.TrimSpace(input.ActorID)
	if actorID == "" {
		actorID = "admin"
	}
	metadata, err := json.Marshal(map[string]any{
		"token_version": tokenVersion,
		"reason":        sanitizeAgentTokenReason(input.Reason),
		"request_id":    strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return err
	}
	event := db.AuditEvent{
		ID:                 utils.GenerateID("audit_event"),
		Action:             action,
		StatusPageID:       "",
		AffectedObjectType: "agent",
		AffectedObjectID:   agentID,
		ActorType:          actorType,
		ActorID:            actorID,
		MetadataJSON:       string(metadata),
		CreatedAt:          time.Now().UTC(),
	}
	return tx.Create(&event).Error
}

func (s *AgentService) UpdateLastSeen(agentID string) error {
	if err := s.db.Model(&db.Agent{}).Where("id = ?", agentID).Update("last_seen", time.Now()).Error; err != nil {
		s.logger.Error("Failed to update last_seen", "agent_id", agentID, "error", err)
		return err
	}

	s.logger.Debug("Updated last_seen timestamp", "agent_id", agentID)
	return nil
}

type SetMaintenanceModeRequest struct {
	MaintenanceMode *bool `json:"maintenance_mode" binding:"required"`
}

func (s *AgentService) SetMaintenanceMode(agentID string, maintenanceMode bool) error {
	if err := s.db.Model(&db.Agent{}).Where("id = ?", agentID).Update("maintenance_mode", maintenanceMode).Error; err != nil {
		s.logger.Error("Failed to set maintenance mode", "agent_id", agentID, "maintenance_mode", maintenanceMode, "error", err)
		return err
	}

	s.logger.Info("Maintenance mode updated", "agent_id", agentID, "maintenance_mode", maintenanceMode)
	return nil
}

func (s *AgentService) GetAgent(agentID string) (*db.Agent, error) {
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		s.logger.Error("Failed to get agent", "agent_id", agentID, "error", err)
		return nil, err
	}
	return &agent, nil
}

// ListAgentsOpts holds query params for ListAgents.
type ListAgentsOpts struct {
	Limit        int
	Offset       int
	Search       string // agent name, machine_id, meta, or monitor name (LIKE)
	Status       string // up, down, degraded, unknown; applied as post-filter
	Maintenance  string // true, false, or empty
	StaleOnly    bool
	HasIncidents bool
	LastSeen     string // e.g. 24h, 7d: last_seen >= now-duration
	Uptime       string // e.g. 72h, 3d: uptime_seconds >= parsed
	Sort         string // name, last_seen, created_at
	Order        string // asc, desc
}

func (s *AgentService) ListAgents(opts ListAgentsOpts) ([]AgentListRow, int64, error) {
	query := s.db.Model(&db.Agent{}).Where("deleted_at IS NULL OR deleted_at = ?", time.Time{})
	query = s.applyAgentListDatabaseFilters(query, opts)

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

	var agents []db.Agent
	if err := query.Find(&agents).Error; err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		return nil, 0, err
	}

	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		ids = append(ids, a.ID)
	}

	// monitor_count per agent
	monitorCounts := make(map[string]int64)
	if len(ids) > 0 {
		var counts []struct {
			AgentID string
			C       int64
		}
		s.db.Model(&db.Monitor{}).Where("agent_id IN ? AND lifecycle = ?", ids, "active").Select("agent_id, COUNT(*) as c").Group("agent_id").Find(&counts)
		for _, c := range counts {
			monitorCounts[c.AgentID] = c.C
		}
	}

	// Latest report per agent: uptime_seconds, Location.IP (N+1 acceptable for now)
	uptimeMap := make(map[string]uint64)
	ipMap := make(map[string]string)
	for _, id := range ids {
		var r db.AgentReport
		if err := s.db.Where("agent_id = ?", id).Order("created_at DESC").Limit(1).First(&r).Error; err == nil {
			uptimeMap[id] = r.UptimeSeconds
			if loc := r.Location.Data(); loc.IP != "" {
				ipMap[id] = loc.IP
			}
		}
	}

	// Status filter: compute health per agent, keep only matching
	statusFilter := strings.ToLower(strings.TrimSpace(opts.Status))
	uptimeThreshold, hasUptime := parseListUptime(opts.Uptime)

	healthSvc := NewHealthService(s.db, s.logger)
	cfg := DefaultHealthConfig()

	rows := make([]AgentListRow, 0, len(agents))
	for _, a := range agents {
		snapshot := AgentHealthSnapshot{
			OverallHealth: "unknown",
			AgentHealth:   "unknown",
			MonitorHealth: "unknown",
			Reason:        "health has not been computed yet",
		}
		if computed, err := healthSvc.ComputeAgentHealthSnapshot(a.ID, cfg); err == nil {
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

		row := AgentListRow{
			Agent:              a,
			MonitorCount:       monitorCounts[a.ID],
			Status:             snapshot.OverallHealth,
			AvailabilityHealth: snapshot.AgentHealth,
			MonitorHealth:      snapshot.MonitorHealth,
			StatusReason:       snapshot.Reason,
		}
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
		return []AgentListRow{}, count, nil
	}

	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}

	return rows[offset:end], count, nil
}

func (s *AgentService) applyAgentListDatabaseFilters(query *gorm.DB, opts ListAgentsOpts) *gorm.DB {
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where(
			"name LIKE ? OR machine_id LIKE ? OR meta LIKE ? OR id IN (?)",
			like,
			like,
			like,
			s.db.Model(&db.Monitor{}).Select("agent_id").Where("name LIKE ? AND lifecycle = ?", like, "active"),
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
			s.db.Model(&db.Incident{}).Select("agent_id").Where("status IN ?", activeIncidentStatuses()),
		)
	}

	if opts.LastSeen != "" {
		if d, ok := parseListDuration(opts.LastSeen); ok {
			query = query.Where("last_seen >= ?", time.Now().Add(-d))
		}
	}

	return query
}

func (s *AgentService) GetAgentCount() (int64, error) {
	var count int64
	if err := s.db.Model(&db.Agent{}).Where("deleted_at IS NULL OR deleted_at = ?", time.Time{}).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count agents", "error", err)
		return 0, err
	}
	return count, nil
}
