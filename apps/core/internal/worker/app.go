package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"strconv"
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
	tcpRunnerKind           = "tcp"
	tcpRunnerName           = "tcp_port"
	maxHTTPResponseDrainLen = 512
	maxHTTPBodyCaptureLen   = 4096
)

type dialContextFunc func(context.Context, string, string) (net.Conn, error)

// Options configures the Core monitor worker foundation.
type Options struct {
	HealthInterval time.Duration
	CheckInterval  time.Duration
	LeaseDuration  time.Duration
	ClaimLimit     int
	WorkerID       string
	HTTPClient     *http.Client
	TCPDialContext dialContextFunc
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
	tcpDialContext dialContextFunc
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
	tcpDialContext := opts.TCPDialContext
	if tcpDialContext == nil {
		tcpDialer := &net.Dialer{}
		tcpDialContext = tcpDialer.DialContext
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
		tcpDialContext: tcpDialContext,
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
	case tcpRunnerKind, tcpRunnerName:
		result := a.runTCPCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeTCPReport(monitorConfig.MonitorID, result)
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
	URL               string   `json:"url"`
	Method            string   `json:"method"`
	ExpectedStatus    int      `json:"expected_status"`
	ExpectedStatuses  []int    `json:"expected_statuses"`
	RequiredContains  []string `json:"required_contains"`
	ForbiddenContains []string `json:"forbidden_contains"`
}

type httpStatusResult struct {
	Health           string
	FinishedAt       time.Time
	Duration         time.Duration
	TargetURL        string
	Method           string
	ExpectedStatus   int
	ExpectedStatuses []int
	StatusCode       int
	BodySample       string
	BodyTruncated    bool
	Error            error
	FailureStage     string
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
	result.ExpectedStatuses = runnerConfig.ExpectedStatuses

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

	result.StatusCode = response.StatusCode
	bodySample, truncated, err := captureHTTPBodySample(response.Body)
	if err != nil {
		result.Error = err
		result.FailureStage = "http_body"
		return result
	}
	result.BodySample = bodySample
	result.BodyTruncated = truncated

	if !httpStatusMatches(response.StatusCode, result.ExpectedStatus, result.ExpectedStatuses) {
		result.Error = fmt.Errorf("unexpected HTTP status %d", response.StatusCode)
		result.FailureStage = "http_response"
		return result
	}
	if missing := missingRequiredContains(result.BodySample, runnerConfig.RequiredContains); len(missing) > 0 {
		result.Error = fmt.Errorf("required response content not found: %s", strings.Join(missing, ", "))
		result.FailureStage = "body_required"
		return result
	}
	if found := foundForbiddenContains(result.BodySample, runnerConfig.ForbiddenContains); len(found) > 0 {
		result.Error = fmt.Errorf("forbidden response content found: %s", strings.Join(found, ", "))
		result.FailureStage = "body_forbidden"
		return result
	}

	result.Health = "up"
	return result
}

type tcpConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type tcpResult struct {
	Health       string
	FinishedAt   time.Time
	Duration     time.Duration
	Host         string
	Port         int
	Address      string
	Error        error
	FailureStage string
}

func (a *App) runTCPCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) tcpResult {
	startedAt := time.Now()
	result := tcpResult{
		Health:     "down",
		FinishedAt: startedAt.UTC(),
	}

	runnerConfig, err := parseTCPConfig(monitorConfig.ConfigJSON)
	if err != nil {
		result.Error = err
		result.FailureStage = "config"
		return result
	}
	result.Host = runnerConfig.Host
	result.Port = runnerConfig.Port
	result.Address = net.JoinHostPort(runnerConfig.Host, strconv.Itoa(runnerConfig.Port))

	timeout := time.Duration(monitorConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := a.tcpDialContext(checkCtx, "tcp", result.Address)
	result.FinishedAt = time.Now().UTC()
	result.Duration = time.Since(startedAt)
	if err != nil {
		result.Error = err
		result.FailureStage = tcpFailureStage(err)
		return result
	}
	if conn != nil {
		_ = conn.Close()
	}
	result.Health = "up"
	return result
}

func parseTCPConfig(raw string) (tcpConfig, error) {
	var cfg tcpConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse config json: %w", err)
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		return cfg, fmt.Errorf("host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("port must be between 1 and 65535")
	}
	return cfg, nil
}

func tcpFailureStage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return "connect"
}

func captureHTTPBodySample(body io.Reader) (string, bool, error) {
	limited := io.LimitReader(body, maxHTTPBodyCaptureLen+1)
	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	truncated := len(bodyBytes) > maxHTTPBodyCaptureLen
	if truncated {
		bodyBytes = bodyBytes[:maxHTTPBodyCaptureLen]
	}
	_, _ = io.CopyN(io.Discard, body, maxHTTPResponseDrainLen)
	return string(bodyBytes), truncated, nil
}

func missingRequiredContains(body string, required []string) []string {
	missing := []string{}
	for _, keyword := range required {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if !strings.Contains(body, keyword) {
			missing = append(missing, keyword)
		}
	}
	return missing
}

func foundForbiddenContains(body string, forbidden []string) []string {
	found := []string{}
	for _, keyword := range forbidden {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if strings.Contains(body, keyword) {
			found = append(found, keyword)
		}
	}
	return found
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
	seenStatuses := map[int]struct{}{}
	normalizedStatuses := []int{}
	for _, status := range cfg.ExpectedStatuses {
		if status < 100 || status > 599 {
			return cfg, fmt.Errorf("expected_statuses must contain values between 100 and 599")
		}
		if _, exists := seenStatuses[status]; exists {
			continue
		}
		seenStatuses[status] = struct{}{}
		normalizedStatuses = append(normalizedStatuses, status)
	}
	if cfg.ExpectedStatus != 0 {
		if _, exists := seenStatuses[cfg.ExpectedStatus]; !exists {
			normalizedStatuses = append([]int{cfg.ExpectedStatus}, normalizedStatuses...)
		}
	}
	cfg.ExpectedStatuses = normalizedStatuses

	return cfg, nil
}

func httpStatusMatches(statusCode int, expectedStatus int, expectedStatuses []int) bool {
	if len(expectedStatuses) == 0 && expectedStatus == 0 {
		return statusCode >= 200 && statusCode <= 299
	}
	for _, expected := range expectedStatuses {
		if statusCode == expected {
			return true
		}
	}
	if expectedStatus == 0 {
		return false
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

func (a *App) storeTCPReport(monitorID string, result tcpResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   tcpPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = tcpPayload(result, result.Error)
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
	}
	if len(result.ExpectedStatuses) > 0 {
		payload["expected_statuses"] = result.ExpectedStatuses
	}
	if result.ExpectedStatus == 0 && len(result.ExpectedStatuses) == 0 {
		payload["expected_status"] = "2xx"
	}
	if result.FailureStage == "body_required" || result.FailureStage == "body_forbidden" {
		payload["body_sample"] = result.BodySample
		payload["body_truncated"] = result.BodyTruncated
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}

func tcpPayload(result tcpResult, resultErr error) map[string]interface{} {
	payload := map[string]interface{}{
		"runner":        "core",
		"type":          "tcp",
		"host":          result.Host,
		"port":          result.Port,
		"address":       result.Address,
		"duration_ms":   result.Duration.Milliseconds(),
		"ok":            result.Health == "up",
		"collected_at":  result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage": result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
