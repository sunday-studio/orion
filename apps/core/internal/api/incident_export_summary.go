package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxIncidentExportRows = 10000

// exportIncidentsCSV downloads the filtered incident list.
// @Summary      Export incidents as CSV
// @Description  Same filters as GET /v1/incidents (status, severity, q, opened_since, ...), no paging, at most 10,000 rows, newest first.
// @Tags         incidents
// @Produce      text/csv
// @ID           exportIncidentsCSV
// @Param        status        query  string  false  "Comma-separated statuses" default(open,acknowledged,covered)
// @Param        severity      query  string  false  "Comma-separated severities"
// @Param        q             query  string  false  "Text search"
// @Param        opened_since  query  string  false  "24h, 7d, 30d or RFC3339"
// @Success      200  {file}  file
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/incidents/export [get]
func (s *Server) exportIncidentsCSV(c *gin.Context) {
	filters, err := incidentFiltersFromQuery(c)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	var incidents []db.Incident
	if err := s.incidentListQuery(filters).Order("opened_at DESC").Limit(maxIncidentExportRows).Find(&incidents).Error; err != nil {
		utils.InternalError(c, "Failed to export incidents", err)
		return
	}
	nodes, monitors := map[string]string{}, map[string]string{}
	var nodeRows []db.Node
	s.db.Select("id", "name").Find(&nodeRows)
	for _, a := range nodeRows {
		nodes[a.ID] = a.Name
	}
	var monitorRows []db.Monitor
	s.db.Select("id", "name").Find(&monitorRows)
	for _, m := range monitorRows {
		monitors[m.ID] = m.Name
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="incidents.csv"`)
	c.Header("Cache-Control", "no-store")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"id", "title", "status", "severity", "monitor", "server", "opened_at", "resolved_at", "duration_seconds", "responder", "source"})
	now := time.Now().UTC()
	for _, i := range incidents {
		end := now
		resolved := ""
		if i.ResolvedAt != nil {
			end, resolved = *i.ResolvedAt, i.ResolvedAt.UTC().Format(time.RFC3339)
		}
		_ = w.Write([]string{csvSafe(incidentDisplayID(i.Number)), csvSafe(i.Title), i.Status, i.Severity, csvSafe(monitors[i.MonitorID]), csvSafe(nodes[i.NodeID]),
			i.OpenedAt.UTC().Format(time.RFC3339), resolved, strconv.FormatInt(int64(end.Sub(i.OpenedAt).Seconds()), 10), csvSafe(i.Responder), i.Source})
	}
	w.Flush()
}

// csvSafe stops spreadsheet formula injection from titles typed by users or taken from monitors.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}

// IncidentDetailSummary is everything the detail screen shows beside the timeline.
type IncidentDetailSummary struct {
	DetectedAt               time.Time               `json:"detected_at"`
	NotifiedAt               *time.Time              `json:"notified_at"`
	AcknowledgedAt           *time.Time              `json:"acknowledged_at"`
	ResolvedAt               *time.Time              `json:"resolved_at"`
	TimeToAcknowledgeSeconds *int64                  `json:"time_to_acknowledge_seconds"`
	TimeToResolveSeconds     *int64                  `json:"time_to_resolve_seconds"`
	DurationSeconds          int64                   `json:"duration_seconds"`
	FailedChecks             int                     `json:"failed_checks"`
	NotifiedChannels         []string                `json:"notified_channels"`
	Source                   IncidentDetailSource    `json:"source"`
	Expected                 *IncidentExpectation    `json:"expected,omitempty"`
	Actual                   *IncidentExpectation    `json:"actual,omitempty"`
	StatusPage               IncidentStatusPageState `json:"status_page"`
	CoveredByDisplayID       string                  `json:"covered_by_display_id,omitempty"`
}

type IncidentDetailSource struct {
	MonitorID   string `json:"monitor_id,omitempty"`
	MonitorName string `json:"monitor_name,omitempty"`
	MonitorType string `json:"monitor_type,omitempty"`
	// Target is what the check hits, for example "GET /health" or "db.internal:5432".
	Target     string `json:"target,omitempty"`
	ServerID   string `json:"server_id,omitempty"`
	ServerName string `json:"server_name,omitempty"`
}

// IncidentExpectation is one side of the "Expected vs Actual" box.
type IncidentExpectation struct {
	Title  string     `json:"title" example:"200 OK"`
	Detail string     `json:"detail,omitempty" example:"under 10s"`
	At     *time.Time `json:"at,omitempty"`
}

type IncidentStatusPageState struct {
	// DraftExists is true when a status page incident is linked to this one.
	DraftExists      bool `json:"draft_exists"`
	PublishedUpdates int  `json:"published_updates"`
}

func (s *Server) incidentDetailSummary(incident db.Incident, node db.Node, monitor db.Monitor, events []db.IncidentEvent, deliveries []db.AlertDelivery, reports []db.MonitorReport) IncidentDetailSummary {
	now := time.Now().UTC()
	sum := IncidentDetailSummary{DetectedAt: incident.OpenedAt, ResolvedAt: incident.ResolvedAt, NotifiedChannels: []string{}}
	end := now
	if incident.ResolvedAt != nil {
		end = *incident.ResolvedAt
		d := utils.NonNegativeDurationSeconds(end.Sub(incident.OpenedAt))
		sum.TimeToResolveSeconds = &d
	}
	sum.DurationSeconds = utils.NonNegativeDurationSeconds(end.Sub(incident.OpenedAt))
	for _, e := range events {
		if e.Type == "incident_acknowledged" && sum.AcknowledgedAt == nil {
			t := e.CreatedAt
			sum.AcknowledgedAt = &t
			d := utils.NonNegativeDurationSeconds(t.Sub(incident.OpenedAt))
			sum.TimeToAcknowledgeSeconds = &d
		}
	}
	seenChannel := map[string]bool{}
	for _, d := range deliveries {
		if d.Status != "sent" {
			continue
		}
		t := d.CreatedAt
		if d.LastAttemptAt != nil {
			t = *d.LastAttemptAt
		}
		if sum.NotifiedAt == nil || t.Before(*sum.NotifiedAt) {
			sum.NotifiedAt = &t
		}
		if !seenChannel[d.Channel] {
			seenChannel[d.Channel] = true
			sum.NotifiedChannels = append(sum.NotifiedChannels, d.Channel)
		}
	}
	if incident.MonitorID != "" {
		var failed int64
		s.db.Model(&db.MonitorReport{}).Where("monitor_id = ? AND health IN ? AND created_at >= ? AND created_at <= ?", incident.MonitorID, []string{"down", "degraded"}, incident.OpenedAt, end).Count(&failed)
		sum.FailedChecks = int(failed)
	}
	sum.Source = IncidentDetailSource{MonitorID: incident.MonitorID, MonitorName: monitor.Name, MonitorType: monitor.Type, ServerID: node.ID, ServerName: node.Name}
	var cfg db.CoreMonitorConfig
	if incident.MonitorID != "" && s.db.Where("monitor_id = ?", incident.MonitorID).Limit(1).Find(&cfg).RowsAffected > 0 {
		sum.Source.Target = monitorTarget(cfg)
		sum.Expected = expectedFromConfig(cfg)
	}
	sum.Actual = actualFromReports(reports, incident)
	var linked db.StatusPageIncident
	if s.db.Where("internal_incident_id = ?", incident.ID).Limit(1).Find(&linked).RowsAffected > 0 {
		sum.StatusPage.DraftExists = true
		var published int64
		s.db.Model(&db.StatusPageIncidentUpdate{}).Joins("JOIN status_page_incidents spi ON spi.id = status_page_incident_updates.incident_id").Where("spi.internal_incident_id = ? AND status_page_incident_updates.published_at IS NOT NULL", incident.ID).Count(&published)
		sum.StatusPage.PublishedUpdates = int(published)
	}
	if incident.CoveredByIncidentID != "" {
		var covering db.Incident
		if s.db.Select("number").Where("id = ?", incident.CoveredByIncidentID).Limit(1).Find(&covering).RowsAffected > 0 {
			sum.CoveredByDisplayID = incidentDisplayID(covering.Number)
		}
	}
	return sum
}

func monitorConfigMap(cfg db.CoreMonitorConfig) map[string]any {
	out := map[string]any{}
	_ = json.Unmarshal([]byte(cfg.ConfigJSON), &out)
	return out
}

func monitorTarget(cfg db.CoreMonitorConfig) string {
	m := monitorConfigMap(cfg)
	if raw, ok := m["url"].(string); ok && raw != "" {
		method, _ := m["method"].(string)
		if method == "" {
			method = "GET"
		}
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			path := u.EscapedPath()
			if path == "" {
				path = "/"
			}
			return strings.ToUpper(method) + " " + u.Host + path
		}
		return strings.ToUpper(method) + " " + raw
	}
	if host, ok := m["host"].(string); ok && host != "" {
		if port, ok := m["port"].(float64); ok {
			return fmt.Sprintf("%s:%d", host, int(port))
		}
		return host
	}
	return ""
}

func expectedFromConfig(cfg db.CoreMonitorConfig) *IncidentExpectation {
	m := monitorConfigMap(cfg)
	timeout := fmt.Sprintf("within %ds", cfg.TimeoutSeconds)
	switch {
	case m["expected_status"] != nil:
		if code, ok := m["expected_status"].(float64); ok {
			return &IncidentExpectation{Title: fmt.Sprintf("%d %s", int(code), http.StatusText(int(code))), Detail: timeout}
		}
	case cfg.Kind == "heartbeat":
		return &IncidentExpectation{Title: "Signal on schedule", Detail: fmt.Sprintf("every %ds", cfg.IntervalSeconds)}
	case cfg.Kind == "tcp":
		return &IncidentExpectation{Title: "Connection accepted", Detail: timeout}
	}
	return &IncidentExpectation{Title: "Healthy check", Detail: timeout}
}

func actualFromReports(reports []db.MonitorReport, incident db.Incident) *IncidentExpectation {
	if len(reports) == 0 {
		return nil
	}
	latest := reports[0]
	for _, r := range reports {
		if r.CreatedAt.After(latest.CreatedAt) {
			latest = r
		}
	}
	at := latest.CreatedAt
	out := &IncidentExpectation{At: &at, Title: strings.ToUpper(latest.Health[:1]) + latest.Health[1:]}
	var payload map[string]any
	if json.Unmarshal([]byte(latest.Payload), &payload) == nil {
		if msg, ok := payload["error"].(string); ok && msg != "" {
			out.Detail = msg
			if len(out.Detail) > 200 {
				out.Detail = out.Detail[:200]
			}
		}
	}
	return out
}
