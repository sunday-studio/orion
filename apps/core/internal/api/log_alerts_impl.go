package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func toLogAlertRule(r db.LogAlertRule) LogAlertRule {
	return LogAlertRule{ID: r.ID, Name: r.Name, AppID: r.AppID, Mode: r.Mode, Severity: r.Severity, Event: r.Event, Q: r.Text, Threshold: r.Threshold,
		WindowSeconds: r.WindowSeconds, Enabled: r.Enabled, State: r.State, LastCount: r.LastCount, LastEvaluatedAt: r.LastEvaluatedAt, CreatedAt: r.CreatedAt}
}

func validateLogAlertRule(r db.LogAlertRule) error {
	switch {
	case strings.TrimSpace(r.Name) == "" || len(r.Name) > 80:
		return errors.New("name is required (at most 80 characters)")
	case r.Mode != service.LogAlertModeThreshold && r.Mode != service.LogAlertModeAbsence:
		return errors.New("mode must be threshold or absence")
	case r.WindowSeconds < 60 || r.WindowSeconds > 86400:
		return errors.New("window_seconds must be between 60 and 86400")
	case r.Threshold < 0 || r.Threshold > 1_000_000:
		return errors.New("threshold must be between 0 and 1000000")
	case len(r.Severity) > 40 || len(r.Event) > 200 || len(r.Text) > 200:
		return errors.New("severity, event and q are limited in length")
	}
	if r.Severity != "" {
		if _, _, ranged := logstore.SeverityRange(r.Severity); !ranged {
			for _, name := range splitList(r.Severity) {
				if canonical, _ := logstore.NormalizeSeverity(name); canonical == "info" && strings.ToLower(strings.TrimSpace(name)) != "info" && !isKnownInfoAlias(name) {
					return errors.New("severity contains an unknown level: " + name)
				}
			}
		}
	}
	return nil
}

func isKnownInfoAlias(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "information", "informational", "notice", "log", "ok":
		return true
	}
	return false
}

func (s *Server) logAlertService() *service.LogAlertService {
	var store *logstore.Store
	if s.logs != nil {
		store = s.logs.store
	}
	return service.NewLogAlertService(s.db, s.logger, s.cfg, store)
}

func (s *Server) listLogAlertRulesImpl(c *gin.Context) {
	var rows []db.LogAlertRule
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to list log alert rules", err)
		return
	}
	out := make([]LogAlertRule, 0, len(rows))
	for _, r := range rows {
		out = append(out, toLogAlertRule(r))
	}
	utils.SuccessResponse(c, http.StatusOK, "Log alert rules retrieved successfully", gin.H{"rules": out})
}

func (s *Server) createLogAlertRuleImpl(c *gin.Context) {
	var req LogAlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "name and app_id are required")
		return
	}
	var app db.App
	if err := s.db.Select("id").Where("id = ?", req.AppID).First(&app).Error; err != nil {
		utils.BadRequest(c, "app_id does not match an app")
		return
	}
	now := time.Now().UTC()
	rule := db.LogAlertRule{ID: utils.GenerateID("log_alert"), Name: strings.TrimSpace(req.Name), AppID: req.AppID, Mode: service.LogAlertModeThreshold,
		Severity: strings.TrimSpace(req.Severity), Event: strings.TrimSpace(req.Event), Text: strings.TrimSpace(req.Q), Threshold: 0, WindowSeconds: 300, Enabled: true, State: "ok", CreatedAt: now, UpdatedAt: now}
	if req.Mode != "" {
		rule.Mode = req.Mode
	}
	if req.Threshold != nil {
		rule.Threshold = *req.Threshold
	}
	if req.WindowSeconds != nil {
		rule.WindowSeconds = *req.WindowSeconds
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if err := validateLogAlertRule(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&rule).Error; err != nil {
		utils.InternalError(c, "Failed to create the rule", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Log alert rule created", gin.H{"rule": toLogAlertRule(rule)})
}

func (s *Server) loadLogAlertRule(c *gin.Context) (db.LogAlertRule, bool) {
	var rule db.LogAlertRule
	if err := s.db.Where("id = ?", c.Param("id")).First(&rule).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "Log alert rule not found")
		} else {
			utils.InternalError(c, "Failed to load the rule", err)
		}
		return db.LogAlertRule{}, false
	}
	return rule, true
}

func (s *Server) updateLogAlertRuleImpl(c *gin.Context) {
	rule, ok := s.loadLogAlertRule(c)
	if !ok {
		return
	}
	var req LogAlertRuleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	wasEnabled := rule.Enabled
	if strings.TrimSpace(req.Name) != "" {
		rule.Name = strings.TrimSpace(req.Name)
	}
	if req.Mode != "" {
		rule.Mode = req.Mode
	}
	if req.Severity != nil {
		rule.Severity = strings.TrimSpace(*req.Severity)
	}
	if req.Event != nil {
		rule.Event = strings.TrimSpace(*req.Event)
	}
	if req.Q != nil {
		rule.Text = strings.TrimSpace(*req.Q)
	}
	if req.Threshold != nil {
		rule.Threshold = *req.Threshold
	}
	if req.WindowSeconds != nil {
		rule.WindowSeconds = *req.WindowSeconds
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if err := validateLogAlertRule(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if wasEnabled && !rule.Enabled {
		if err := s.logAlertService().Retire(&rule, time.Now().UTC()); err != nil {
			utils.InternalError(c, "Failed to resolve the rule's incident", err)
			return
		}
		rule.State = "ok"
	}
	rule.UpdatedAt = time.Now().UTC()
	if err := s.db.Save(&rule).Error; err != nil {
		utils.InternalError(c, "Failed to update the rule", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Log alert rule updated", gin.H{"rule": toLogAlertRule(rule)})
}

func (s *Server) deleteLogAlertRuleImpl(c *gin.Context) {
	rule, ok := s.loadLogAlertRule(c)
	if !ok {
		return
	}
	if err := s.logAlertService().Retire(&rule, time.Now().UTC()); err != nil {
		utils.InternalError(c, "Failed to resolve the rule's incident", err)
		return
	}
	if err := s.db.Delete(&db.LogAlertRule{}, "id = ?", rule.ID).Error; err != nil {
		utils.InternalError(c, "Failed to delete the rule", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Log alert rule deleted", nil)
}

func (s *Server) getIncidentLogsImpl(c *gin.Context) {
	var incident db.Incident
	if err := s.db.Where("id = ?", c.Param("id")).First(&incident).Error; err != nil {
		utils.NotFound(c, "Incident not found")
		return
	}
	limit := min(utils.QueryInt(c, "limit", 100), logstore.MaxPageSize)
	from := incident.OpenedAt.Add(-15 * time.Minute)
	to := time.Now().UTC()
	if incident.ResolvedAt != nil {
		to = incident.ResolvedAt.Add(5 * time.Minute)
	}

	var entries []LogEntry
	appID := strings.TrimSpace(c.Query("app_id"))
	if appID == "" {
		var rule db.LogAlertRule
		if err := s.db.Select("app_id").Where("monitor_id = ?", incident.MonitorID).First(&rule).Error; err == nil {
			appID = rule.AppID
		}
	}
	if appID != "" && s.logs != nil {
		rows, _, err := s.logs.store.List(logstore.Query{AppIDs: []string{appID}, From: from, To: to, Limit: limit})
		if err != nil {
			utils.InternalError(c, "Failed to load app logs", err)
			return
		}
		names := s.appNames([]string{appID})
		for _, r := range rows {
			entries = append(entries, toLogEntry(r, names))
		}
	}
	var agent db.Agent
	if err := s.db.Select("id", "machine_id").Where("id = ?", incident.AgentID).First(&agent).Error; err == nil && agent.MachineId != "core" && agent.MachineId != "" {
		var rows []db.ServiceLogEntry
		if err := s.db.Where("agent_id = ? AND occurred_at BETWEEN ? AND ?", agent.ID, from, to).Order("occurred_at DESC").Limit(limit).Find(&rows).Error; err == nil {
			entries = append(entries, agentLogEntries(rows)...)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Timestamp.After(entries[j].Timestamp) })
	if len(entries) > limit {
		entries = entries[:limit]
	}
	if entries == nil {
		entries = []LogEntry{}
	}
	utils.SuccessResponse(c, http.StatusOK, "Incident logs retrieved successfully", LogListResponse{Entries: entries})
}
