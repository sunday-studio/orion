package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"sync"
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
	// DispatchInterval controls how often due alert deliveries are sent (default 15s).
	DispatchInterval time.Duration
	// SweepInterval controls how often silent agent monitors are checked (default 30s).
	SweepInterval time.Duration
	// SweepBootGrace delays the first silent-monitor sweep after Worker start so a
	// Core outage does not make every monitor look silent at once (default 60s, negative disables).
	SweepBootGrace time.Duration
	// Concurrency bounds how many Core monitor checks run at once (default 16).
	Concurrency int
}

// App is the independent Core monitor worker process.
type App struct {
	db             *gorm.DB
	logger         *utils.Logger
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
	alerts         *service.AlertService
	incidents      *service.IncidentService
	sweepEvery     time.Duration
	sweepBootGrace time.Duration
	startedAt      time.Time
	dispatchEvery  time.Duration
	concurrency    int
	slots          chan struct{}
	inflight       sync.WaitGroup
}

type checkRunResult struct {
	finishedAt time.Time
	success    bool
	complete   bool
}

// NewApp creates a worker app bound to the Core database.
func NewApp(database *gorm.DB, logger *utils.Logger, opts Options) *App {
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
		alerts:         service.NewAlertService(database, logger, opts.Config),
		incidents:      service.NewIncidentService(database, logger, opts.Config),
		sweepEvery:     sweepInterval(opts.SweepInterval),
		sweepBootGrace: sweepBootGrace(opts.SweepBootGrace),
		startedAt:      time.Now().UTC(),
		dispatchEvery:  dispatchInterval(opts.DispatchInterval),
		concurrency:    checkConcurrency(opts.Concurrency),
		slots:          make(chan struct{}, checkConcurrency(opts.Concurrency)),
	}
}

// Run logs worker health and executes due Core monitor checks until the context is canceled.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Core monitor worker started", "worker_id", a.workerID, "health_interval", a.healthInterval.String(), "check_interval", a.checkInterval.String())
	a.logHealth(ctx)
	// Checks run on a context that outlives ctx: on shutdown they get a grace
	// period to finish instead of being cut off and recorded as failures.
	execCtx, cancelChecks := context.WithCancel(context.Background())
	defer cancelChecks()
	defer a.drainChecks(cancelChecks)
	if err := a.dispatchDueChecks(ctx, execCtx); err != nil {
		a.logger.Error("Core monitor worker due check sweep failed", "error", err)
	}
	if err := a.reconcileMissedHeartbeats(ctx, time.Now().UTC()); err != nil {
		a.logger.Error("Core monitor worker heartbeat reconciliation failed", "error", err)
	}

	healthTicker := time.NewTicker(a.healthInterval)
	defer healthTicker.Stop()
	checkTicker := time.NewTicker(a.checkInterval)
	defer checkTicker.Stop()
	dispatchTicker := time.NewTicker(a.dispatchEvery)
	defer dispatchTicker.Stop()
	sweepTicker := time.NewTicker(a.sweepEvery)
	defer sweepTicker.Stop()
	a.dispatchAlerts()
	a.sweepSilentMonitors()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Core monitor worker shutting down", "reason", ctx.Err().Error())
			return nil
		case <-healthTicker.C:
			a.logHealth(ctx)
		case <-dispatchTicker.C:
			a.dispatchAlerts()
		case <-sweepTicker.C:
			a.sweepSilentMonitors()
		case <-checkTicker.C:
			if err := a.dispatchDueChecks(ctx, execCtx); err != nil {
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
	updates := map[string]any{
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
