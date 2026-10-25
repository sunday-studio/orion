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

func (s *IncidentService) reportOwner(monitor db.Monitor, payload MonitorReportPayload) (db.Node, db.Monitor, error) {
	var node db.Node
	if monitor.NodeID != "" {
		err := s.db.Where("id = ?", monitor.NodeID).First(&node).Error
		if err == nil {
			return node, monitor, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Node{}, monitor, err
		}
		if !isCoreProducedMonitorReport(payload) {
			return db.Node{}, monitor, err
		}
	}

	if !isCoreProducedMonitorReport(payload) {
		return db.Node{}, monitor, gorm.ErrRecordNotFound
	}

	node, err := s.ensureCoreOwnerNode(monitor.NodeID)
	if err != nil {
		return db.Node{}, monitor, err
	}
	if monitor.NodeID != node.ID {
		if err := s.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Update("node_id", node.ID).Error; err != nil {
			return db.Node{}, monitor, err
		}
		monitor.NodeID = node.ID
	}
	return node, monitor, nil
}

func (s *IncidentService) ensureCoreOwnerNode(preferredID string) (db.Node, error) {
	var node db.Node
	if preferredID != "" {
		err := s.db.Where("id = ?", preferredID).First(&node).Error
		if err == nil {
			return node, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Node{}, err
		}
	}

	err := s.db.Where("machine_id = ?", coreOwnerMachineID).First(&node).Error
	if err == nil {
		return node, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Node{}, err
	}

	ownerID := strings.TrimSpace(preferredID)
	if ownerID == "" {
		ownerID = coreOwnerNodeID
	}
	token, err := utils.GenerateToken()
	if err != nil {
		return db.Node{}, err
	}

	now := time.Now().UTC()
	node = db.Node{
		ID:                       ownerID,
		MachineId:                coreOwnerMachineID,
		Name:                     coreOwnerName,
		System:                   true,
		OS:                       "core",
		Arch:                     "internal",
		Token:                    token,
		ReportingIntervalSeconds: 60,
		CreatedAt:                now,
		LastSeen:                 now,
		Meta:                     `{"owner":"core"}`,
	}
	if err := s.db.Create(&node).Error; err != nil {
		var existing db.Node
		if findErr := s.db.Where("machine_id = ? OR id = ?", coreOwnerMachineID, ownerID).First(&existing).Error; findErr == nil {
			return existing, nil
		}
		return db.Node{}, err
	}
	return node, nil
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

func incidentTitle(node db.Node, monitor db.Monitor, health string) string {
	if isCoreOwnerNode(node) {
		return fmt.Sprintf("Core monitor %s: %s", health, monitor.Name)
	}
	return fmt.Sprintf("%s is %s", monitor.Name, health)
}

func incidentMessage(node db.Node, monitor db.Monitor, health string) string {
	if isCoreOwnerNode(node) {
		return fmt.Sprintf("Core monitor %s reported %s", monitor.Name, health)
	}
	return fmt.Sprintf("Monitor %s reported %s", monitor.Name, health)
}

func isCoreOwnerNode(node db.Node) bool {
	return node.MachineId == coreOwnerMachineID
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
