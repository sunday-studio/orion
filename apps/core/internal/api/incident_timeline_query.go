package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"sort"
	"strconv"
	"strings"
)

func incidentTimeline(events []db.IncidentEvent, deliveries []db.AlertDelivery, reports []db.MonitorReport) []IncidentTimelineItemResponse {
	timeline := make([]IncidentTimelineItemResponse, 0, len(events)+len(deliveries))
	evidenceByReportID := incidentTimelineEvidenceByReportID(reports)
	for _, event := range events {
		timeline = append(timeline, IncidentTimelineItemResponse{ID: event.ID, Type: event.Type, Source: "incident_event", Message: event.Message, Evidence: evidenceByReportID[event.MonitorReportID], MonitorReportID: event.MonitorReportID, ActorType: event.ActorType, ActorID: event.ActorID, Note: event.Note, CreatedAt: event.CreatedAt})
	}
	for _, delivery := range deliveries {
		message := delivery.Channel + " notification " + delivery.Status
		if delivery.Error != "" {
			message += ": " + safeAlertDeliveryError(delivery.Error)
		}
		timeline = append(timeline, IncidentTimelineItemResponse{ID: delivery.ID, Type: "alert_delivery", Source: "alert_delivery", Message: message, AlertDeliveryID: delivery.ID, Channel: delivery.Channel, Status: delivery.Status, CreatedAt: delivery.CreatedAt})
	}
	sort.SliceStable(timeline, func(i, j int) bool {
		return timeline[i].CreatedAt.Before(timeline[j].CreatedAt)
	})
	return timeline
}
func incidentTimelineEvidenceByReportID(reports []db.MonitorReport) map[string]string {
	evidenceByReportID := make(map[string]string, len(reports))
	for _, report := range reports {
		evidence := monitorReportEvidence(report)
		if evidence != "" {
			evidenceByReportID[report.ID] = evidence
		}
	}
	return evidenceByReportID
}
func monitorReportEvidence(report db.MonitorReport) string {
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(service.SafeMonitorReportPayload(report.Payload)), &fields); err != nil {
		return ""
	}
	for _, key := range []string{"payload", "failure_stage", "failure_reason", "message", "error", "summary", "status", "status_code"} {
		if evidence := monitorReportEvidenceValue(fields[key]); evidence != "" {
			return evidence
		}
	}
	return ""
}
func monitorReportEvidenceValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case map[string]interface{}, []interface{}:
		body, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(body)
	default:
		return ""
	}
}
