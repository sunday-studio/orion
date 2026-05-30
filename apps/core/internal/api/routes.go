package api

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Server struct {
	db                           *gorm.DB
	logger                       *logging.Logger
	cfg                          *config.Config
	agentService                 *service.AgentService
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
	publicSubscriberLimiter      *RateLimiter
	publicStatusMailSend         func(publicStatusMailMessage) error
	router                       *gin.Engine
}

func NewServer(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *Server {
	agentService := service.NewAgentService(database, logger)
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
	router := gin.Default()
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
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware(logger))
	router.Use(RuntimeDiagnosticsMiddleware(runtimeDiagnosticsService))

	server := &Server{
		db:                           database,
		logger:                       logger,
		cfg:                          cfg,
		agentService:                 agentService,
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
		publicSubscriberLimiter:      NewRateLimiter(10, time.Minute),
		router:                       router,
	}
	server.publicStatusMailSend = server.deliverPublicStatusMail

	server.setupRoutes()

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Swagger documentation
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Health check endpoint (no versioning)
	s.router.GET("/health", s.healthCheck)

	// Public status page routes
	s.router.GET("/", s.getCustomDomainStatusPage)
	s.router.GET("/feed.atom", s.getCustomDomainStatusPageAtomFeed)
	s.router.GET("/status/:slug/feed.atom", s.getStatusPageAtomFeed)
	s.router.GET("/status/:slug/badge.svg", s.getPublicStatusPageBadge)
	s.router.GET("/status/:slug/history", s.getPublicStatusPageHistory)
	s.router.GET("/status/:slug/components/:component_id/badge.svg", s.getPublicStatusPageComponentBadge)
	s.router.GET("/status/:slug/components/:component_id/uptime", s.getPublicStatusPageComponentUptime)
	s.router.GET("/status/:slug/components/:component_id/history", s.getPublicStatusPageComponentHistory)
	s.router.GET("/status/:slug/incidents/:incident_id/history", s.getPublicStatusPageIncidentHistory)
	s.router.POST("/status/:slug/subscribers", s.createPublicStatusPageSubscriber)
	s.router.GET("/status/:slug/subscribers/confirm/:token", s.confirmPublicStatusPageSubscriber)
	s.router.GET("/status/:slug/subscribers/manage/:token", s.getPublicStatusPageSubscriberPreferences)
	s.router.PUT("/status/:slug/subscribers/manage/:token", s.updatePublicStatusPageSubscriberPreferences)
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
			public.POST("/register", s.registerAgent)
			public.POST("/auth/login", s.login)
			public.POST("/heartbeats/:token", s.receiveHeartbeatSuccess)
			public.POST("/heartbeats/:token/success", s.receiveHeartbeatSuccess)
			public.POST("/heartbeats/:token/failure", s.receiveHeartbeatFailure)
		}

		// Frontend routes (JWT when ORION_ADMIN_* and ORION_JWT_SECRET are set)
		frontend := v1.Group("/")
		frontend.Use(s.frontendAuthMiddleware())
		{
			frontend.GET("/agents", s.listAgents)
			frontend.GET("/agents/summary", s.getAgentSummary)
			frontend.GET("/agents/:id", s.getAgentDetail)
			frontend.GET("/agents/:id/health", s.getAgentHealth)
			frontend.GET("/agents/:id/reports", s.getAgentReports)
			frontend.GET("/agents/:id/service-logs", s.listAgentServiceLogs)
			frontend.GET("/agents/:id/uptime", s.getAgentUptime)
			frontend.GET("/agents/:id/monitors", s.listMonitors)
			frontend.GET("/agents/:id/token/status", s.getAgentTokenStatus)
			frontend.POST("/agents/:agent_id/token/rotate", s.rotateAgentToken)
			frontend.POST("/agents/:agent_id/token/revoke", s.revokeAgentToken)
			frontend.POST("/agents/:agent_id/token/reissue", s.reissueAgentToken)
			frontend.GET("/monitors", s.listAllMonitors)
			frontend.POST("/monitors", s.createCoreMonitor)
			frontend.GET("/monitors/summary", s.getMonitorSummary)
			frontend.PATCH("/monitors/:id", s.updateCoreMonitor)
			frontend.DELETE("/monitors/:id", s.deleteCoreMonitor)
			frontend.GET("/monitors/:id/config", s.getCoreMonitorConfig)
			frontend.POST("/monitors/:id/pause", s.pauseCoreMonitor)
			frontend.POST("/monitors/:id/resume", s.resumeCoreMonitor)
			frontend.POST("/monitors/:id/test", s.testCoreMonitor)
			frontend.GET("/monitors/:id", s.getMonitorDetail)
			frontend.GET("/monitors/:id/uptime", s.getMonitorUptime)
			frontend.GET("/monitors/:id/history", s.getMonitorHistory)
			frontend.GET("/health/summary", s.getSystemHealth)
			frontend.GET("/health/issues", s.getHealthIssues)
			frontend.GET("/diagnostics/core", s.getCoreDiagnostics)
			frontend.GET("/diagnostics/core-worker", s.getCoreWorkerDiagnostics)
			frontend.GET("/incidents", s.listIncidents)
			frontend.GET("/incidents/:id", s.getIncidentDetail)
			frontend.GET("/incidents/:id/timeline", s.getIncidentTimeline)
			frontend.POST("/incidents/:id/acknowledge", s.acknowledgeIncident)
			frontend.POST("/incidents/:id/resolve", s.resolveIncident)
			frontend.POST("/incidents/:id/cover", s.coverIncident)
			frontend.POST("/incidents/:id/reopen", s.reopenIncident)
			frontend.GET("/incidents/candidates", s.getIncidentCandidates)
			frontend.GET("/alerts/deliveries", s.listAlertDeliveries)
			frontend.GET("/alerts/channels", s.listAlertChannels)
			frontend.POST("/alerts/channels", s.createAlertChannel)
			frontend.PATCH("/alerts/channels/:id", s.updateAlertChannel)
			frontend.POST("/alerts/channels/:id/test", s.testAlertChannel)
			frontend.DELETE("/alerts/channels/:id", s.deleteAlertChannel)
			frontend.GET("/alerts/routes", s.listAlertRoutes)
			frontend.POST("/alerts/routes", s.createAlertRoute)
			frontend.POST("/alerts/routes/dry-run", s.dryRunAlertRoutes)
			frontend.PATCH("/alerts/routes/:id", s.updateAlertRoute)
			frontend.DELETE("/alerts/routes/:id", s.deleteAlertRoute)
			frontend.GET("/alerts/rules", s.listAlertRules)
			frontend.POST("/alerts/rules", s.createAlertRule)
			frontend.POST("/alerts/rules/dry-run", s.dryRunAlertRules)
			frontend.PATCH("/alerts/rules/:id", s.updateAlertRule)
			frontend.DELETE("/alerts/rules/:id", s.deleteAlertRule)
			frontend.POST("/alerts/rules/:id/enable", s.enableAlertRule)
			frontend.POST("/alerts/rules/:id/disable", s.disableAlertRule)
			s.registerStatusPageAdminRoutes(frontend)
			frontend.GET("/events", s.listOrionEvents)
			frontend.GET("/logs/service", s.listServiceLogs)
			frontend.GET("/settings/data-lifecycle", s.getDataLifecycleSettings)
			frontend.PUT("/settings/data-lifecycle", s.updateDataLifecycleSettings)
			frontend.POST("/settings/data-lifecycle/actions/rollup", s.runDataLifecycleRollup)
			frontend.POST("/settings/data-lifecycle/actions/archive", s.runDataLifecycleArchive)
		}

		// Protected routes (agent-to-core)
		protected := v1.Group("/agents")
		protected.Use(AuthMiddleware())
		{
			protected.POST("/:agent_id/register-monitor", ValidateAgentToken(s.agentService, s.authService), s.registerMonitor)
			protected.POST("/:agent_id/unregister-monitor", ValidateAgentToken(s.agentService, s.authService), s.unregisterMonitor)
			protected.POST("/:agent_id/report", ValidateAgentToken(s.agentService, s.authService), s.receiveAgentReport)
			protected.POST("/:agent_id/logs/batch", ValidateAgentToken(s.agentService, s.authService), s.receiveAgentLogBatch)
			protected.POST("/:agent_id/:monitor_id/report", ValidateAgentToken(s.agentService, s.authService), s.receiveMonitorReport)
			protected.PUT("/:agent_id/maintenance", ValidateAgentToken(s.agentService, s.authService), s.setMaintenanceMode)
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
	httpServer := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

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
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}

// RequestIDMiddleware generates a unique request ID for each request
func RequestIDMiddleware(logger *logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already in header (for tracing across services)
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate new UUID if not present
			requestID = uuid.New().String()
		}

		// Set request ID in context
		c.Set("request_id", requestID)

		// Add request ID to response header
		c.Header("X-Request-ID", requestID)

		// Log request with request ID
		logger.Debug("Request received", "request_id", requestID, "method", c.Request.Method, "path", c.Request.URL.Path)

		c.Next()
	}
}

func RuntimeDiagnosticsMiddleware(diagnostics *service.RuntimeDiagnosticsService) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
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
