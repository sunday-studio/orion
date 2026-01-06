package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

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

func (s *AgentService) ListAgents(limit int, offset int) ([]db.Agent, error) {
	var agents []db.Agent

	query := s.db.Where("deleted_at IS NULL").Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&agents).Error; err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved agents", "count", len(agents))
	return agents, nil
}

func (s *AgentService) GetAgentCount() (int64, error) {
	var count int64
	if err := s.db.Model(&db.Agent{}).Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		s.logger.Error("Failed to count agents", "error", err)
		return 0, err
	}
	return count, nil
}
