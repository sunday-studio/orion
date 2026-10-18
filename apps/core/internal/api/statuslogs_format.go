package api

import (
	"fmt"
	"orion/core/internal/db"
	"strconv"
	"strings"
	"time"
)

// StatusLogEntry is one thing Orion saw: a check, a state change, a report, an incident step, a delivery or an admin action.
type StatusLogEntry struct {
	ID string    `json:"id"`
	At time.Time `json:"at"`
	// Level is info, warn or error.
	Level string `json:"level" enums:"info,warn,error"`
	// Type is monitor, alert, server, incident or system.
	Type string `json:"type" enums:"monitor,alert,server,incident,system"`
	// Kind is the specific event (check, transition, report, registered, created, delivery, audit, or an incident event type).
	Kind              string `json:"kind"`
	Title             string `json:"title" example:"Delivery failed"`
	Detail            string `json:"detail,omitempty" example:"oncall webhook returned 502, attempt 3 of 3"`
	ServerID          string `json:"server_id,omitempty"`
	ServerName        string `json:"server_name,omitempty"`
	MonitorID         string `json:"monitor_id,omitempty"`
	MonitorName       string `json:"monitor_name,omitempty"`
	IncidentID        string `json:"incident_id,omitempty"`
	IncidentDisplayID string `json:"incident_display_id,omitempty"`
}

func statusEntry(r statusRow) StatusLogEntry {
	e := StatusLogEntry{ID: r.ID, At: time.UnixMilli(r.AtMs).UTC(), Level: r.Level, Type: r.Type, Kind: r.Kind, ServerID: r.ServerID, ServerName: r.ServerName,
		MonitorID: r.MonitorID, MonitorName: r.MonitorName, IncidentID: r.IncidentID, IncidentDisplayID: incidentDisplayID(r.IncNum)}
	switch r.Kind {
	case "check":
		e.Title = map[string]string{"up": "Check passed", "degraded": "Check degraded", "down": "Check failed"}[r.A]
		if e.Title == "" {
			e.Title = "Check reported"
		}
		e.Detail, _, _ = describeMonitorReport(db.MonitorReport{Health: r.A, Payload: r.B})
	case "transition":
		e.Title = map[string]string{"up": "Recovered", "degraded": "Degraded", "down": "Went down"}[r.A]
		if e.Title == "" {
			e.Title = "Changed state"
		}
		e.Detail, _, _ = describeMonitorReport(db.MonitorReport{Health: r.A, Payload: r.B})
	case "report":
		e.Title = "Report accepted"
		e.Detail = joinNonEmpty(", ", percentText("cpu", r.A), percentText("memory", r.B), percentText("disk", r.C))
	case "registered":
		e.Title, e.Detail = "Server registered", r.A
	case "created":
		e.Title, e.Detail = "Monitor created", r.A
	case "delivery":
		e.Title, e.Detail = deliveryText(r)
	case "audit":
		e.Title = auditEventMessage(db.AuditEvent{Action: r.A, MetadataJSON: r.B})
		e.Detail = r.C
	default: // incident events
		e.Title, e.Detail = incidentEventText(r, e.IncidentDisplayID)
	}
	return e
}

func percentText(label, value string) string {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s %d%%", label, int(f+0.5))
}

func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func incidentEventText(r statusRow, display string) (string, string) {
	if display == "" {
		display = "Incident"
	}
	verbs := map[string]string{"incident_opened": "opened", "incident_acknowledged": "acknowledged", "incident_resolved": "resolved", "incident_covered": "marked covered", "incident_reopened": "reopened",
		"incident_declared": "declared", "incident_coverage_expired": "coverage ended", "incident_coverage_suppressed": "coverage suppressed an alert"}
	detail := r.A
	switch {
	case r.Kind == "incident_note":
		return "Note on " + display, firstNonEmpty(r.B, r.A)
	case r.Kind == "incident_responder_changed":
		return "Responder changed on " + display, r.A
	case r.Kind == "incident_public_update":
		return "Public update on " + display, r.A
	case r.Kind == "monitor_failed":
		return "Check failed", firstNonEmpty(r.A, r.D)
	}
	if verb, ok := verbs[r.Kind]; ok {
		if r.Kind == "incident_opened" && r.D != "" {
			detail = r.D
		}
		return display + " " + verb, detail
	}
	return display + " " + strings.ReplaceAll(strings.TrimPrefix(r.Kind, "incident_"), "_", " "), detail
}

func deliveryText(r statusRow) (string, string) {
	channel := firstNonEmpty(r.A, "channel")
	switch r.C {
	case "sent":
		duration := ""
		if r.N4 > 0 {
			duration = fmt.Sprintf("%.1f s", float64(r.N4)/1000)
		}
		return "Delivered to " + channel, joinNonEmpty(", ", r.B, duration)
	case "failed":
		cause := safeAlertDeliveryError(r.D)
		if r.N3 > 0 {
			cause = fmt.Sprintf("returned %d", r.N3)
		} else if r.D != "" {
			cause = "could not be reached"
		}
		return "Delivery failed", fmt.Sprintf("%s %s, attempt %d of %d", channel, cause, max(r.N1, 1), max(r.N2, 1))
	case "suppressed":
		return "Delivery suppressed", joinNonEmpty(", ", channel, safeAlertDeliveryError(r.D))
	case "cooldown":
		return "Delivery held by cooldown", channel
	default:
		return "Waiting to deliver", channel
	}
}
