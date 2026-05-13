package service

import (
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
	MonitorCount  int64   `json:"monitor_count"`
	IP            *string `json:"ip,omitempty"`
	UptimeSeconds *uint64 `json:"uptime_seconds,omitempty"`
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

func NewAgentService(database *gorm.DB, logger *logging.Logger) *AgentService {
	return &AgentService{
		db:     database,
		logger: logger,
	}
}

type RegisterRequest struct {
	MachineId string `json:"machine_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	OS        string `json:"os" binding:"required"`
	Arch      string `json:"arch" binding:"required"`
	Meta      string `json:"meta,omitempty"`
}

type RegisterResponse struct {
	AgentID string `json:"agent_id"`
	Token   string `json:"token"`
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
	// Update metadata if it has changed (OS, arch, name may change after system updates)
	updates := make(map[string]interface{})
	updates["last_seen"] = time.Now()

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

	token, err := utils.GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, err
	}

	agent := db.Agent{
		ID:        agentID,
		MachineId: req.MachineId,
		Name:      req.Name,
		OS:        req.OS,
		Arch:      req.Arch,
		Token:     token,
		Meta:      req.Meta,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	if err := s.db.Create(&agent).Error; err != nil {
		s.logger.Error("Failed to create agent", "error", err)
		return nil, err
	}

	s.logger.Info("New agent registered", "machine_id", req.MachineId, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agentID,
		Token:   agent.Token,
	}, nil
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
	MaintenanceMode bool `json:"maintenance_mode" binding:"required"`
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

	// Over-fetch when status or uptime filter is set (post-filter)
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	fetchLimit := limit
	if opts.Status != "" || opts.Uptime != "" {
		fetchLimit = limit * 5
	}
	query = query.Limit(fetchLimit).Offset(opts.Offset)

	var agents []db.Agent
	if err := query.Find(&agents).Error; err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		return nil, 0, err
	}

	// Base count (same database filters, no status/uptime post-filter)
	var count int64
	countQuery := s.db.Model(&db.Agent{}).Where("deleted_at IS NULL OR deleted_at = ?", time.Time{})
	countQuery = s.applyAgentListDatabaseFilters(countQuery, opts)
	countQuery.Count(&count)

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
		if statusFilter != "" {
			h, _, _, _, err := healthSvc.ComputeAgentHealth(a.ID, cfg)
			if err != nil || h != statusFilter {
				continue
			}
		}
		u := uptimeMap[a.ID]
		if hasUptime && u < uptimeThreshold {
			continue
		}

		row := AgentListRow{Agent: a, MonitorCount: monitorCounts[a.ID]}
		if v, ok := uptimeMap[a.ID]; ok {
			row.UptimeSeconds = &v
		}
		if v, ok := ipMap[a.ID]; ok {
			row.IP = &v
		}
		rows = append(rows, row)
		if len(rows) >= limit {
			break
		}
	}

	return rows, count, nil
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

	if opts.StaleOnly {
		threshold := time.Now().Add(-time.Duration(DefaultHealthConfig().StaleDataThresholdMinutes) * time.Minute)
		query = query.Where("last_seen = ? OR last_seen < ?", time.Time{}, threshold)
	}

	if opts.HasIncidents {
		query = query.Where(
			"id IN (?)",
			s.db.Model(&db.Incident{}).Select("agent_id").Where("status IN ?", []string{"open", "acknowledged"}),
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
