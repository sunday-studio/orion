package service

import (
	"encoding/json"
	"orion/core/internal/db"
	"strings"
)

func (s *IncidentService) impactedComponentsForMonitorIncident(agent db.Agent, monitor db.Monitor, impact string) ([]db.IncidentComponentImpact, error) {
	type componentRow struct {
		ComponentID   string
		ComponentName string
	}

	matchClauses := make([]string, 0, 2)
	matchArgs := make([]any, 0, 4)
	if monitor.ID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "monitor", monitor.ID)
	}
	if agent.ID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "agent", agent.ID)
	}
	if len(matchClauses) == 0 {
		return []db.IncidentComponentImpact{}, nil
	}

	var rows []componentRow
	err := s.db.Table("status_page_components AS components").
		Select("components.id AS component_id, components.public_name AS component_name").
		Joins("JOIN status_page_component_mappings AS mappings ON mappings.component_id = components.id").
		Where("components.visible = ?", true).
		Where(strings.Join(matchClauses, " OR "), matchArgs...).
		Order("components.sort_order ASC, components.public_name ASC, mappings.resource_type ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	status := statusPageComponentImpactStatus(impact)
	seen := map[string]bool{}
	components := make([]db.IncidentComponentImpact, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.ComponentID) == "" || seen[row.ComponentID] {
			continue
		}
		seen[row.ComponentID] = true
		components = append(components, db.IncidentComponentImpact{
			ComponentID:   row.ComponentID,
			ComponentName: row.ComponentName,
			Status:        status,
			Impact:        strings.TrimSpace(impact),
		})
	}
	return cleanIncidentComponentImpacts(components), nil
}

func encodeIncidentComponentImpacts(components []db.IncidentComponentImpact) string {
	cleaned := cleanIncidentComponentImpacts(components)
	if len(cleaned) == 0 {
		return "[]"
	}
	encoded, err := json.Marshal(cleaned)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

func cleanIncidentComponentImpacts(components []db.IncidentComponentImpact) []db.IncidentComponentImpact {
	cleaned := make([]db.IncidentComponentImpact, 0, len(components))
	for _, component := range components {
		component.ComponentID = strings.TrimSpace(component.ComponentID)
		component.ComponentName = strings.TrimSpace(component.ComponentName)
		component.Status = strings.TrimSpace(component.Status)
		component.Impact = strings.TrimSpace(component.Impact)
		if component.ComponentID == "" && component.ComponentName == "" {
			continue
		}
		cleaned = append(cleaned, component)
	}
	return cleaned
}

func statusPageComponentImpactStatus(impact string) string {
	switch strings.TrimSpace(impact) {
	case "up":
		return "operational"
	case "degraded":
		return "degraded"
	case "down", "stale":
		return "major_outage"
	default:
		return "unknown"
	}
}
