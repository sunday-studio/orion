package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func seedLabelChannels(server *Server) {
	for _, n := range []string{"oncall webhook", "ops-team", "dev digest"} {
		server.db.Create(&db.AlertChannel{ID: "ch-" + n, Name: n, Type: "email", Enabled: true, SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents())})
	}
}

func TestLabelRoutesCRUDOrderAndDryRun(t *testing.T) {
	server := alertServer(t)
	seedLabelChannels(server)
	type routeEnv struct {
		Data struct {
			Route LabelRouteResponse `json:"route"`
		} `json:"data"`
	}
	mk := func(matchers []LabelMatcherDTO, ch string, cont bool) LabelRouteResponse {
		r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes", gin.H{"matchers": matchers, "channel_ids": []string{ch}, "continue_matching": cont}, "")
		var env routeEnv
		decodeResponse(t, r, &env)
		if r.Code != http.StatusCreated {
			t.Fatalf("create = %d %s", r.Code, r.Body.String())
		}
		return env.Data.Route
	}
	a := mk([]LabelMatcherDTO{{Key: "env", Value: "prod"}, {Key: "severity", Value: "critical"}}, "ch-oncall webhook", true)
	b := mk([]LabelMatcherDTO{{Key: "env", Value: "prod"}}, "ch-ops-team", false)
	c := mk(nil, "ch-dev digest", false)
	if a.When != "env=prod AND severity=critical" || a.Then != "Continue" || b.Then != "Stop" || c.When == "" {
		t.Fatalf("responses: %+v %+v %+v", a, b, c)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes", gin.H{"matchers": []LabelMatcherDTO{{Key: "env", Value: "prod"}}, "channel_ids": []string{"nope"}}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("missing channel = %d", r.Code)
	}

	dry := func(labels ...string) LabelRouteDryRunResponse {
		r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes/dry-run", gin.H{"labels": labels}, "")
		var env struct {
			Data LabelRouteDryRunResponse `json:"data"`
		}
		decodeResponse(t, r, &env)
		if r.Code != http.StatusOK {
			t.Fatalf("dry-run = %d %s", r.Code, r.Body.String())
		}
		return env.Data
	}
	both := dry("env=prod", "severity=critical")
	if len(both.Channels) != 2 || both.Summary != "Routes 1 and 2 deliver to oncall webhook and ops-team." || !both.Evaluations[2].Skipped {
		t.Fatalf("both = %+v", both)
	}
	if only := dry("env=prod", "severity=low"); len(only.Channels) != 1 || only.Channels[0] != "ops-team" {
		t.Fatalf("only = %+v", only)
	}
	if rest := dry("env=dev"); len(rest.Channels) != 1 || rest.Channels[0] != "dev digest" {
		t.Fatalf("rest = %+v", rest)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes/dry-run", gin.H{"labels": []string{"broken"}}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("bad label = %d", r.Code)
	}

	// Reorder: the catch-all first stops everything else.
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes/reorder", gin.H{"ids": []string{c.ID, a.ID, b.ID}}, ""); r.Code != http.StatusOK {
		t.Fatalf("reorder = %d %s", r.Code, r.Body.String())
	}
	if first := dry("env=prod", "severity=critical"); len(first.Channels) != 1 || first.Channels[0] != "dev digest" {
		t.Fatalf("after reorder = %+v", first)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/label-routes/reorder", gin.H{"ids": []string{c.ID}}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("partial reorder = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/label-routes/"+c.ID, nil, ""); r.Code != http.StatusOK {
		t.Errorf("delete = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/label-routes/"+c.ID, nil, ""); r.Code != http.StatusNotFound {
		t.Errorf("delete again = %d", r.Code)
	}
}

func TestLabelRoutesQueueDeliveriesAndRuleCooldownReplay(t *testing.T) {
	server := alertServer(t)
	seedLabelChannels(server)
	agent := registerTestAgent(t, server)
	now := time.Now().UTC()
	server.db.Create(&db.Incident{ID: "incL", Number: 901, Status: "open", Severity: "critical", Title: "disk full", AgentID: agent.Data.AgentID, OpenedAt: now, LastEventAt: now, Source: "monitor"})
	alerts := service.NewAlertService(server.db, server.logger, server.cfg)
	if _, err := alerts.CreateLabelRoute(service.LabelRouteInput{Matchers: []service.LabelMatcher{{Key: "severity", Op: "=", Value: "critical"}}, ChannelIDs: []string{"ch-ops-team"}}); err != nil {
		t.Fatal(err)
	}
	if err := alerts.QueueIncidentNotifications("incL", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	var rows []db.AlertDelivery
	server.db.Find(&rows)
	if len(rows) != 1 || rows[0].Channel != "ops-team" {
		t.Fatalf("deliveries = %+v", rows)
	}

	// A rule with its own cooldown, and its enriched response + replay.
	r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules", gin.H{"name": "Critical down", "channel_ids": []string{"ch-oncall webhook"}, "severities": []string{"critical"}, "cooldown_seconds": 600}, "")
	var created struct {
		Data struct {
			Rule AlertRuleResponse `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, r, &created)
	rule := created.Data.Rule
	if r.Code != http.StatusCreated || rule.CooldownSeconds != 600 || rule.HowItRuns.Limit != "Once per 10 min" || rule.HowItRuns.Notify[0] != "oncall webhook" || rule.Summary == "" {
		t.Fatalf("rule = %d %+v", r.Code, rule)
	}
	rp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/"+rule.ID+"/replay", nil, "")
	var replay struct {
		Data RuleReplayResponse `json:"data"`
	}
	decodeResponse(t, rp, &replay)
	if rp.Code != http.StatusOK || replay.Data.IncidentsChecked != 1 || len(replay.Data.WouldSend) != 1 || replay.Data.WouldSend[0].Channels[0] != "oncall webhook" {
		t.Fatalf("replay = %d %+v", rp.Code, replay.Data)
	}
	var after int64
	server.db.Model(&db.AlertDelivery{}).Count(&after)
	if after != 1 {
		t.Fatalf("replay stored deliveries: %d", after)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/missing/replay", nil, ""); r.Code != http.StatusNotFound {
		t.Errorf("missing rule replay = %d", r.Code)
	}
}

func TestLabelRoutesKeepLegacyFallbackForUnmatchedEvents(t *testing.T) {
	server := alertServer(t)
	seedLabelChannels(server)
	agent := registerTestAgent(t, server)
	now := time.Now().UTC()
	server.db.Create(&db.Incident{ID: "incF", Number: 902, Status: "open", Severity: "low", Title: "slow", AgentID: agent.Data.AgentID, OpenedAt: now, LastEventAt: now, Source: "monitor"})
	alerts := service.NewAlertService(server.db, server.logger, server.cfg)
	if _, err := alerts.CreateLabelRoute(service.LabelRouteInput{Matchers: []service.LabelMatcher{{Key: "severity", Op: "=", Value: "critical"}}, ChannelIDs: []string{"ch-ops-team"}}); err != nil {
		t.Fatal(err)
	}
	if err := alerts.QueueIncidentNotifications("incF", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	var n int64
	server.db.Model(&db.AlertDelivery{}).Count(&n)
	if n != 3 {
		t.Fatalf("unmatched event with no rules should reach every channel, got %d deliveries", n)
	}
}
