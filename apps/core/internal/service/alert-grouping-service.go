package service

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"
)

func (s *AlertService) groupingDecision(event AlertRouteContext, policy string, delay time.Duration) (alertGroupingDecision, error) {
	if policy == db.AlertGroupingPolicyNone {
		return alertGroupingDecision{Policy: policy}, nil
	}
	if event.EventType != db.AlertEventIncidentOpened && event.EventType != db.AlertEventIncidentResolved {
		return alertGroupingDecision{}, nil
	}

	var incident db.Incident
	if err := s.db.Where("id = ?", event.IncidentID).First(&incident).Error; err != nil {
		return alertGroupingDecision{}, err
	}

	switch event.EventType {
	case db.AlertEventIncidentOpened:
		return s.openIncidentGroupingDecision(event, incident, policy, delay)
	case db.AlertEventIncidentResolved:
		return s.resolveIncidentGroupingDecision(event, incident, policy)
	default:
		return alertGroupingDecision{}, nil
	}
}

func (s *AlertService) openIncidentGroupingDecision(event AlertRouteContext, incident db.Incident, policy string, delay time.Duration) (alertGroupingDecision, error) {
	existingGroup, found, err := s.alertGroupForIncident(incident.ID)
	if err != nil {
		return alertGroupingDecision{}, err
	}
	if found {
		return alertGroupingDecision{GroupID: existingGroup.ID, Policy: policy}, nil
	}

	now := time.Now().UTC()
	groupKey := alertGroupKey(event)
	var group db.AlertGroup
	result := s.db.Where("group_key = ? AND status = ?", groupKey, "open").
		Order("last_event_at DESC").
		Limit(1).
		Find(&group)
	if result.Error != nil {
		return alertGroupingDecision{}, result.Error
	}

	if result.RowsAffected == 0 {
		group = db.AlertGroup{
			ID:              utils.GenerateID("alert_group"),
			GroupKey:        groupKey,
			Status:          "open",
			EventType:       event.EventType,
			Severity:        incident.Severity,
			Summary:         alertGroupSummary(event, 1),
			FirstIncidentID: incident.ID,
			LastIncidentID:  incident.ID,
			IncidentCount:   1,
			FirstEventAt:    now,
			LastEventAt:     now,
		}
		if err := s.db.Create(&group).Error; err != nil {
			return alertGroupingDecision{}, err
		}
		if err := s.createAlertGroupMember(group.ID, incident.ID); err != nil {
			return alertGroupingDecision{}, err
		}
		return alertGroupingDecision{GroupID: group.ID, Policy: policy}, nil
	}

	if err := s.createAlertGroupMember(group.ID, incident.ID); err != nil {
		return alertGroupingDecision{}, err
	}
	nextCount := group.IncidentCount + 1
	updates := map[string]any{
		"last_incident_id": incident.ID,
		"incident_count":   nextCount,
		"last_event_at":    now,
		"summary":          alertGroupSummary(event, nextCount),
	}
	if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(updates).Error; err != nil {
		return alertGroupingDecision{}, err
	}

	decision := alertGroupingDecision{
		GroupID: group.ID,
		Policy:  policy,
	}
	if policy == db.AlertGroupingPolicyDelayedSummary {
		decision.SummaryDue = true
		decision.SummaryDelay = delay
		decision.Reason = alertGroupedSummaryPending
		return decision, nil
	}
	decision.Suppress = true
	decision.Reason = "alert grouped into active alert group"
	return decision, nil
}

func (s *AlertService) resolveIncidentGroupingDecision(event AlertRouteContext, incident db.Incident, policy string) (alertGroupingDecision, error) {
	group, found, err := s.alertGroupForIncident(incident.ID)
	if err != nil || !found {
		return alertGroupingDecision{}, err
	}

	now := time.Now().UTC()
	activeCount, err := s.activeAlertGroupIncidentCount(group.ID)
	if err != nil {
		return alertGroupingDecision{}, err
	}
	if activeCount > 0 {
		if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
			"last_event_at": now,
			"summary":       alertGroupSummary(event, group.IncidentCount),
		}).Error; err != nil {
			return alertGroupingDecision{}, err
		}
		return alertGroupingDecision{
			GroupID:  group.ID,
			Policy:   policy,
			Suppress: true,
			Reason:   "alert grouped; sibling incidents still active",
		}, nil
	}

	if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(map[string]any{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"summary":       fmt.Sprintf("All %d grouped incidents resolved", group.IncidentCount),
	}).Error; err != nil {
		return alertGroupingDecision{}, err
	}
	return alertGroupingDecision{GroupID: group.ID, Policy: policy}, nil
}

func (s *AlertService) alertGroupForIncident(incidentID string) (db.AlertGroup, bool, error) {
	var member db.AlertGroupMember
	result := s.db.Where("incident_id = ?", incidentID).Order("created_at DESC").Limit(1).Find(&member)
	if result.Error != nil {
		return db.AlertGroup{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return db.AlertGroup{}, false, nil
	}

	var group db.AlertGroup
	if err := s.db.Where("id = ?", member.AlertGroupID).First(&group).Error; err != nil {
		return db.AlertGroup{}, false, err
	}
	return group, true, nil
}

func (s *AlertService) createAlertGroupMember(groupID string, incidentID string) error {
	member := db.AlertGroupMember{
		ID:           utils.GenerateID("alert_group_member"),
		AlertGroupID: groupID,
		IncidentID:   incidentID,
	}
	return s.db.Create(&member).Error
}

func (s *AlertService) activeAlertGroupIncidentCount(groupID string) (int64, error) {
	var count int64
	err := s.db.Table("alert_group_members").
		Joins("JOIN incidents ON incidents.id = alert_group_members.incident_id").
		Where("alert_group_members.alert_group_id = ? AND incidents.status IN ?", groupID, activeIncidentStatuses()).
		Count(&count).Error
	return count, err
}

func (s *AlertService) groupingPolicyForEvent(event AlertRouteContext, routes []db.AlertRoute) (string, time.Duration) {
	if len(routes) == 0 {
		return db.AlertGroupingPolicySuppress, time.Duration(db.DefaultAlertGroupingDelaySeconds) * time.Second
	}
	for _, route := range routes {
		matched, _ := routeMatchesEvent(route, event)
		if !matched || !route.Enabled {
			continue
		}
		return normalizeAlertGroupingPolicy(route.GroupingPolicy), time.Duration(normalizeAlertGroupingDelaySeconds(route.GroupingDelaySeconds)) * time.Second
	}
	return db.AlertGroupingPolicySuppress, time.Duration(db.DefaultAlertGroupingDelaySeconds) * time.Second
}

func normalizeAlertGroupingPolicy(policy string) string {
	switch strings.TrimSpace(policy) {
	case "", db.AlertGroupingPolicySuppress:
		return db.AlertGroupingPolicySuppress
	case db.AlertGroupingPolicyDelayedSummary:
		return db.AlertGroupingPolicyDelayedSummary
	case db.AlertGroupingPolicyNone:
		return db.AlertGroupingPolicyNone
	default:
		return db.AlertGroupingPolicySuppress
	}
}

func normalizeAlertGroupingDelaySeconds(value int) int {
	if value <= 0 {
		return db.DefaultAlertGroupingDelaySeconds
	}
	return value
}

func alertGroupKey(event AlertRouteContext) string {
	monitorType := strings.TrimSpace(event.MonitorType)
	if monitorType == "" {
		monitorType = "unknown"
	}
	return fmt.Sprintf("agent:%s|monitor_type:%s|severity:%s", event.AgentID, monitorType, event.Severity)
}

func alertGroupSummary(event AlertRouteContext, count int) string {
	monitorType := strings.TrimSpace(event.MonitorType)
	if monitorType == "" {
		monitorType = "unknown monitor"
	}
	return fmt.Sprintf("%d %s %s incident(s) on agent %s", count, event.Severity, monitorType, event.AgentID)
}
