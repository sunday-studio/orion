package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Server struct {
	db                           *gorm.DB
	logger                       *utils.Logger
	cfg                          *config.Config
	nodeService                  *service.NodeService
	enrollments                  *service.EnrollmentService
	authService                  *service.AuthService
	reportService                *service.ReportService
	serviceLogService            *service.ServiceLogService
	monitorService               *service.MonitorService
	coreMonitorManagementService *service.CoreMonitorManagementService
	settingsService              *service.SettingsService
	rollupService                *service.RollupService
	archiveService               *service.ArchiveService
	workerDiagnosticsService     *service.WorkerDiagnosticsService
	runtimeDiagnosticsService    *service.RuntimeDiagnosticsService
	loginLimiter                 *RateLimiter
	registerLimiter              *RateLimiter
	testRunLimiter               *RateLimiter
	publicSubscriberLimiter      *RateLimiter
	publicStatusMailSend         func(publicStatusMailMessage) error
	lookupCNAME                  func(ctx context.Context, host string) (string, error)
	lookupHost                   func(ctx context.Context, host string) ([]string, error)
	router                       *gin.Engine
	logs                         *productLogs
	// auditRouteProbe lets tests ask which routes pass through the admin middleware.
	auditRouteProbe bool
}

func NewServer(database *gorm.DB, logger *utils.Logger, cfg *config.Config) *Server {
	nodeService := service.NewNodeService(database, logger)
	authService := service.NewAuthService(database, logger)
	reportService := service.NewReportService(database, logger, cfg)
	serviceLogService := service.NewServiceLogService(database, logger)
	monitorService := service.NewMonitorService(database, logger)
	coreMonitorManagementService := service.NewCoreMonitorManagementService(database, logger, cfg)
	settingsService := service.NewSettingsService(database, logger, cfg.DataDir)
	rollupService := service.NewRollupService(database, logger)
	archiveService := service.NewArchiveService(database, logger, cfg.DataDir)
	workerDiagnosticsService := service.NewWorkerDiagnosticsService(database, logger)
	runtimeDiagnosticsService := service.NewRuntimeDiagnosticsService(database, logger)
	reportService.SetDiagnostics(runtimeDiagnosticsService)
	router := gin.New()
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		logger.Error("Invalid ORION_TRUSTED_PROXIES; trusting no proxies", "error", err)
		_ = router.SetTrustedProxies(nil)
	}
	corsOrigins := cfg.CORSOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	}

	router.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID", "Cache-Control", "Pragma"},
		AllowCredentials: true,
	}))
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware(logger))
	router.Use(AccessLogMiddleware(logger))
	router.Use(SecurityHeadersMiddleware())
	router.Use(BodyLimitMiddleware())
	router.Use(RuntimeDiagnosticsMiddleware(runtimeDiagnosticsService))

	server := &Server{
		db:                           database,
		logger:                       logger,
		cfg:                          cfg,
		enrollments:                  service.NewEnrollmentService(database, logger),
		nodeService:                  nodeService,
		authService:                  authService,
		reportService:                reportService,
		serviceLogService:            serviceLogService,
		monitorService:               monitorService,
		coreMonitorManagementService: coreMonitorManagementService,
		settingsService:              settingsService,
		rollupService:                rollupService,
		archiveService:               archiveService,
		workerDiagnosticsService:     workerDiagnosticsService,
		runtimeDiagnosticsService:    runtimeDiagnosticsService,
		loginLimiter:                 NewRateLimiter(cfg.LoginRateLimitAttempts, time.Duration(cfg.LoginRateLimitWindowSecs)*time.Second),
		registerLimiter:              NewRateLimiter(cfg.RegisterRateLimitPerMinute, time.Minute),
		publicSubscriberLimiter:      NewRateLimiter(10, time.Minute),
		testRunLimiter:               NewRateLimiter(20, time.Minute),
		router:                       router,
	}
	server.publicStatusMailSend = server.deliverPublicStatusMail
	server.lookupCNAME = net.DefaultResolver.LookupCNAME
	server.lookupHost = net.DefaultResolver.LookupHost

	server.setupRoutes()

	server.applyInstanceSettings()
	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Swagger documentation (off unless ORION_ENABLE_SWAGGER=true)
	if s.cfg.EnableSwagger {
		s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Health check endpoint (no versioning)
	s.router.GET("/health", s.healthCheck)
	s.router.GET("/metrics", s.metricsAuthMiddleware(), s.getMetrics)

	// Alert email opt-in / unsubscribe pages (signed tokens; GET only renders, POST acts)
	s.router.GET("/alerts/email/confirm/:token", s.alertEmailPage("confirm"))
	s.router.POST("/alerts/email/confirm/:token", s.alertEmailAction("confirm"))
	s.router.GET("/alerts/email/unsubscribe/:token", s.alertEmailPage("unsubscribe"))
	s.router.POST("/alerts/email/unsubscribe/:token", s.alertEmailAction("unsubscribe"))

	// Public status page routes
	s.router.GET("/", s.getCustomDomainStatusPage)
	s.router.GET("/feed.atom", s.getCustomDomainStatusPageAtomFeed)
	s.router.GET("/status/:slug/feed.atom", s.getStatusPageAtomFeed)
	s.router.GET("/status/:slug/feed.rss", s.getStatusPageRSSFeed)
	s.router.GET("/status/:slug/subscribe", s.getPublicSubscribePage)
	s.router.POST("/status/:slug/subscribe", s.postPublicSubscribePage)
	s.router.GET("/status/:slug/badge.svg", s.getPublicStatusPageBadge)
	s.router.GET("/status/:slug/badge.json", s.getPublicStatusPageBadgeJSON)
	s.router.GET("/status/:slug/history", s.getPublicStatusPageHistory)
	s.router.GET("/status/:slug/components/:component_id/badge.svg", s.getPublicStatusPageComponentBadge)
	s.router.GET("/status/:slug/components/:component_id/uptime", s.getPublicStatusPageComponentUptime)
	s.router.GET("/status/:slug/components/:component_id/history", s.getPublicStatusPageComponentHistory)
	s.router.GET("/status/:slug/incidents/:incident_id/history", s.getPublicStatusPageIncidentHistory)
	s.router.POST("/status/:slug/subscribers", s.createPublicStatusPageSubscriber)
	s.router.GET("/status/:slug/subscribers/confirm/:token", s.confirmPublicStatusPageSubscriber)
	s.router.GET("/status/:slug/subscribers/manage/:token", s.getPublicStatusPageSubscriberPreferences)
	s.router.PUT("/status/:slug/subscribers/manage/:token", s.updatePublicStatusPageSubscriberPreferences)
	s.router.POST("/status/:slug/subscribers/manage/:token", s.postPublicSubscriberPreferencesForm)
	s.router.GET("/status/:slug/subscribers/unsubscribe/:token", s.getPublicUnsubscribePage)
	s.router.POST("/status/:slug/subscribers/unsubscribe/:token", s.unsubscribePublicStatusPageSubscriber)
	s.router.GET("/status/:slug/incidents/:incident_id", s.getPublicStatusPageIncident)
	s.router.GET("/status/:slug/incidents", s.listPublicStatusPageIncidents)
	s.router.GET("/status/:slug", s.getPublicStatusPage)

	// Version 1 API routes
	v1 := s.router.Group("/v1")
	{
		// Public routes
		public := v1.Group("/")
		{
			public.POST("/register", s.registerNode)
			public.POST("/register/preflight", s.preflightNodeRegistration)
			public.POST("/auth/login", s.login)
			public.POST("/ingest/logs", s.ingestLogs)
			public.POST("/ingest/otlp/v1/logs", s.ingestOTLPLogs)
			public.POST("/heartbeats/:token", s.receiveHeartbeatSuccess)
			public.POST("/heartbeats/:token/success", s.receiveHeartbeatSuccess)
			public.POST("/heartbeats/:token/failure", s.receiveHeartbeatFailure)
			public.POST("/heartbeat/success", s.receiveHeartbeatSuccessHeader)
			public.POST("/heartbeat/failure", s.receiveHeartbeatFailureHeader)
		}

		// Frontend routes (JWT when ORION_ADMIN_* and ORION_JWT_SECRET are set)
		frontend := v1.Group("/")
		frontend.Use(s.frontendAuthMiddleware(), s.auditMiddleware())
		s.registerSettingsRoutes(frontend)
		{
			frontend.GET("/nodes", s.listNodes)
			frontend.GET("/nodes/summary", s.getNodeSummary)
			frontend.GET("/nodes/:id", s.getNodeDetail)
			frontend.PATCH("/nodes/:id", s.updateNode)
			frontend.DELETE("/nodes/:id", s.decommissionNode)
			frontend.GET("/nodes/:id/health", s.getNodeHealth)
			frontend.GET("/nodes/:id/reports", s.getNodeReports)
			frontend.GET("/nodes/:id/service-logs", s.listNodeServiceLogs)
			frontend.GET("/nodes/:id/uptime", s.getNodeUptime)
			frontend.GET("/nodes/:id/monitors", s.listMonitors)
			frontend.GET("/nodes/:id/discovery", s.getNodeDiscovery)
			frontend.GET("/nodes/:id/app-resources", s.getNodeAppResources)
			frontend.GET("/nodes/:id/discovery/requests", s.listNodeDiscoveryRequests)
			frontend.POST("/nodes/:node_id/discovery/requests", s.createNodeDiscoveryRequest)
			frontend.DELETE("/nodes/:id/discovery/requests/:request_id", s.cancelNodeDiscoveryRequest)
			frontend.POST("/nodes/:node_id/discovery/accept", s.acceptDiscoveredItems)
			frontend.GET("/nodes/:id/monitor-definitions", s.listNodeMonitorDefinitions)
			frontend.POST("/nodes/:node_id/monitor-definitions", s.createNodeMonitorDefinition)
			frontend.DELETE("/nodes/:id/monitor-definitions/:definition_id", s.deleteNodeMonitorDefinition)
			frontend.PATCH("/nodes/:id/monitor-definitions/:definition_id", s.updateNodeMonitorDefinition)
			frontend.PATCH("/nodes/:id/monitor-definitions", s.bulkUpdateNodeMonitorDefinitions)
			frontend.GET("/nodes/:id/token/status", s.getNodeTokenStatus)
			frontend.POST("/nodes/:node_id/token/rotate", s.rotateNodeToken)
			frontend.POST("/nodes/:node_id/token/revoke", s.revokeNodeToken)
			frontend.POST("/nodes/:node_id/token/reissue", s.reissueNodeToken)
			frontend.GET("/monitors", s.listAllMonitors)
			frontend.POST("/monitors", s.createCoreMonitor)
			frontend.POST("/monitors/bulk", s.bulkMonitorAction)
			frontend.POST("/monitors/test-run", s.testRunUnsavedMonitor)
			frontend.GET("/monitors/summary", s.getMonitorSummary)
			frontend.PATCH("/monitors/:id", s.updateCoreMonitor)
			frontend.DELETE("/monitors/:id", s.deleteCoreMonitor)
			frontend.GET("/monitors/:id/config", s.getCoreMonitorConfig)
			frontend.POST("/monitors/:id/pause", s.pauseCoreMonitor)
			frontend.POST("/monitors/:id/resume", s.resumeCoreMonitor)
			frontend.POST("/monitors/:id/test", s.testCoreMonitor)
			frontend.GET("/monitors/:id", s.getMonitorDetail)
			frontend.GET("/monitors/:id/uptime", s.getMonitorUptime)
			frontend.GET("/monitors/:id/response-times", s.getMonitorResponseTimes)
			frontend.GET("/monitors/:id/availability", s.getMonitorAvailability)
			frontend.POST("/monitors/:id/clone", s.cloneCoreMonitor)
			frontend.POST("/monitors/:id/test-run", s.testRunMonitor)
			frontend.PUT("/monitors/:id/maintenance", s.setMonitorMaintenance)
			frontend.DELETE("/monitors/:id/maintenance", s.clearMonitorMaintenance)
			frontend.GET("/monitors/:id/history", s.getMonitorHistory)
			frontend.GET("/health/summary", s.getSystemHealth)
			frontend.GET("/health/issues", s.getHealthIssues)
			frontend.GET("/diagnostics/core", s.getCoreDiagnostics)
			frontend.GET("/diagnostics/core-worker", s.getCoreWorkerDiagnostics)
			frontend.GET("/incidents", s.listIncidents)
			frontend.POST("/incidents", s.declareIncident)
			frontend.POST("/incidents/bulk", s.bulkIncidentAction)
			frontend.GET("/incidents/export", s.exportIncidentsCSV)
			frontend.GET("/incidents/:id", s.getIncidentDetail)
			frontend.POST("/incidents/:id/notes", s.addIncidentNote)
			frontend.GET("/incidents/:id/public-update", s.getIncidentPublicTargets)
			frontend.POST("/incidents/:id/public-updates", s.postIncidentPublicUpdate)
			frontend.PUT("/incidents/:id/responder", s.setIncidentResponder)
			frontend.GET("/incidents/:id/timeline", s.getIncidentTimeline)
			frontend.POST("/incidents/:id/acknowledge", s.acknowledgeIncident)
			frontend.POST("/incidents/:id/resolve", s.resolveIncident)
			frontend.POST("/incidents/:id/cover", s.coverIncident)
			frontend.POST("/incidents/:id/reopen", s.reopenIncident)
			frontend.GET("/incidents/candidates", s.getIncidentCandidates)
			frontend.POST("/incidents/candidates/:id/dismiss", s.dismissIncidentCandidate)
			frontend.POST("/incidents/candidates/:id/open", s.openIncidentCandidate)
			frontend.GET("/alerts/deliveries", s.listAlertDeliveries)
			frontend.POST("/alerts/deliveries/:id/retry", s.retryAlertDelivery)
			frontend.POST("/enrollments", s.createEnrollment)
			frontend.GET("/enrollments/:id", s.getEnrollment)
			frontend.DELETE("/enrollments/:id", s.cancelEnrollment)
			frontend.GET("/overview", s.getOverview)
			frontend.GET("/search", s.searchEverything)
			frontend.GET("/auth/me", s.authMe)
			frontend.GET("/alerts/deliveries/:id", s.getAlertDelivery)
			frontend.GET("/alerts/payload-preview", s.getAlertPayloadPreview)
			frontend.POST("/alerts/channels/test-run", s.testRunAlertChannel)
			frontend.GET("/alerts/channels/:id", s.getAlertChannel)
			frontend.POST("/alerts/channels/:id/secret/regenerate", s.regenerateAlertChannelSecret)
			frontend.GET("/alerts/channels", s.listAlertChannels)
			frontend.POST("/alerts/channels", s.createAlertChannel)
			frontend.PATCH("/alerts/channels/:id", s.updateAlertChannel)
			frontend.POST("/alerts/channels/:id/test", s.testAlertChannel)
			frontend.DELETE("/alerts/channels/:id", s.deleteAlertChannel)
			frontend.GET("/alerts/channels/:id/recipients", s.listAlertChannelRecipients)
			frontend.POST("/alerts/channels/:id/recipients", s.addAlertChannelRecipient)
			frontend.DELETE("/alerts/channels/:id/recipients/:recipient_id", s.removeAlertChannelRecipient)
			frontend.POST("/alerts/channels/:id/recipients/:recipient_id/resend", s.resendAlertChannelRecipient)
			frontend.GET("/alerts/routes", s.listAlertRoutes)
			frontend.POST("/alerts/routes", s.createAlertRoute)
			frontend.POST("/alerts/routes/dry-run", s.dryRunAlertRoutes)
			frontend.PATCH("/alerts/routes/:id", s.updateAlertRoute)
			frontend.DELETE("/alerts/routes/:id", s.deleteAlertRoute)
			frontend.GET("/alerts/rules", s.listAlertRules)
			frontend.POST("/alerts/rules", s.createAlertRule)
			frontend.POST("/alerts/rules/dry-run", s.dryRunAlertRules)
			frontend.POST("/alerts/rules/:id/replay", s.replayAlertRule)
			frontend.GET("/alerts/label-routes", s.listLabelRoutes)
			frontend.POST("/alerts/label-routes", s.createLabelRoute)
			frontend.POST("/alerts/label-routes/reorder", s.reorderLabelRoutes)
			frontend.POST("/alerts/label-routes/dry-run", s.dryRunLabelRoutes)
			frontend.PATCH("/alerts/label-routes/:id", s.updateLabelRoute)
			frontend.DELETE("/alerts/label-routes/:id", s.deleteLabelRoute)
			frontend.PATCH("/alerts/rules/:id", s.updateAlertRule)
			frontend.DELETE("/alerts/rules/:id", s.deleteAlertRule)
			frontend.POST("/alerts/rules/:id/enable", s.enableAlertRule)
			frontend.POST("/alerts/rules/:id/disable", s.disableAlertRule)
			s.registerStatusPageAdminRoutes(frontend)
			frontend.GET("/events", s.listOrionEvents)
			frontend.GET("/logs/service", s.listServiceLogs)
			s.registerProductLogRoutes(frontend)
			frontend.GET("/settings/data-lifecycle", s.getDataLifecycleSettings)
			frontend.PUT("/settings/data-lifecycle", s.updateDataLifecycleSettings)
			frontend.POST("/settings/data-lifecycle/actions/rollup", s.runDataLifecycleRollup)
			frontend.POST("/settings/data-lifecycle/actions/archive", s.runDataLifecycleArchive)
		}

		// Protected routes (node-to-core)
		protected := v1.Group("/nodes")
		protected.Use(AuthMiddleware())
		{
			protected.POST("/:node_id/register-monitor", ValidateNodeToken(s.nodeService, s.authService), s.registerMonitor)
			protected.POST("/:node_id/unregister-monitor", ValidateNodeToken(s.nodeService, s.authService), s.unregisterMonitor)
			protected.POST("/:node_id/report", ValidateNodeToken(s.nodeService, s.authService), s.receiveNodeReport)
			protected.POST("/:node_id/logs/batch", ValidateNodeToken(s.nodeService, s.authService), s.receiveNodeLogBatch)
			protected.POST("/:node_id/:monitor_id/report", ValidateNodeToken(s.nodeService, s.authService), s.receiveMonitorReport)
			protected.POST("/:node_id/discovery", ValidateNodeToken(s.nodeService, s.authService), s.receiveNodeDiscovery)
			protected.POST("/:node_id/discovery/requests/pull", ValidateNodeToken(s.nodeService, s.authService), s.pullNodeDiscoveryRequests)
			protected.POST("/:node_id/discovery/requests/:request_id/result", ValidateNodeToken(s.nodeService, s.authService), s.finishNodeDiscoveryRequest)
			protected.POST("/:node_id/monitor-definitions/results", ValidateNodeToken(s.nodeService, s.authService), s.recordNodeDefinitionResults)
			protected.POST("/:node_id/monitor-definitions/sync", ValidateNodeToken(s.nodeService, s.authService), s.pullNodeMonitorDefinitions)
			protected.POST("/:node_id/token-check", ValidateNodeToken(s.nodeService, s.authService), s.checkNodeToken)
			protected.PUT("/:node_id/maintenance", ValidateNodeToken(s.nodeService, s.authService), s.setMaintenanceMode)
		}
	}

	s.router.NoRoute(func(c *gin.Context) {
		s.serveConsole(c)
	})
}

func (s *Server) serveConsole(c *gin.Context) {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Status(http.StatusNotFound)
		return
	}

	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/v1/") || path == "/v1" || strings.HasPrefix(path, "/swagger/") {
		c.Status(http.StatusNotFound)
		return
	}

	webDir := "web"
	indexPath := filepath.Join(webDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	cleanPath := filepath.Clean(strings.TrimPrefix(path, "/"))
	if cleanPath != "." && cleanPath != "" && !strings.HasPrefix(cleanPath, "..") {
		filePath := filepath.Join(webDir, cleanPath)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			c.File(filePath)
			return
		}
	}

	c.File(indexPath)
}

func (s *Server) Start(ctx context.Context, addr string) error {
	s.logger.Info("Starting HTTP server", "address", addr)
	// Request contexts derive from baseCtx, so cancelling it ends long-lived responses
	// (the live log tail) that http.Server.Shutdown would otherwise wait on forever.
	baseCtx, cancelStreams := context.WithCancel(context.Background())
	defer cancelStreams()
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		BaseContext:       func(net.Listener) context.Context { return baseCtx },
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go s.runMaintenanceNoticeLoop(baseCtx)
	go s.runExportCleanupLoop(baseCtx)

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.logger.Info("Shutting down HTTP server")
		cancelStreams()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			// Slow clients outlasting the grace period are cut off; that is not a startup failure.
			s.logger.Warn("HTTP server did not drain within the grace period; closing remaining connections", "error", err)
			_ = httpServer.Close()
		}
		return <-errCh
	}
}

// RequestIDMiddleware generates a unique request ID for each request
func RequestIDMiddleware(logger *utils.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Reuse a caller-supplied id only if it is short and safe (tracing across services).
		requestID := requestIDFrom(c.GetHeader("X-Request-ID"))

		// Set request ID in context
		c.Set("request_id", requestID)

		// Add request ID to response header
		c.Header("X-Request-ID", requestID)

		logger.Debug("Request received", "request_id", requestID, "method", c.Request.Method)

		c.Next()
	}
}

func RuntimeDiagnosticsMiddleware(diagnostics *service.RuntimeDiagnosticsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched" // bounded label: raw paths would grow the metrics map without limit
		}
		diagnostics.RecordRequest(route, c.Writer.Status(), time.Since(startedAt))
	}
}

// healthCheck returns server health status
// @Summary      Health check
// @Description  Returns the health status of the API server
// @Tags         health
// @Accept       json
// @Produce      json
// @ID           getHealth
// @Success      200  {object}  object{status=string,service=string}
// @Router       /health [get]
func (s *Server) healthCheck(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	s.logger.Debug("Health check requested", "request_id", requestID)
	sqlDB, err := s.db.DB()
	if err != nil {
		s.logger.Error("Database health check failed", "request_id", requestID, "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"service":  "orion-core",
			"database": "unavailable",
		})
		return
	}
	if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		s.logger.Error("Database ping failed", "request_id", requestID, "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":   "unhealthy",
			"service":  "orion-core",
			"database": "unavailable",
		})
		return
	}
	c.JSON(200, gin.H{
		"status":   "healthy",
		"service":  "orion-core",
		"database": "ok",
	})
}
