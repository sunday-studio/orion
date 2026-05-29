package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultHealthInterval    = 30 * time.Second
	defaultCheckInterval     = 5 * time.Second
	defaultLeaseDuration     = 2 * time.Minute
	defaultHeartbeatInterval = 60 * time.Second
	defaultHTTPTimeout       = 10 * time.Second
	defaultCoreWorkerID      = "core-monitor-worker"
	httpStatusRunnerKind     = "http"
	httpStatusRunnerName     = "http_status"
	httpKeywordRunnerName    = "http_keyword"
	expectedStatusRunnerName = "expected_status"
	tcpRunnerKind            = "tcp"
	tcpRunnerName            = "tcp_port"
	dnsRunnerKind            = "dns"
	tlsRunnerKind            = "tls"
	tlsRunnerName            = "tls_certificate"
	udpRunnerKind            = "udp"
	apiRequestRunnerKind     = "api_request"
	domainExpirationKind     = "domain_expiration"
	pingRunnerKind           = "ping"
	mailRunnerKind           = "mail"
	smtpRunnerKind           = "smtp"
	imapRunnerKind           = "imap"
	popRunnerKind            = "pop"
	pop3RunnerKind           = "pop3"
	syntheticRunnerKind      = "synthetic"
	syntheticRunnerName      = "synthetic_multi_step"
	playwrightRunnerKind     = "playwright"
	playwrightRunnerName     = "playwright_transaction"
	maxHTTPResponseDrainLen  = 512
	maxHTTPBodyCaptureLen    = 4096
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
	DNSResolver    dnsResolver
	TLSCheck       tlsCheckFunc
	UDPDialContext dialContextFunc
	PlaywrightRun  playwrightRunFunc
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
	dnsResolver    dnsResolver
	tlsCheck       tlsCheckFunc
	tlsWarningDays int
	udpDialContext dialContextFunc
	playwrightRun  playwrightRunFunc
	targetPolicy   service.CoreMonitorTargetPolicy
	scheduler      *service.CoreMonitorSchedulerService
	reports        *service.ReportService
}

type checkRunResult struct {
	finishedAt time.Time
	success    bool
	complete   bool
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
	targetPolicy := service.NewCoreMonitorTargetPolicy(opts.Config)
	httpClient = targetPolicy.HTTPClient(httpClient)
	tcpDialContext := opts.TCPDialContext
	if tcpDialContext == nil {
		tcpDialContext = targetPolicy.DialContext
	}
	dnsResolver := opts.DNSResolver
	if dnsResolver == nil {
		dnsResolver = net.DefaultResolver
	}
	tlsCheck := opts.TLSCheck
	if tlsCheck == nil {
		tlsCheck = defaultTLSCheckWithDialContext(targetPolicy.DialContext)
	}
	tlsWarningDays := defaultTLSWarningDays
	if opts.Config != nil && opts.Config.AlertTLSExpiryDays > 0 {
		tlsWarningDays = opts.Config.AlertTLSExpiryDays
	}
	udpDialContext := opts.UDPDialContext
	if udpDialContext == nil {
		udpDialContext = targetPolicy.DialContext
	}
	playwrightRun := opts.PlaywrightRun
	if playwrightRun == nil {
		playwrightRun = defaultPlaywrightRun
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
		dnsResolver:    dnsResolver,
		tlsCheck:       tlsCheck,
		tlsWarningDays: tlsWarningDays,
		udpDialContext: udpDialContext,
		playwrightRun:  playwrightRun,
		targetPolicy:   targetPolicy,
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
	if err := a.reconcileMissedHeartbeats(ctx, time.Now().UTC()); err != nil {
		a.logger.Error("Core monitor worker heartbeat reconciliation failed", "error", err)
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
			if err := a.reconcileMissedHeartbeats(ctx, time.Now().UTC()); err != nil {
				a.logger.Error("Core monitor worker heartbeat reconciliation failed", "error", err)
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

func (a *App) reconcileMissedHeartbeats(ctx context.Context, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var configs []db.CoreMonitorConfig
	if err := a.db.
		Preload("Monitor").
		Joins("JOIN monitors ON monitors.id = core_monitor_configs.monitor_id").
		Where("core_monitor_configs.kind = ?", "heartbeat").
		Where("core_monitor_configs.paused = ?", false).
		Where("core_monitor_configs.last_signal_at IS NOT NULL").
		Where("monitors.lifecycle = ?", "active").
		Find(&configs).Error; err != nil {
		return err
	}

	for _, monitorConfig := range configs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !heartbeatSignalIsMissed(monitorConfig, now) {
			continue
		}
		if heartbeatFailureIsCurrent(monitorConfig) {
			continue
		}
		if err := a.storeMissedHeartbeatReport(monitorConfig, now); err != nil {
			return err
		}
	}
	return nil
}

func heartbeatSignalIsMissed(config db.CoreMonitorConfig, now time.Time) bool {
	if config.LastSignalAt == nil {
		return false
	}
	interval := time.Duration(config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	grace := time.Duration(heartbeatGraceSeconds(config.ConfigJSON)) * time.Second
	return now.After(config.LastSignalAt.UTC().Add(interval + grace))
}

func heartbeatFailureIsCurrent(config db.CoreMonitorConfig) bool {
	if config.LastSignalAt == nil || config.LastFailureAt == nil {
		return false
	}
	return !config.LastFailureAt.UTC().Before(config.LastSignalAt.UTC())
}

func heartbeatGraceSeconds(configJSON string) int {
	var cfg struct {
		GraceSeconds int `json:"grace_seconds"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil || cfg.GraceSeconds < 0 {
		return 0
	}
	return cfg.GraceSeconds
}

func (a *App) storeMissedHeartbeatReport(monitorConfig db.CoreMonitorConfig, now time.Time) error {
	intervalSeconds := monitorConfig.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = int(defaultHeartbeatInterval.Seconds())
	}
	graceSeconds := heartbeatGraceSeconds(monitorConfig.ConfigJSON)
	missedAfter := monitorConfig.LastSignalAt.UTC().Add(time.Duration(intervalSeconds+graceSeconds) * time.Second)
	payload := service.MonitorReportPayload{
		Timestamp: now.Format(time.RFC3339),
		Health:    "stale",
		Metrics: map[string]interface{}{
			"runner":           "core",
			"type":             "heartbeat",
			"failure_stage":    "missed_signal",
			"last_signal_at":   monitorConfig.LastSignalAt.UTC().Format(time.RFC3339),
			"missed_after":     missedAfter.Format(time.RFC3339),
			"interval_seconds": intervalSeconds,
			"grace_seconds":    graceSeconds,
		},
	}
	if _, err := a.reports.StoreMonitorReport(monitorConfig.MonitorID, payload); err != nil {
		return err
	}
	return a.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", monitorConfig.MonitorID).
		Updates(map[string]interface{}{
			"last_failure_at": now,
			"updated_at":      now,
		}).Error
}

// RunImmediateCheck executes one Core monitor check without requiring a scheduler lease.
func (a *App) RunImmediateCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) error {
	result, err := a.runCheckAndStore(ctx, monitorConfig)
	if err != nil {
		return err
	}
	if !result.complete {
		return fmt.Errorf("unsupported core monitor kind %q", monitorConfig.Kind)
	}
	interval := time.Duration(monitorConfig.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultCheckInterval
	}
	updates := map[string]interface{}{
		"last_run_at":      &result.finishedAt,
		"next_run_at":      result.finishedAt.Add(interval),
		"lease_owner":      "",
		"lease_expires_at": nil,
		"updated_at":       result.finishedAt,
	}
	if result.success {
		updates["last_success_at"] = &result.finishedAt
	} else {
		updates["last_failure_at"] = &result.finishedAt
	}
	return a.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorConfig.MonitorID).Updates(updates).Error
}

func (a *App) runClaimedCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) error {
	result, err := a.runCheckAndStore(ctx, monitorConfig)
	if err != nil {
		return err
	}
	if !result.complete {
		return nil
	}

	_, err = a.scheduler.CompleteCoreMonitorCheck(service.CompleteCoreMonitorCheckRequest{
		MonitorID:  monitorConfig.MonitorID,
		LeaseOwner: a.workerID,
		FinishedAt: result.finishedAt,
		Success:    result.success,
	})
	return err
}

func (a *App) runCheckAndStore(ctx context.Context, monitorConfig db.CoreMonitorConfig) (checkRunResult, error) {
	finishedAt := time.Now().UTC()
	success := false
	complete := true
	reportErr := error(nil)

	switch strings.ToLower(strings.TrimSpace(monitorConfig.Kind)) {
	case httpStatusRunnerKind, httpStatusRunnerName, httpKeywordRunnerName, expectedStatusRunnerName:
		result := a.runHTTPStatusCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeHTTPStatusReport(monitorConfig.MonitorID, result)
	case tcpRunnerKind, tcpRunnerName:
		result := a.runTCPCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeTCPReport(monitorConfig.MonitorID, result)
	case dnsRunnerKind:
		result := a.runDNSCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeDNSReport(monitorConfig.MonitorID, result)
	case tlsRunnerKind, tlsRunnerName:
		result := a.runTLSCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeTLSReport(monitorConfig.MonitorID, result)
	case udpRunnerKind:
		result := a.runUDPCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeUDPReport(monitorConfig.MonitorID, result)
	case apiRequestRunnerKind:
		result := a.runAPIRequestCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeAPIRequestReport(monitorConfig.MonitorID, result)
	case domainExpirationKind:
		result := a.runDomainExpirationCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeDomainExpirationReport(monitorConfig.MonitorID, result)
	case pingRunnerKind:
		result := a.runPingCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storePingReport(monitorConfig.MonitorID, result)
	case mailRunnerKind, smtpRunnerKind, imapRunnerKind, popRunnerKind, pop3RunnerKind:
		result := a.runMailCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeMailReport(monitorConfig.MonitorID, result)
	case syntheticRunnerKind, syntheticRunnerName:
		result := a.runSyntheticCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storeSyntheticReport(monitorConfig.MonitorID, result)
	case playwrightRunnerKind, playwrightRunnerName:
		result := a.runPlaywrightCheck(ctx, monitorConfig)
		finishedAt = result.FinishedAt
		success = result.Health == "up"
		reportErr = a.storePlaywrightReport(monitorConfig.MonitorID, result)
	default:
		complete = false
		a.logger.Warn("Skipping unsupported Core monitor kind", "monitor_id", monitorConfig.MonitorID, "kind", monitorConfig.Kind)
	}
	if reportErr != nil {
		return checkRunResult{}, reportErr
	}
	return checkRunResult{finishedAt: finishedAt, success: success, complete: complete}, nil
}
