package api

import (
	"orion/core/internal/logging"
	"orion/core/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

type Server struct {
	db             *gorm.DB
	logger         *logging.Logger
	agentService   *service.AgentService
	authService    *service.AuthService
	reportService  *service.ReportService
	monitorService *service.MonitorService
	router         *gin.Engine
}

func NewServer(database *gorm.DB, logger *logging.Logger) *Server {
	agentService := service.NewAgentService(database, logger)
	authService := service.NewAuthService(database, logger)
	reportService := service.NewReportService(database, logger)
	monitorService := service.NewMonitorService(database, logger)
	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(RequestIDMiddleware(logger))

	server := &Server{
		db:             database,
		logger:         logger,
		agentService:   agentService,
		authService:    authService,
		reportService:  reportService,
		monitorService: monitorService,
		router:         router,
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

	// Version 1 API routes
	v1 := s.router.Group("/v1")
	{
		// Public routes
		public := v1.Group("/")
		{
			public.POST("/register", s.registerAgent)
		}

		// Frontend routes (no auth for now, can add later)
		frontend := v1.Group("/")
		{
			frontend.GET("/agents", s.listAgents)
			frontend.GET("/agents/:id", s.getAgentDetail)
			frontend.GET("/agents/:id/health", s.getAgentHealth)
			frontend.GET("/agents/:id/monitors", s.listMonitors)
			frontend.GET("/monitors/:id", s.getMonitorDetail)
			frontend.GET("/monitors/:id/history", s.getMonitorHistory)
			frontend.GET("/health/summary", s.getSystemHealth)
			frontend.GET("/health/issues", s.getHealthIssues)
			frontend.GET("/incidents/candidates", s.getIncidentCandidates)
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
}

func (s *Server) Start(addr string) error {
	s.logger.Info("Starting HTTP server", "address", addr)
	return s.router.Run(addr)
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
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "orion-core",
	})
}
