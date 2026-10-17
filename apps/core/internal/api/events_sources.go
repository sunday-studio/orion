package api

import (
	"encoding/base64"
	"errors"
	"orion/core/internal/db"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	maxOrionEventsLimit  = 200
	maxOrionEventsWindow = 5000
	orionEventBatch      = 200
	orionEventMaxBatches = 25
)

// orionEventCursor is the keyset position: events are ordered (created_at, row id) descending
// and a page starts strictly after the cursor.
type orionEventCursor struct {
	At time.Time
	ID string
}

func encodeOrionEventCursor(e OrionEventResponse) string {
	return base64.RawURLEncoding.EncodeToString([]byte(e.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + e.rowID))
}

func decodeOrionEventCursor(raw string) (orionEventCursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return orionEventCursor{}, err
	}
	at, id, ok := strings.Cut(string(data), "|")
	if !ok {
		return orionEventCursor{}, errors.New("malformed cursor")
	}
	parsed, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return orionEventCursor{}, err
	}
	return orionEventCursor{At: parsed.UTC(), ID: id}, nil
}

func keysetBefore(q *gorm.DB, cursor *orionEventCursor) *gorm.DB {
	if cursor == nil {
		return q
	}
	return q.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.At, cursor.At, cursor.ID)
}

func orionEventAfter(a, b OrionEventResponse) int { // newest first
	if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
		return c
	}
	return strings.Compare(b.rowID, a.rowID)
}

// orionEventFetcher returns up to n events strictly older than the cursor, newest first, unfiltered.
type orionEventFetcher func(cursor *orionEventCursor, n int) ([]OrionEventResponse, error)

// orionEventsPage merges every source into one newest-first list of at most `need` events
// that match the filters and come after the cursor. Each source is read in bounded keyset
// batches, so the cost depends on the page size, not on table size.
func (s *Server) orionEventsPage(cursor *orionEventCursor, need int, filters orionEventFilters) ([]OrionEventResponse, error) {
	var merged []OrionEventResponse
	for _, fetch := range s.orionEventFetchers(filters.Source) {
		position := cursor
		var kept []OrionEventResponse
		for batch := 0; batch < orionEventMaxBatches && len(kept) < need; batch++ {
			rows, err := fetch(position, orionEventBatch)
			if err != nil {
				return nil, err
			}
			kept = append(kept, filterOrionEvents(rows, filters)...)
			if len(rows) < orionEventBatch {
				break
			}
			last := rows[len(rows)-1]
			position = &orionEventCursor{At: last.CreatedAt.UTC(), ID: last.rowID}
		}
		if len(kept) > need {
			kept = kept[:need]
		}
		merged = append(merged, kept...)
	}
	slices.SortStableFunc(merged, orionEventAfter)
	if len(merged) > need {
		merged = merged[:need]
	}
	return merged, nil
}

func (s *Server) orionEventFetchers(source string) []orionEventFetcher {
	byName := map[string]orionEventFetcher{
		"agent":          s.fetchAgentEvents,
		"monitor":        s.fetchMonitorEvents,
		"agent_report":   s.fetchAgentReportEvents,
		"monitor_report": s.fetchMonitorReportEvents,
		"incident_event": s.fetchIncidentEvents,
		"alert_delivery": s.fetchDeliveryEvents,
	}
	source = strings.ToLower(strings.TrimSpace(source))
	if source == "" {
		return []orionEventFetcher{s.fetchAgentEvents, s.fetchMonitorEvents, s.fetchAgentReportEvents, s.fetchMonitorReportEvents,
			s.fetchIncidentEvents, s.fetchDeliveryEvents, s.fetchAuditEvents, s.fetchLifecycleSettingsEvents}
	}
	if fetch, ok := byName[source]; ok {
		return []orionEventFetcher{fetch}
	}
	// audit-derived sources (audit, data_lifecycle, <object>_lifecycle)
	return []orionEventFetcher{s.fetchAuditEvents, s.fetchLifecycleSettingsEvents}
}

func (s *Server) fetchAgentEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.Agent
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, a := range rows {
		out = append(out, OrionEventResponse{ID: a.ID + ":registered", Type: "agent_registered", Source: "agent", Message: "Agent " + a.Name + " registered", AgentID: a.ID, CreatedAt: a.CreatedAt, rowID: a.ID})
	}
	return out, nil
}

func (s *Server) fetchMonitorEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.Monitor
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, OrionEventResponse{ID: m.ID + ":registered", Type: "monitor_registered", Source: "monitor", Message: "Monitor " + m.Name + " registered", AgentID: m.AgentID, MonitorID: m.ID, CreatedAt: m.CreatedAt, rowID: m.ID})
	}
	return out, nil
}

func (s *Server) fetchAgentReportEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.AgentReport
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrionEventResponse{ID: r.ID, Type: "agent_report_received", Source: "agent_report", Message: "Agent report received", AgentID: r.AgentID, CreatedAt: r.CreatedAt, rowID: r.ID})
	}
	return out, nil
}

func (s *Server) fetchMonitorReportEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.MonitorReport
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, OrionEventResponse{ID: r.ID, Type: "monitor_report_received", Source: "monitor_report", Message: "Monitor report received with " + r.Health + " health", MonitorID: r.MonitorID, CreatedAt: r.CreatedAt, rowID: r.ID})
	}
	return out, nil
}

func (s *Server) fetchIncidentEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.IncidentEvent
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, e := range rows {
		out = append(out, OrionEventResponse{ID: e.ID, Type: e.Type, Source: "incident_event", Message: e.Message, IncidentID: e.IncidentID, CreatedAt: e.CreatedAt, rowID: e.ID})
	}
	return out, nil
}

func (s *Server) fetchDeliveryEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.AlertDelivery
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, OrionEventResponse{ID: d.ID, Type: "alert_" + d.Status, Source: "alert_delivery", Message: d.Channel + " notification " + d.Status, IncidentID: d.IncidentID, CreatedAt: d.CreatedAt, rowID: d.ID})
	}
	return out, nil
}

func (s *Server) fetchAuditEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var rows []db.AuditEvent
	if err := keysetBefore(s.db, cursor).Order("created_at DESC, id DESC").Limit(n).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]OrionEventResponse, 0, len(rows))
	for _, event := range rows {
		source := "audit"
		if event.AffectedObjectType == "data_lifecycle" {
			source = "data_lifecycle"
		} else if event.AffectedObjectType != "" {
			source = event.AffectedObjectType + "_lifecycle"
		}
		out = append(out, OrionEventResponse{ID: event.ID, Type: event.Action, Source: source, Message: auditEventMessage(event), AgentID: auditEventAgentID(event), CreatedAt: event.CreatedAt, rowID: event.ID})
	}
	return out, nil
}

// fetchLifecycleSettingsEvents adds the two synthetic "last run" events kept on the settings row.
func (s *Server) fetchLifecycleSettingsEvents(cursor *orionEventCursor, n int) ([]OrionEventResponse, error) {
	var settings db.DataLifecycleSettings
	if err := s.db.First(&settings, 1).Error; err != nil {
		return nil, nil
	}
	var out []OrionEventResponse
	add := func(at *time.Time, kind, id, message string) {
		if at == nil {
			return
		}
		row := "data-lifecycle:" + id + ":" + at.Format("20060102150405")
		if cursor != nil && (at.After(cursor.At) || (at.Equal(cursor.At) && row >= cursor.ID)) {
			return
		}
		out = append(out, OrionEventResponse{ID: row, Type: kind, Source: "data_lifecycle", Message: message, CreatedAt: *at, rowID: row})
	}
	add(settings.LastRollupRunAt, "retention_rollup_ran", "rollup", "Data lifecycle rollup ran")
	message := "Data lifecycle archive ran"
	if settings.LastArchiveStatus != "" {
		message += " with " + settings.LastArchiveStatus + " status"
	}
	add(settings.LastArchiveRunAt, "retention_archive_ran", "archive", message)
	slices.SortStableFunc(out, orionEventAfter)
	return out, nil
}
