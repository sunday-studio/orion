package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultHealthInterval   = 30 * time.Second
	defaultCheckInterval    = 5 * time.Second
	defaultLeaseDuration    = 2 * time.Minute
	defaultHTTPTimeout      = 10 * time.Second
	defaultCoreWorkerID     = "core-monitor-worker"
	httpStatusRunnerKind    = "http"
	httpStatusRunnerName    = "http_status"
	maxHTTPResponseDrainLen = 512
)

// Options configures the Core monitor worker foundation.
type Options struct {
	HealthInterval time.Duration
	CheckInterval  time.Duration
	LeaseDuration  time.Duration
	ClaimLimit     int
	WorkerID       string
	HTTPClient     *http.Client
	Config         *config.Config
}

// App is the independent Core monitor worker process.
type App struct {
	db             *gorm.DB
	logger         *logging.Logger
	healthInterval time.Duration
	checkInterval  time.Duration
	leaseDuration  time.Duration
	claimLimit     int
	workerID       string
	httpClient     *http.Client
	scheduler      *service.CoreMonitorSchedulerService
	reports        *service.ReportService
}

// NewApp creates a worker app bound to the Core database.
func NewApp(database *gorm.DB, logger *logging.Logger, opts Options) *App {
	healthInterval := opts.HealthInterval
	if healthInterval <= 0 {
		healthInterval = defaultHealthInterval
	}
	checkInterval := opts.CheckInterval
	if checkInterval <= 0 {
		checkInterval = defaultCheckInterval
	}
	leaseDuration := opts.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = defaultLeaseDuration
	}
	workerID := strings.TrimSpace(opts.WorkerID)
	if workerID == "" {
		workerID = defaultCoreWorkerID
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &App{
		db:             database,
		logger:         logger,
		healthInterval: healthInterval,
		checkInterval:  checkInterval,
		leaseDuration:  leaseDuration,
		claimLimit:     opts.ClaimLimit,
		workerID:       workerID,
		httpClient:     httpClient,
		scheduler:      service.NewCoreMonitorSchedulerService(database, logger),
		reports:        service.NewReportService(database, logger, opts.Config),
	}
}

// Run logs worker health and executes due Core monitor checks until the context is canceled.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Core monitor worker started", "worker_id", a.workerID, "health_interval", a.healthInterval.String(), "check_interval", a.checkInterval.String())
	a.logHealth(ctx)
	if err := a.runDueChecks(ctx); err != nil {
		a.logger.Error("Core monitor worker due check sweep failed", "error", err)
	}

	healthTicker := time.NewTicker(a.healthInterval)
	defer healthTicker.Stop()
	checkTicker := time.NewTicker(a.checkInterval)
	defer checkTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Core monitor worker shutting down", "reason", ctx.Err().Error())
			return nil
		case <-healthTicker.C:
			a.logHealth(ctx)
		case <-checkTicker.C:
			if err := a.runDueChecks(ctx); err != nil {
				a.logger.Error("Core monitor worker due check sweep failed", "error", err)
			}
		}
	}
}

func (a *App) logHealth(ctx context.Context) {
	if err := a.checkDatabase(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.logger.Info("Core monitor worker health check interrupted", "error", err)
			return
		}
		a.logger.Error("Core monitor worker health check failed", "database", "unavailable", "error", err)
		return
	}
	a.logger.Info("Core monitor worker health check passed", "database", "ok")
}

func (a *App) checkDatabase(ctx context.Context) error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (a *App) runDueChecks(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	claimed, err := a.scheduler.ClaimDueCoreMonitorConfigs(service.ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    a.workerID,
		Limit:         a.claimLimit,
		LeaseDuration: a.leaseDuration,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	for _, monitorConfig := range claimed {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := a.runClaimedCheck(ctx, monitorConfig); err != nil {
			a.logger.Error("Core monitor check failed", "monitor_id", monitorConfig.MonitorID, "kind", monitorConfig.Kind, "error", err)
		}
	}
	return nil
}

func (a *App) runClaimedCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) error {
	finishedAt := time.Now().UTC()
	success := false
	complete := true
	reportErr := error(nil)

	switch strings.ToLower(strings.TrimSpace(monitorConfig.Kind)) {
	case httpStatusRunnerKind, httpStatusRunnerName:
		result := a.runHTTPStatusCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeHTTPStatusReport(monitorConfig.MonitorID, result)
	default:
		complete = false
		a.logger.Warn("Skipping unsupported Core monitor kind", "monitor_id", monitorConfig.MonitorID, "kind", monitorConfig.Kind)
	}
	if reportErr != nil {
		return reportErr
	}
	if !complete {
		return nil
	}

	_, err := a.scheduler.CompleteCoreMonitorCheck(service.CompleteCoreMonitorCheckRequest{
		MonitorID:  monitorConfig.MonitorID,
		LeaseOwner: a.workerID,
		FinishedAt: finishedAt,
		Success:    success,
	})
	return err
}

type httpStatusConfig struct {
	URL            string `json:"url"`
	Method         string `json:"method"`
	ExpectedStatus int    `json:"expected_status"`
}

type httpStatusResult struct {
	Health         string
	FinishedAt     time.Time
	Duration       time.Duration
	TargetURL      string
	Method         string
	ExpectedStatus int
	StatusCode     int
	Error          error
	FailureStage   string
}

func (a *App) runHTTPStatusCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) httpStatusResult {
	startedAt := time.Now()
	finishedAt := startedAt.UTC()
	result := httpStatusResult{
		Health:     "down",
		FinishedAt: finishedAt,
		Method:     http.MethodGet,
	}

	runnerConfig, err := parseHTTPStatusConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.TargetURL = runnerConfig.URL
	result.Method = runnerConfig.Method
	result.ExpectedStatus = runnerConfig.ExpectedStatus

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(checkCtx, result.Method, result.TargetURL, nil)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_request"
		return result
	}
	request.Header.Set("User-Agent", "orion-core-monitor-worker")

	response, err := a.httpClient.Do(request)
	finishedAt = time.Now().UTC()
	result.FinishedAt = finishedAt
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_request"
		return result
	}
	defer response.Body.Close()
	_, _ = io.CopyN(io.Discard, response.Body, maxHTTPResponseDrainLen)

	result.StatusCode = response.StatusCode
	if httpStatusMatches(response.StatusCode, result.ExpectedStatus) {
		result.Health = "up"
		return result
	}

	result.Error = fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
	result.FailureStage = "http_response"
	return result
}

func parseHTTPStatusConfig(raw string) (httpStatusConfig, error) {
	var cfg httpStatusConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}

	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return cfg, fmt.Errorf("url is required")
	}
	parsedURL, err := url.ParseRequestURI(cfg.URL)
	if err != nil {
		return cfg, fmt.Errorf("url is invalid: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return cfg, fmt.Errorf("url scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return cfg, fmt.Errorf("url host is required")
	}

	cfg.Method = strings.ToUpper(strings.TrimSpace(cfg.Method))
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.Method != http.MethodGet && cfg.Method != http.MethodHead {
		return cfg, fmt.Errorf("method must be GET or HEAD")
	}
	if cfg.ExpectedStatus != 0 && (cfg.ExpectedStatus < 100 || cfg.ExpectedStatus > 599) {
		return cfg, fmt.Errorf("expected_status must be between 100 and 599")
	}

	return cfg, nil
}

func httpStatusMatches(statusCode int, expectedStatus int) bool {
	if expectedStatus == 0 {
		return statusCode >= 200 && statusCode <= 299
	}
	return statusCode == expectedStatus
}

func (a *App) storeHTTPStatusReport(monitorID string, result httpStatusResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   httpStatusPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = httpStatusPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func httpStatusPayload(result httpStatusResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":        "core",
		"type":          "http",
		"method":        result.Method,
		"target_url":    result.TargetURL,
		"status_code":   result.StatusCode,
		"duration_ms":   result.Duration.Milliseconds(),
		"ok":            result.Health == "up",
		"collected_at":  result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage": result.FailureStage,
	}
	if result.ExpectedStatus > 0 {
		payload["expected_status"] = result.ExpectedStatus
	} else {
		payload["expected_status"] = "2xx"
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
