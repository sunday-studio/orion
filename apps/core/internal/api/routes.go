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
	db              *gorm.DB
	logger          *logging.Logger
	cfg             *config.Config
	agentService    *service.AgentService
	authService     *service.AuthService
	reportService   *service.ReportService
	monitorService  *service.MonitorService
	settingsService *service.SettingsService
	rollupService   *service.RollupService
	archiveService  *service.ArchiveService
	loginLimiter    *RateLimiter
	router          *gin.Engine
}

func NewServer(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *Server {
	agentService := service.NewAgentService(database, logger)
	authService := service.NewAuthService(database, logger)
	reportService := service.NewReportService(database, logger, cfg)
	monitorService := service.NewMonitorService(database, logger)
	settingsService := service.NewSettingsService(database, logger, cfg.DataDir)
	rollupService := service.NewRollupService(database, logger)
	archiveService := service.NewArchiveService(database, logger, cfg.DataDir)
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

	server := &Server{
		db:              database,
		logger:          logger,
		cfg:             cfg,
		agentService:    agentService,
		authService:     authService,
		reportService:   reportService,
		monitorService:  monitorService,
		settingsService: settingsService,
		rollupService:   rollupService,
		archiveService:  archiveService,
		loginLimiter:    NewRateLimiter(cfg.LoginRateLimitAttempts, time.Duration(cfg.LoginRateLimitWindowSecs)*time.Second),
		router:          router,
	}

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
	s.router.GET("/status/:slug/feed.atom", s.getStatusPageAtomFeed)
	s.router.GET("/status/:slug/history", s.getPublicStatusPageHistory)
	s.router.GET("/status/:slug/components/:component_id/uptime", s.getPublicStatusPageComponentUptime)
	s.router.GET("/status/:slug/components/:component_id/history", s.getPublicStatusPageComponentHistory)
	s.router.GET("/status/:slug/incidents/:incident_id/history", s.getPublicStatusPageIncidentHistory)
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
			frontend.GET("/agents/:id/uptime", s.getAgentUptime)
			frontend.GET("/agents/:id/monitors", s.listMonitors)
			frontend.GET("/monitors", s.listAllMonitors)
			frontend.GET("/monitors/summary", s.getMonitorSummary)
			frontend.GET("/monitors/:id", s.getMonitorDetail)
			frontend.GET("/monitors/:id/uptime", s.getMonitorUptime)
			frontend.GET("/monitors/:id/history", s.getMonitorHistory)
			frontend.GET("/health/summary", s.getSystemHealth)
			frontend.GET("/health/issues", s.getHealthIssues)
			frontend.GET("/incidents", s.listIncidents)
			frontend.GET("/incidents/:id", s.getIncidentDetail)
			frontend.GET("/incidents/:id/timeline", s.getIncidentTimeline)
			frontend.GET("/incidents/candidates", s.getIncidentCandidates)
			frontend.GET("/alerts/deliveries", s.listAlertDeliveries)
			frontend.GET("/alerts/channels", s.listAlertChannels)
			frontend.POST("/alerts/channels", s.createAlertChannel)
			frontend.PATCH("/alerts/channels/:id", s.updateAlertChannel)
			frontend.POST("/alerts/channels/:id/test", s.testAlertChannel)
			frontend.DELETE("/alerts/channels/:id", s.deleteAlertChannel)
			frontend.GET("/alerts/smtp-services", s.listAlertSMTPServices)
			frontend.POST("/alerts/smtp-services", s.createAlertSMTPService)
			frontend.PATCH("/alerts/smtp-services/:id", s.updateAlertSMTPService)
			frontend.POST("/alerts/smtp-services/:id/test", s.testAlertSMTPService)
			frontend.DELETE("/alerts/smtp-services/:id", s.deleteAlertSMTPService)
			frontend.GET("/alerts/email-destinations", s.listAlertEmailDestinations)
			frontend.POST("/alerts/email-destinations", s.createAlertEmailDestination)
			frontend.PATCH("/alerts/email-destinations/:id", s.updateAlertEmailDestination)
			frontend.POST("/alerts/email-destinations/:id/test", s.testAlertEmailDestination)
			frontend.DELETE("/alerts/email-destinations/:id", s.deleteAlertEmailDestination)
			frontend.GET("/alerts/routes", s.listAlertRoutes)
			frontend.POST("/alerts/routes", s.createAlertRoute)
			frontend.POST("/alerts/routes/dry-run", s.dryRunAlertRoutes)
			frontend.PATCH("/alerts/routes/:id", s.updateAlertRoute)
			frontend.DELETE("/alerts/routes/:id", s.deleteAlertRoute)
			frontend.GET("/alerts/rules", s.listAlertRules)
			s.registerStatusPageAdminRoutes(frontend)
			frontend.GET("/events", s.listOrionEvents)
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
