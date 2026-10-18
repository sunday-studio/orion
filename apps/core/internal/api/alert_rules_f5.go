package api

import (
	"errors"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RuleHowItRuns is the structured "Critical down, how it runs" block.
type RuleHowItRuns struct {
	When   string   `json:"when" example:"An incident opens"`
	If     string   `json:"if" example:"Severity is critical or high"`
	Limit  string   `json:"limit" example:"Once per 10 min"`
	Notify []string `json:"notify"`
	// Quiet is true for suppress rules, which notify nobody.
	Quiet bool `json:"quiet"`
}

func humanSeconds(sec int) string {
	switch {
	case sec >= 86400 && sec%86400 == 0:
		return fmt.Sprintf("%d d", sec/86400)
	case sec >= 3600 && sec%3600 == 0:
		return fmt.Sprintf("%d h", sec/3600)
	case sec >= 60 && sec%60 == 0:
		return fmt.Sprintf("%d min", sec/60)
	default:
		return fmt.Sprintf("%d s", sec)
	}
}

func (s *Server) ruleHowItRuns(route db.AlertRoute, names map[string]string) RuleHowItRuns {
	var events []string
	for _, e := range service.RuleEvents(route) {
		switch e {
		case "incident_opened":
			events = append(events, "An incident opens")
		case "incident_resolved":
			events = append(events, "An incident resolves")
		default:
			events = append(events, strings.ReplaceAll(e, "_", " "))
		}
	}
	var conds []string
	if sev := service.DecodeList(route.Severities); len(sev) > 0 {
		conds = append(conds, "Severity is "+strings.Join(sev, " or "))
	}
	if types := service.DecodeList(route.MonitorTypes); len(types) > 0 {
		conds = append(conds, "Monitor type is "+strings.Join(types, " or "))
	}
	if n := len(service.DecodeList(route.AgentIDs)); n > 0 {
		conds = append(conds, fmt.Sprintf("Server is one of %d selected", n))
	}
	if n := len(service.DecodeList(route.MonitorIDs)); n > 0 {
		conds = append(conds, fmt.Sprintf("Monitor is one of %d selected", n))
	}
	out := RuleHowItRuns{When: strings.Join(events, " or "), If: "Any incident", Limit: "No limit", Notify: []string{}, Quiet: route.Suppress}
	if len(conds) > 0 {
		out.If = strings.Join(conds, " and ")
	}
	cooldown := route.CooldownSeconds
	if cooldown == 0 && s.cfg != nil {
		cooldown = s.cfg.AlertCooldownSeconds
	}
	if cooldown > 0 {
		out.Limit = "Once per " + humanSeconds(cooldown)
	}
	for _, id := range service.DecodeList(route.ChannelIDs) {
		if n := names[id]; n != "" {
			out.Notify = append(out.Notify, n)
		}
	}
	return out
}

// enrichedRule is the rule response with summary, last-24h count and the how-it-runs block.
func (s *Server) enrichedRule(rule db.AlertRoute) AlertRuleResponse {
	r := alertRuleResponse(rule)
	names := s.channelNamesByID()
	r.CooldownSeconds = rule.CooldownSeconds
	r.Summary = ruleSummary(rule, names)
	if rule.CooldownSeconds > 0 {
		r.Summary += ", cooldown " + humanSeconds(rule.CooldownSeconds)
	}
	r.HowItRuns = s.ruleHowItRuns(rule, names)
	s.db.Model(&db.AlertDelivery{}).Where("route_id = ? AND created_at >= ? AND status IN ?", rule.ID, time.Now().UTC().Add(-24*time.Hour), []string{"sent", "failed", "pending"}).
		Select("COUNT(DISTINCT incident_id || ':' || event_type)").Scan(&r.Last24hFired)
	return r
}

type RuleReplayItem struct {
	IncidentID        string    `json:"incident_id"`
	IncidentDisplayID string    `json:"incident_display_id,omitempty"`
	Title             string    `json:"title"`
	Event             string    `json:"event"`
	At                time.Time `json:"at"`
	Channels          []string  `json:"channels"`
}

type RuleReplayResponse struct {
	WindowHours      int              `json:"window_hours"`
	IncidentsChecked int              `json:"incidents_checked"`
	WouldSend        []RuleReplayItem `json:"would_send"`
	// Nothing is delivered; the note repeats that for the screen.
	Note string `json:"note"`
}

// replayAlertRule shows what a rule would have sent over a recent window.
// @Summary      Replay an alert rule
// @Description  Replays the last 24 hours (hours up to 168) against this rule and lists the notifications it would send. Nothing is delivered or stored.
// @Tags         alerts
// @Produce      json
// @ID           replayAlertRule
// @Param        id     path      string  true   "Rule ID"
// @Param        hours  query     int     false  "Window in hours" default(24)
// @Success      200    {object}  utils.APIResponse{data=RuleReplayResponse}
// @Failure      404   {object}  utils.APIResponse
// @Router       /v1/alerts/rules/{id}/replay [post]
func (s *Server) replayAlertRule(c *gin.Context) {
	var rule db.AlertRoute
	if err := s.db.Where("id = ?", c.Param("id")).First(&rule).Error; err != nil {
		utils.NotFound(c, "Alert rule not found")
		return
	}
	hours := max(1, min(utils.QueryInt(c, "hours", 24), 168))
	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)
	var incidents []db.Incident
	s.db.Where("opened_at >= ? OR resolved_at >= ?", since, since).Order("opened_at DESC").Limit(500).Find(&incidents)
	alerts := service.NewAlertService(s.db, s.logger, s.cfg)
	names := s.channelNamesByID()
	out := RuleReplayResponse{WindowHours: hours, IncidentsChecked: len(incidents), WouldSend: []RuleReplayItem{}, Note: "Replays the last " + itoa(hours) + " hours. Nothing is delivered."}
	for _, inc := range incidents {
		for _, event := range service.RuleEvents(rule) {
			at := inc.OpenedAt
			switch event {
			case "incident_opened":
			case "incident_resolved":
				if inc.ResolvedAt == nil {
					continue
				}
				at = *inc.ResolvedAt
			default:
				continue
			}
			if at.Before(since) {
				continue
			}
			ctx, err := alerts.LoadAlertRouteContext(inc.ID, event)
			if err != nil || !rule.Enabled || !service.RuleMatchesEvent(rule, *ctx) {
				continue
			}
			item := RuleReplayItem{IncidentID: inc.ID, IncidentDisplayID: incidentDisplayID(inc.Number), Title: inc.Title, Event: event, At: at, Channels: []string{}}
			if !rule.Suppress {
				for _, id := range service.DecodeList(rule.ChannelIDs) {
					if n := names[id]; n != "" {
						item.Channels = append(item.Channels, n)
					}
				}
			}
			out.WouldSend = append(out.WouldSend, item)
		}
	}
	sort.SliceStable(out.WouldSend, func(i, j int) bool { return out.WouldSend[i].At.After(out.WouldSend[j].At) })
	utils.SuccessResponse(c, http.StatusOK, "Replay finished", out)
}

// ---- label routes (the Routes tab) ----

type LabelMatcherDTO struct {
	Key   string `json:"key" example:"env"`
	Op    string `json:"op,omitempty" enums:"=,!=" example:"="`
	Value string `json:"value" example:"prod"`
}

type LabelRouteRequest struct {
	Name             string            `json:"name,omitempty"`
	Matchers         []LabelMatcherDTO `json:"matchers"`
	ChannelIDs       []string          `json:"channel_ids"`
	ContinueMatching bool              `json:"continue_matching"`
	Enabled          *bool             `json:"enabled,omitempty"`
}

type LabelRouteResponse struct {
	ID               string            `json:"id"`
	Number           int               `json:"number"`
	Name             string            `json:"name,omitempty"`
	Enabled          bool              `json:"enabled"`
	Matchers         []LabelMatcherDTO `json:"matchers"`
	When             string            `json:"when" example:"env=prod AND severity=critical"`
	ChannelIDs       []string          `json:"channel_ids"`
	ChannelNames     []string          `json:"channel_names"`
	ContinueMatching bool              `json:"continue_matching"`
	// Then reads "Continue" or "Stop".
	Then      string    `json:"then" enums:"Continue,Stop"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toMatchers(in []LabelMatcherDTO) []service.LabelMatcher {
	out := make([]service.LabelMatcher, 0, len(in))
	for _, m := range in {
		out = append(out, service.LabelMatcher{Key: m.Key, Op: m.Op, Value: m.Value})
	}
	return out
}

func (s *Server) labelRouteResponse(r db.AlertLabelRoute, number int, names map[string]string) LabelRouteResponse {
	matchers := service.DecodeMatchers(r.Matchers)
	dto := make([]LabelMatcherDTO, 0, len(matchers))
	for _, m := range matchers {
		dto = append(dto, LabelMatcherDTO{Key: m.Key, Op: m.Op, Value: m.Value})
	}
	ids := service.DecodeList(r.ChannelIDs)
	channelNames := make([]string, 0, len(ids))
	for _, id := range ids {
		if n := names[id]; n != "" {
			channelNames = append(channelNames, n)
		}
	}
	return LabelRouteResponse{ID: r.ID, Number: number, Name: r.Name, Enabled: r.Enabled, Matchers: dto, When: service.DescribeMatchers(matchers), ChannelIDs: ids, ChannelNames: channelNames,
		ContinueMatching: r.ContinueMatching, Then: map[bool]string{true: "Continue", false: "Stop"}[r.ContinueMatching], CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

func (s *Server) labelRouteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrLabelRouteInvalid):
		utils.BadRequest(c, strings.TrimPrefix(err.Error(), service.ErrLabelRouteInvalid.Error()+": "))
	case errors.Is(err, service.ErrLabelRouteNotFound):
		utils.NotFound(c, "Route not found")
	default:
		utils.InternalError(c, "Route request failed", err)
	}
}

// listLabelRoutes lists routes in evaluation order.
// @Summary      List label routes
// @Description  Routes pick channels by label. They are tried in order; the first match wins unless Continue is on. A route's channels are added to the channels chosen by rules.
// @Tags         alerts
// @Produce      json
// @ID           listLabelRoutes
// @Success      200  {object}  utils.APIResponse{data=object{routes=[]LabelRouteResponse,count=int}}
// @Router       /v1/alerts/label-routes [get]
func (s *Server) listLabelRoutes(c *gin.Context) {
	routes, err := service.NewAlertService(s.db, s.logger, s.cfg).ListLabelRoutes()
	if err != nil {
		s.labelRouteError(c, err)
		return
	}
	names := s.channelNamesByID()
	out := make([]LabelRouteResponse, 0, len(routes))
	for i, r := range routes {
		out = append(out, s.labelRouteResponse(r, i+1, names))
	}
	utils.SuccessResponse(c, http.StatusOK, "Routes retrieved", gin.H{"routes": out, "count": len(out)})
}

// createLabelRoute adds a route at the end.
// @Summary      Create a label route
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createLabelRoute
// @Param        request  body      LabelRouteRequest  true  "Route"
// @Success      201      {object}  utils.APIResponse{data=object{route=LabelRouteResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Router       /v1/alerts/label-routes [post]
func (s *Server) createLabelRoute(c *gin.Context) {
	var req LabelRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid route payload")
		return
	}
	route, err := service.NewAlertService(s.db, s.logger, s.cfg).CreateLabelRoute(service.LabelRouteInput{Name: req.Name, Matchers: toMatchers(req.Matchers), ChannelIDs: req.ChannelIDs, ContinueMatching: req.ContinueMatching, Enabled: req.Enabled})
	if err != nil {
		s.labelRouteError(c, err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Route created", gin.H{"route": s.labelRouteResponse(*route, route.Position, s.channelNamesByID())})
}

// updateLabelRoute replaces a route's matchers, channels and flags.
// @Summary      Update a label route
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateLabelRoute
// @Param        id       path      string             true  "Route ID"
// @Param        request  body      LabelRouteRequest  true  "Route"
// @Success      200      {object}  utils.APIResponse{data=object{route=LabelRouteResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/alerts/label-routes/{id} [patch]
func (s *Server) updateLabelRoute(c *gin.Context) {
	var req LabelRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid route payload")
		return
	}
	route, err := service.NewAlertService(s.db, s.logger, s.cfg).UpdateLabelRoute(c.Param("id"), service.LabelRouteInput{Name: req.Name, Matchers: toMatchers(req.Matchers), ChannelIDs: req.ChannelIDs, ContinueMatching: req.ContinueMatching, Enabled: req.Enabled})
	if err != nil {
		s.labelRouteError(c, err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Route updated", gin.H{"route": s.labelRouteResponse(*route, route.Position, s.channelNamesByID())})
}

// deleteLabelRoute removes a route.
// @Summary      Delete a label route
// @Tags         alerts
// @Produce      json
// @ID           deleteLabelRoute
// @Param        id   path      string  true  "Route ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/alerts/label-routes/{id} [delete]
func (s *Server) deleteLabelRoute(c *gin.Context) {
	if err := service.NewAlertService(s.db, s.logger, s.cfg).DeleteLabelRoute(c.Param("id")); err != nil {
		s.labelRouteError(c, err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Route deleted", gin.H{})
}

type ReorderLabelRoutesRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// reorderLabelRoutes sets the evaluation order.
// @Summary      Reorder label routes
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           reorderLabelRoutes
// @Param        request  body      ReorderLabelRoutesRequest  true  "All route ids in the new order"
// @Success      200      {object}  utils.APIResponse
// @Failure      400      {object}  utils.APIResponse
// @Router       /v1/alerts/label-routes/reorder [post]
func (s *Server) reorderLabelRoutes(c *gin.Context) {
	var req ReorderLabelRoutesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "ids is required")
		return
	}
	if err := service.NewAlertService(s.db, s.logger, s.cfg).ReorderLabelRoutes(req.IDs); err != nil {
		s.labelRouteError(c, err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Routes reordered", gin.H{})
}

type LabelRouteDryRunRequest struct {
	// Labels are key=value pairs, for example ["env=prod", "severity=critical"].
	Labels []string `json:"labels"`
}

type LabelRouteEvaluation struct {
	Number  int    `json:"number"`
	RouteID string `json:"route_id"`
	When    string `json:"when"`
	Matched bool   `json:"matched"`
	// Skipped: an earlier matching route without Continue ended the evaluation.
	Skipped      bool     `json:"skipped"`
	ChannelNames []string `json:"channel_names"`
}

type LabelRouteDryRunResponse struct {
	Evaluations []LabelRouteEvaluation `json:"evaluations"`
	Channels    []string               `json:"channels"`
	// Summary reads "Routes 1 and 2 deliver to oncall webhook and ops-team."
	Summary string `json:"summary"`
}

// dryRunLabelRoutes tests a label set against the routes.
// @Summary      Dry-run label routes
// @Description  Tests a label set (for example env=prod, severity=critical) and returns which routes match and which channels would be notified. Nothing is sent.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           dryRunLabelRoutes
// @Param        request  body      LabelRouteDryRunRequest  true  "Labels"
// @Success      200      {object}  utils.APIResponse{data=LabelRouteDryRunResponse}
// @Failure      400      {object}  utils.APIResponse
// @Router       /v1/alerts/label-routes/dry-run [post]
func (s *Server) dryRunLabelRoutes(c *gin.Context) {
	var req LabelRouteDryRunRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Labels) > 50 {
		utils.BadRequest(c, "labels must be a list of key=value pairs (at most 50)")
		return
	}
	for _, l := range req.Labels {
		if k, _, ok := strings.Cut(l, "="); !ok || k == "" {
			utils.BadRequest(c, fmt.Sprintf("%q is not a key=value label", l))
			return
		}
	}
	matches, err := service.NewAlertService(s.db, s.logger, s.cfg).EvaluateLabelRoutes(req.Labels)
	if err != nil {
		s.labelRouteError(c, err)
		return
	}
	names := s.channelNamesByID()
	out := LabelRouteDryRunResponse{Evaluations: make([]LabelRouteEvaluation, 0, len(matches)), Channels: []string{}}
	var matchedNumbers []string
	seen := map[string]bool{}
	for _, m := range matches {
		r := s.labelRouteResponse(m.Route, m.Number, names)
		out.Evaluations = append(out.Evaluations, LabelRouteEvaluation{Number: m.Number, RouteID: m.Route.ID, When: r.When, Matched: m.Matched, Skipped: m.Skipped, ChannelNames: r.ChannelNames})
		if m.Matched {
			matchedNumbers = append(matchedNumbers, itoa(m.Number))
			for _, n := range r.ChannelNames {
				if !seen[n] {
					seen[n] = true
					out.Channels = append(out.Channels, n)
				}
			}
		}
	}
	switch {
	case len(matchedNumbers) == 0:
		out.Summary = "No route matches these labels."
	default:
		label := "Route " + matchedNumbers[0]
		if len(matchedNumbers) > 1 {
			label = "Routes " + strings.Join(matchedNumbers[:len(matchedNumbers)-1], ", ") + " and " + matchedNumbers[len(matchedNumbers)-1]
		}
		out.Summary = label + " deliver to " + joinWithAnd(out.Channels) + "."
	}
	utils.SuccessResponse(c, http.StatusOK, "Dry run finished", out)
}

func joinWithAnd(items []string) string {
	switch len(items) {
	case 0:
		return "nobody"
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}
