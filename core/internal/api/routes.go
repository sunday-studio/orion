package api

import (
	"orion/core/internal/logging"
	"orion/core/internal/service"

	"github.com/gin-gonic/gin"
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
	s.router.GET("/health", s.healthCheck)

	public := s.router.Group("/")
	{
		public.POST("/register", s.registerAgent)

	}

	protected := s.router.Group("/agents")
	protected.Use(AuthMiddleware())
	{
		protected.POST("/:agent_id/register-monitor", ValidateAgentToken(s.agentService, s.authService), s.registerMonitor)
		protected.POST("/:agent_id/report", ValidateAgentToken(s.agentService, s.authService), s.receiveReport)
	}
}

func (s *Server) Start(addr string) error {
	s.logger.Info("Starting HTTP server", "address", addr)
	return s.router.Run(addr)
}

// healthCheck returns server health status
func (s *Server) healthCheck(c *gin.Context) {
	s.logger.Debug("Health check requested")
	c.JSON(200, gin.H{
		"status":  "healthy",
		"service": "orion-core",
	})
}
