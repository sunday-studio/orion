package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *IncidentService) reportOwner(monitor db.Monitor, payload MonitorReportPayload) (db.Agent, db.Monitor, error) {
	var agent db.Agent
	if monitor.AgentID != "" {
		err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error
		if err == nil {
			return agent, monitor, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Agent{}, monitor, err
		}
		if !isCoreProducedMonitorReport(payload) {
			return db.Agent{}, monitor, err
		}
	}

	if !isCoreProducedMonitorReport(payload) {
		return db.Agent{}, monitor, gorm.ErrRecordNotFound
	}

	agent, err := s.ensureCoreOwnerAgent(monitor.AgentID)
	if err != nil {
		return db.Agent{}, monitor, err
	}
	if monitor.AgentID != agent.ID {
		if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Update("agent_id", agent.ID).Error; err != nil {
			return db.Agent{}, monitor, err
		}
		monitor.AgentID = agent.ID
	}
	return agent, monitor, nil
}

func (s *IncidentService) ensureCoreOwnerAgent(preferredID string) (db.Agent, error) {
	var agent db.Agent
	if preferredID != "" {
		err := s.db.Where("id = ?", preferredID).First(&agent).Error
		if err == nil {
			return agent, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Agent{}, err
		}
	}

	err := s.db.Where("machine_id = ?", coreOwnerMachineID).First(&agent).Error
	if err == nil {
		return agent, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Agent{}, err
	}

	ownerID := strings.TrimSpace(preferredID)
	if ownerID == "" {
		ownerID = coreOwnerAgentID
	}
	token, err := utils.GenerateToken()
	if err != nil {
		return db.Agent{}, err
	}

	now := time.Now().UTC()
	agent = db.Agent{
		ID:                       ownerID,
		MachineId:                coreOwnerMachineID,
		Name:                     coreOwnerName,
		OS:                       "core",
		Arch:                     "internal",
		Token:                    token,
		ReportingIntervalSeconds: 60,
		CreatedAt:                now,
		LastSeen:                 now,
		Meta:                     `{"owner":"core"}`,
	}
	if err := s.db.Create(&agent).Error; err != nil {
		var existing db.Agent
		if findErr := s.db.Where("machine_id = ? OR id = ?", coreOwnerMachineID, ownerID).First(&existing).Error; findErr == nil {
			return existing, nil
		}
		return db.Agent{}, err
	}
	return agent, nil
}

func isCoreProducedMonitorReport(payload MonitorReportPayload) bool {
	return mapFieldIsCore(payload.Metrics, "runner") ||
		mapFieldIsCore(payload.Metrics, "source") ||
		mapFieldIsCore(payload.Error, "runner") ||
		mapFieldIsCore(payload.Error, "source")
}

func mapFieldIsCore(value any, key string) bool {
	fields, ok := value.(map[string]any)
	if !ok {
		return false
	}
	raw, ok := fields[key]
	if !ok {
		return false
	}
	text, ok := raw.(string)
	return ok && isCoreValue(text)
}

func isCoreValue(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), coreMonitorRunner)
}

func incidentTitle(agent db.Agent, monitor db.Monitor, health string) string {
	if isCoreOwnerAgent(agent) {
		return fmt.Sprintf("Core monitor %s: %s", health, monitor.Name)
	}
	return fmt.Sprintf("%s is %s", monitor.Name, health)
}

func incidentMessage(agent db.Agent, monitor db.Monitor, health string) string {
	if isCoreOwnerAgent(agent) {
		return fmt.Sprintf("Core monitor %s reported %s", monitor.Name, health)
	}
	return fmt.Sprintf("Monitor %s reported %s", monitor.Name, health)
}

func isCoreOwnerAgent(agent db.Agent) bool {
	return agent.MachineId == coreOwnerMachineID
}

func activeIncidentStatuses() []string {
	return []string{"open", "acknowledged", "covered"}
}

func (s *IncidentService) reopenedIncidentState(monitorID string) string {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		return "unknown"
	}
	health := monitor.ComputedHealth
	if health == "" || health == "unknown" {
		health = monitor.Health
	}
	switch health {
	case "down", "degraded", "stale":
		return health
	default:
		return "unknown"
	}
}
