package main

import (
	"fmt"
	"sort"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type incidentExtraStats struct {
	candidates, dismissed, notes, responders, declared int
}

// seedIncidentExtras gives the Incidents screens realistic data: human INC numbers, responders,
// timeline notes, an acknowledged and a covered incident, a hand-declared incident, and incident
// candidates (failing monitors with no incident), one of them dismissed.
func seedIncidentExtras(database *gorm.DB, allMonitors []db.Monitor, monitorToAgent map[string]db.Agent, monitorToScenario map[string]scenario, now time.Time) (incidentExtraStats, error) {
	stats := incidentExtraStats{}

	var seeded []db.Incident
	if err := database.Where("id LIKE ?", "seed-incident-%").Order("opened_at ASC, id ASC").Find(&seeded).Error; err != nil {
		return stats, err
	}
	var next int
	if err := database.Raw("SELECT COALESCE(MAX(number), 0) + 1 FROM incidents").Scan(&next).Error; err != nil {
		return stats, err
	}
	for _, incident := range seeded {
		if err := database.Model(&db.Incident{}).Where("id = ?", incident.ID).Update("number", next).Error; err != nil {
			return stats, err
		}
		next++
	}

	events := []db.IncidentEvent{}
	addEvent := func(incidentID, kind, message, actor, note string, at time.Time) {
		events = append(events, db.IncidentEvent{
			ID: fmt.Sprintf("seed-incident-event-extra-%s-%02d", incidentID, len(events)+1), IncidentID: incidentID, Type: kind, Message: message,
			ActorType: choose(actor == "core", "system", "user"), ActorID: actor, Note: note, CreatedAt: at,
		})
	}
	responders := []string{"Dana Whitfield", "Sam Okafor", "Priya Raman"}
	notes := []string{"Upstream pool drained, restarting the load balancer.", "Rolled back the 14:10 deploy; watching error rate.", "Paged the database on-call, disk cleanup in progress."}
	var firstOpen string
	covered := 0
	for i, incident := range seeded {
		switch incident.Status {
		case "open":
			if firstOpen == "" {
				firstOpen = incident.ID
			}
			addEvent(incident.ID, "incident_note", "Note added", "admin", notes[i%len(notes)], incident.OpenedAt.Add(11*time.Minute))
			if i%2 == 0 {
				if err := database.Model(&db.Incident{}).Where("id = ?", incident.ID).Updates(map[string]any{"responder": responders[i%len(responders)], "responder_at": incident.OpenedAt.Add(9 * time.Minute)}).Error; err != nil {
					return stats, err
				}
				addEvent(incident.ID, "incident_responder_changed", "Responder set to "+responders[i%len(responders)], "admin", "", incident.OpenedAt.Add(9*time.Minute))
				stats.responders++
			}
			stats.notes++
		case "acknowledged":
			addEvent(incident.ID, "incident_acknowledged", "Incident manually acknowledged", "admin", "Looking into it.", incident.OpenedAt.Add(4*time.Minute))
			if err := database.Model(&db.Incident{}).Where("id = ?", incident.ID).Updates(map[string]any{"responder": responders[(i+1)%len(responders)], "responder_at": incident.OpenedAt.Add(4 * time.Minute)}).Error; err != nil {
				return stats, err
			}
			stats.responders++
		case "resolved":
			addEvent(incident.ID, "incident_acknowledged", "Incident manually acknowledged", "admin", "", incident.OpenedAt.Add(4*time.Minute))
			addEvent(incident.ID, "incident_note", "Note added", "admin", "Fixed by raising the connection pool limit. Postmortem linked in the tracker.", incident.OpenedAt.Add(20*time.Minute))
			stats.notes++
		}
	}
	// One open incident becomes "covered by" the first open one.
	for _, incident := range seeded {
		if incident.Status == "open" && incident.ID != firstOpen && firstOpen != "" && covered == 0 {
			if err := database.Model(&db.Incident{}).Where("id = ?", incident.ID).Updates(map[string]any{"status": "covered", "covered_at": now.Add(-time.Hour), "covered_by_incident_id": firstOpen, "coverage_note": "Same root cause as the first incident."}).Error; err != nil {
				return stats, err
			}
			addEvent(incident.ID, "incident_covered", "Incident marked covered", "admin", "Same root cause as the first incident.", now.Add(-time.Hour))
			covered++
		}
	}

	// A hand-declared, server-level incident.
	var declaredAgent db.Agent
	for _, m := range allMonitors {
		a := monitorToAgent[m.ID]
		if a.ID != "" && a.ID != "seed-agent-core" && !monitorToScenario[m.ID].noReports {
			declaredAgent = a
			break
		}
	}
	if declaredAgent.ID != "" {
		opened := now.Add(-95 * time.Minute)
		declared := db.Incident{ID: "seed-incident-declared-checkout", Number: next, Status: "open", Severity: "high", Title: "Checkout is slow for EU customers", AgentID: declaredAgent.ID, ImpactedComponents: "[]",
			Summary: "Support reports slow checkout from Frankfurt and Paris since the CDN change.", Source: "manual", Responder: "Dana Whitfield", ResponderAt: ptrTime(opened.Add(2 * time.Minute)),
			OpenedAt: opened, LastEventAt: opened.Add(15 * time.Minute), LatestEvent: "Support reports slow checkout from Frankfurt and Paris since the CDN change.", NotificationStatus: "sent", CreatedAt: opened, UpdatedAt: now}
		if err := database.Create(&declared).Error; err != nil {
			return stats, err
		}
		addEvent(declared.ID, "incident_declared", "Incident declared manually", "admin", "", opened)
		addEvent(declared.ID, "incident_note", "Note added", "admin", "Rolling back the CDN rule for eu-west.", opened.Add(15*time.Minute))
		stats.declared++
		stats.notes++
	}
	if len(events) > 0 {
		if err := database.Create(&events).Error; err != nil {
			return stats, err
		}
	}

	candidateStats, err := seedCandidates(database, allMonitors, monitorToScenario, now)
	stats.candidates, stats.dismissed = candidateStats[0], candidateStats[1]
	return stats, err
}

// seedCandidates makes four healthy monitors start failing right now with no incident, so
// Triage and Health issues have rows; the last one is dismissed.
func seedCandidates(database *gorm.DB, monitors []db.Monitor, scenarioByMonitor map[string]scenario, now time.Time) ([2]int, error) {
	var out [2]int
	var picked []db.Monitor
	for _, m := range monitors {
		if m.Lifecycle != "active" || m.ID == "" || scenarioByMonitor[m.ID].status != "up" || scenarioByMonitor[m.ID].noReports || m.Type == "heartbeat" {
			continue
		}
		picked = append(picked, m)
	}
	sort.Slice(picked, func(i, j int) bool { return picked[i].ID < picked[j].ID })
	errors := []string{"timeout after 10s", "p95 1.8s, limit 1.5s", "connection refused", "expected 200, got 503"}
	for i := 0; i < len(picked) && i < 4; i++ {
		m := picked[i]
		health := choose(i%2 == 0, "down", "degraded")
		for n := 1; n <= 3; n++ {
			at := now.Add(time.Duration(n) * time.Second) // newest reports, after the seeded history
			if err := database.Create(&db.MonitorReport{ID: fmt.Sprintf("seed-monitor-report-candidate-%s-%d", m.ID, n), MonitorID: m.ID,
				Payload: mustJSON(map[string]any{"error": errors[i%len(errors)]}), CollectedAt: at.Format(time.RFC3339), Health: health, CreatedAt: at}).Error; err != nil {
				return out, err
			}
		}
		if err := database.Model(&db.Monitor{}).Where("id = ?", m.ID).Updates(map[string]any{"computed_health": health, "health": health}).Error; err != nil {
			return out, err
		}
		if i == 3 {
			if err := database.Save(&db.IncidentCandidateDismissal{MonitorID: m.ID, DismissedBy: "admin", Reason: "Known flaky upstream", DismissedAt: now.Add(-20 * time.Minute)}).Error; err != nil {
				return out, err
			}
			out[1]++
		} else {
			out[0]++
		}
	}
	return out, nil
}
