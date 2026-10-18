package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"orion/core/internal/worker"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sharedDryRun is process-wide: one throwaway database serialises all test runs.
var sharedDryRun dryRunRunner

// dryRunRunner executes a check against a throwaway in-memory database, so the real runners and
// their report logic run unchanged but nothing reaches real history, incidents or alerts.
type dryRunRunner struct {
	sem  chan struct{}
	init sync.Once
	once sync.Once
	db   *gorm.DB
	err  error
}

func (d *dryRunRunner) database() (*gorm.DB, error) {
	d.once.Do(func() {
		tmp, err := gorm.Open(sqlite.Open("file:orion-dry-run?mode=memory&cache=shared"), &gorm.Config{})
		if err == nil {
			err = db.Migrate(tmp)
		}
		d.db, d.err = tmp, err
	})
	return d.db, d.err
}

func (d *dryRunRunner) run(ctx context.Context, s *Server, cfg db.CoreMonitorConfig) (*db.MonitorReport, error) {
	d.init.Do(func() { d.sem = make(chan struct{}, 1) })
	select {
	case d.sem <- struct{}{}:
		defer func() { <-d.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	tmp, err := d.database()
	if err != nil {
		return nil, err
	}
	for _, table := range []string{"monitor_reports", "core_monitor_configs", "incident_events", "alert_deliveries", "incidents", "monitors", "agents"} {
		if err := tmp.Exec("DELETE FROM " + table).Error; err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	if err := tmp.Create(&db.Agent{ID: "dry-run-agent", MachineId: "dry-run", Name: "dry run", OS: "core", Arch: "internal", Token: "x", System: true, CreatedAt: now, LastSeen: now}).Error; err != nil {
		return nil, err
	}
	if err := tmp.Create(&db.Monitor{ID: "dry-run", AgentID: "dry-run-agent", Name: "dry run", Type: cfg.Kind, Lifecycle: "active", Health: "unknown", ComputedHealth: "unknown", System: true, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		return nil, err
	}
	cfg.MonitorID, cfg.NextRunAt, cfg.Paused = "dry-run", now, false
	if err := tmp.Create(&cfg).Error; err != nil {
		return nil, err
	}
	app := worker.NewApp(tmp, s.logger, worker.Options{WorkerID: "dry-run", Config: s.cfg})
	if err := app.RunImmediateCheck(ctx, cfg); err != nil {
		return nil, err
	}
	var report db.MonitorReport
	if err := tmp.Where("monitor_id = ?", "dry-run").Order("created_at DESC").First(&report).Error; err != nil {
		return nil, err
	}
	return &report, nil
}

// MonitorTestRunResponse is the "Test result" dialog.
type MonitorTestRunResponse struct {
	Passed bool `json:"passed"`
	// Status is passed or failed.
	Status       string               `json:"status" enums:"passed,failed"`
	Expected     *IncidentExpectation `json:"expected,omitempty"`
	Actual       IncidentExpectation  `json:"actual"`
	DurationMs   *int64               `json:"duration_ms,omitempty"`
	Error        string               `json:"error,omitempty"`
	FailureStage string               `json:"failure_stage,omitempty"`
	StatusCode   *int                 `json:"status_code,omitempty"`
	// Notes always includes that a test run creates no history or alerts.
	Notes []string  `json:"notes"`
	RanAt time.Time `json:"ran_at"`
}

type TestRunMonitorRequest struct {
	Kind           string                 `json:"kind" binding:"required"`
	Config         map[string]interface{} `json:"config" binding:"required"`
	SecretRefs     map[string]interface{} `json:"secret_refs,omitempty"`
	TimeoutSeconds int                    `json:"timeout_seconds,omitempty"`
}

// testRunMonitor runs a saved monitor's check without recording anything.
// @Summary      Test-run a saved monitor
// @Description  Runs the check once and returns the result. Unlike POST /v1/monitors/{id}/test it stores no report, changes no health and opens no incident or alert. Limited to 20 runs per minute.
// @Tags         monitors
// @Produce      json
// @ID           testRunMonitor
// @Param        id   path      string  true  "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=MonitorTestRunResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      429  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/test-run [post]
func (s *Server) testRunMonitor(c *gin.Context) {
	if !s.testRunLimiter.Allow("test-run") {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many test runs; wait a minute", nil)
		return
	}
	record, err := s.coreMonitorManagementService.GetCoreMonitorConfig(c.Param("id"))
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to load monitor")
		return
	}
	s.finishTestRun(c, record.Config)
}

// testRunUnsavedMonitor runs a check from form values, for "Test now" while creating.
// @Summary      Test-run a monitor that is not saved yet
// @Description  Validates the config with the same rules and target policy as creating a monitor, runs it once and returns the result. Nothing is stored.
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           testRunUnsavedMonitor
// @Param        request  body      TestRunMonitorRequest  true  "Check to run"
// @Success      200      {object}  utils.APIResponse{data=MonitorTestRunResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      429      {object}  utils.APIResponse
// @Router       /v1/monitors/test-run [post]
func (s *Server) testRunUnsavedMonitor(c *gin.Context) {
	if !s.testRunLimiter.Allow("test-run") {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many test runs; wait a minute", nil)
		return
	}
	var req TestRunMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "kind and config are required")
		return
	}
	configJSON, _ := json.Marshal(req.Config)
	secretsJSON, _ := json.Marshal(req.SecretRefs)
	if req.SecretRefs == nil {
		secretsJSON = []byte("{}")
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if err := s.coreMonitorManagementService.ValidateCoreMonitorConfig(kind, string(configJSON), string(secretsJSON)); err != nil {
		s.handleCoreMonitorManagementError(c, err, "Invalid monitor configuration")
		return
	}
	timeout := req.TimeoutSeconds
	if timeout <= 0 || timeout > 60 {
		timeout = 10
	}
	s.finishTestRun(c, db.CoreMonitorConfig{Kind: kind, ConfigJSON: string(configJSON), SecretRefJSON: string(secretsJSON), IntervalSeconds: 60, TimeoutSeconds: timeout})
}

func (s *Server) finishTestRun(c *gin.Context, cfg db.CoreMonitorConfig) {
	if cfg.Kind == "heartbeat" {
		utils.BadRequest(c, "Heartbeat monitors wait for your job's ping; there is nothing to run")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(cfg.TimeoutSeconds+5)*time.Second)
	defer cancel()
	report, err := sharedDryRun.run(ctx, s, cfg)
	if err != nil {
		s.logger.Error("Monitor test run failed to execute", "error", err)
		utils.InternalError(c, "Failed to run the test", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Test run finished", testRunResponse(cfg, *report))
}

func testRunResponse(cfg db.CoreMonitorConfig, report db.MonitorReport) MonitorTestRunResponse {
	summary, duration, _ := describeMonitorReport(report)
	out := MonitorTestRunResponse{Passed: report.Health == "up", DurationMs: duration, Expected: expectedFromConfig(cfg), RanAt: report.CreatedAt,
		Actual: IncidentExpectation{Title: summary}, Notes: []string{"Test runs do not create check history or alerts."}}
	out.Status = map[bool]string{true: "passed", false: "failed"}[out.Passed]
	var payload map[string]any
	if json.Unmarshal([]byte(report.Payload), &payload) == nil {
		if msg, ok := payload["error"].(string); ok {
			out.Error = msg
			out.Actual.Detail = msg
		}
		if stage, ok := payload["failure_stage"].(string); ok && stage != "" {
			out.FailureStage = stage
			if !out.Passed {
				out.Notes = append(out.Notes, fmt.Sprintf("The check failed at stage: %s.", stage))
			}
		}
		if code, ok := payload["status_code"].(float64); ok && code > 0 {
			c := int(code)
			out.StatusCode = &c
		}
	}
	if out.Actual.Title == "" {
		out.Actual.Title = map[bool]string{true: "OK", false: "Failed"}[out.Passed]
	}
	return out
}
