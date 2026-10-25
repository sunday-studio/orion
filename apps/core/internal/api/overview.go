package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type OverviewIncidentResponse struct {
	ID              string     `json:"id"`
	Number          int        `json:"number"`
	DisplayID       string     `json:"display_id"`
	Title           string     `json:"title"`
	Severity        string     `json:"severity"`
	Status          string     `json:"status" enums:"open,acknowledged,covered,resolved"`
	Summary         string     `json:"summary,omitempty"`
	OpenedAt        time.Time  `json:"opened_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	DurationSeconds int64      `json:"duration_seconds"`
	// NotifiedAt and AcknowledgedAt are only filled for needs_you.
	NotifiedAt     *time.Time `json:"notified_at,omitempty"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
}

type OverviewFirstRunResponse struct {
	HasServers       bool `json:"has_servers"`
	HasMonitors      bool `json:"has_monitors"`
	HasApps          bool `json:"has_apps"`
	HasAlertChannels bool `json:"has_alert_channels"`
}

type OverviewStatsResponse struct {
	MonitorsUp       int `json:"monitors_up"`
	MonitorsTotal    int `json:"monitors_total"`
	ServersReporting int `json:"servers_reporting"`
	ServersTotal     int `json:"servers_total"`
	// Uptime30dPercent is null until a daily rollup exists.
	Uptime30dPercent *float64 `json:"uptime_30d_percent"`
}

type OverviewDayResponse struct {
	Date   string `json:"date"`
	Status string `json:"status" enums:"up,warn,down,none"`
}

type OverviewServiceResponse struct {
	Name          string                `json:"name"`
	NodeID        string                `json:"node_id"`
	Status        string                `json:"status" enums:"up,degraded,down,unknown"`
	UptimePercent *float64              `json:"uptime_percent"`
	Days          []OverviewDayResponse `json:"days"`
}

// OverviewResponse is everything the Console home screen shows, in one request.
type OverviewResponse struct {
	GeneratedAt time.Time `json:"generated_at"`
	// State: empty (nothing monitored yet) | ok | issues.
	State                string                     `json:"state" enums:"empty,ok,issues"`
	OngoingIncidentCount int                        `json:"ongoing_incident_count"`
	HealthIssueCount     int                        `json:"health_issue_count"`
	FirstRun             OverviewFirstRunResponse   `json:"first_run"`
	NeedsYou             *OverviewIncidentResponse  `json:"needs_you"`
	OngoingIncidents     []OverviewIncidentResponse `json:"ongoing_incidents"`
	RecentIncidents      []OverviewIncidentResponse `json:"recent_incidents"`
	Stats                OverviewStatsResponse      `json:"stats"`
	// Services is the 90-day strip per server, worst first, at most 8.
	Services []OverviewServiceResponse `json:"services"`
}

func overviewIncident(i service.OverviewIncident) OverviewIncidentResponse {
	return OverviewIncidentResponse{ID: i.ID, Number: i.Number, DisplayID: incidentDisplayID(i.Number), Title: i.Title, Severity: i.Severity, Status: i.Status, Summary: i.Summary, OpenedAt: i.OpenedAt, ResolvedAt: i.ResolvedAt,
		DurationSeconds: i.DurationSeconds, NotifiedAt: i.NotifiedAt, AcknowledgedAt: i.AcknowledgedAt}
}

// getOverview returns the home screen data.
// @Summary      Console overview
// @Description  Summary numbers, the incident needing attention, ongoing and recent incidents, first-run flags and the 90-day service strip in one call.
// @Tags         overview
// @Produce      json
// @ID           getOverview
// @Success      200  {object}  utils.APIResponse{data=OverviewResponse}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/overview [get]
func (s *Server) getOverview(c *gin.Context) {
	o, err := service.NewOverviewBuilder(s.db, s.logger, s.reportService).Build(time.Now().UTC())
	if err != nil {
		s.logger.Error("Failed to build overview", "error", err)
		utils.InternalError(c, "Failed to build the overview", err)
		return
	}
	out := OverviewResponse{GeneratedAt: o.GeneratedAt, State: o.State, OngoingIncidentCount: o.OngoingIncidentCount, HealthIssueCount: o.HealthIssueCount,
		FirstRun: OverviewFirstRunResponse(o.FirstRun), OngoingIncidents: []OverviewIncidentResponse{}, RecentIncidents: []OverviewIncidentResponse{}, Services: []OverviewServiceResponse{},
		Stats: OverviewStatsResponse{MonitorsUp: o.Stats.MonitorsUp, MonitorsTotal: o.Stats.MonitorsTotal, ServersReporting: o.Stats.ServersReporting, ServersTotal: o.Stats.ServersTotal, Uptime30dPercent: o.Stats.Uptime30dPercent}}
	if o.NeedsYou != nil {
		n := overviewIncident(*o.NeedsYou)
		out.NeedsYou = &n
	}
	for _, i := range o.Ongoing {
		out.OngoingIncidents = append(out.OngoingIncidents, overviewIncident(i))
	}
	for _, i := range o.Recent {
		out.RecentIncidents = append(out.RecentIncidents, overviewIncident(i))
	}
	for _, svc := range o.Services {
		r := OverviewServiceResponse{Name: svc.Name, NodeID: svc.NodeID, Status: svc.Status, UptimePercent: svc.UptimePercent, Days: []OverviewDayResponse{}}
		for _, d := range svc.Days {
			r.Days = append(r.Days, OverviewDayResponse(d))
		}
		out.Services = append(out.Services, r)
	}
	utils.SuccessResponse(c, http.StatusOK, "Overview retrieved", out)
}
