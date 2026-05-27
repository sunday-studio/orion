package worker

import (
	"context"
	"errors"
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
	defaultHealthInterval   = 30 * time.Second
	defaultCheckInterval    = 5 * time.Second
	defaultLeaseDuration    = 2 * time.Minute
	defaultHTTPTimeout      = 10 * time.Second
	defaultCoreWorkerID     = "core-monitor-worker"
	httpStatusRunnerKind    = "http"
	httpStatusRunnerName    = "http_status"
	tcpRunnerKind           = "tcp"
	tcpRunnerName           = "tcp_port"
	dnsRunnerKind           = "dns"
	tlsRunnerKind           = "tls"
	tlsRunnerName           = "tls_certificate"
	udpRunnerKind           = "udp"
	apiRequestRunnerKind    = "api_request"
	domainExpirationKind    = "domain_expiration"
	pingRunnerKind          = "ping"
	mailRunnerKind          = "mail"
	smtpRunnerKind          = "smtp"
	imapRunnerKind          = "imap"
	popRunnerKind           = "pop"
	pop3RunnerKind          = "pop3"
	syntheticRunnerKind     = "synthetic"
	syntheticRunnerName     = "synthetic_multi_step"
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
	DNSResolver    dnsResolver
	TLSCheck       tlsCheckFunc
	UDPDialContext dialContextFunc
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
	dnsResolver := opts.DNSResolver
	if dnsResolver == nil {
		dnsResolver = net.DefaultResolver
	}
	tlsCheck := opts.TLSCheck
	if tlsCheck == nil {
		tlsCheck = defaultTLSCheck
	}
	tlsWarningDays := defaultTLSWarningDays
	if opts.Config != nil && opts.Config.AlertTLSExpiryDays > 0 {
		tlsWarningDays = opts.Config.AlertTLSExpiryDays
	}
	udpDialContext := opts.UDPDialContext
	if udpDialContext == nil {
		udpDialer := &net.Dialer{}
		udpDialContext = udpDialer.DialContext
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
