package api

import (
	"orion/core/internal/logging"
	"orion/core/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Server represents the HTTP server
type Server struct {
	db            *gorm.DB
	logger        *logging.Logger
	agentService  *service.AgentService
	authService   *service.AuthService
	reportService *service.ReportService
	router        *gin.Engine
}

// NewServer creates a new HTTP server instance
func NewServer(database *gorm.DB, logger *logging.Logger) *Server {
	// Initialize services
	agentService := service.NewAgentService(database, logger)
	authService := service.NewAuthService(database, logger)
	reportService := service.NewReportService(database, logger)

	// Create Gin router
	router := gin.Default()

	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	server := &Server{
		db:            database,
		logger:        logger,
		agentService:  agentService,
		authService:   authService,
		reportService: reportService,
		router:        router,
	}

	// Setup routes
	server.setupRoutes()

	return server
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)

	// Public routes
	public := s.router.Group("/")
	{
		public.POST("/register", s.registerAgent)
	}

	// Protected routes (require authentication)
	protected := s.router.Group("/")
	protected.Use(AuthMiddleware())
	{
		protected.POST("/report/:agent_id", ValidateAgentToken(s.agentService, s.authService), s.receiveReport)
	}
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	s.logger.Info("Starting HTTP server", "address", addr)
	return s.router.Run(addr)
}

// healthCheck returns server health status
func (s *Server) healthCheck(c *gin.Context) {
	s.logger.Debug("Health check requested")
	c.JSON(200, gin.H{
		"status": "healthy",
		"service": "orion-core",
	})
}
