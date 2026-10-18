package api

import "github.com/gin-gonic/gin"

// registerProductLogRoutes wires the product-logs management and read routes
// (admin JWT). Ingest routes are public-group routes authenticated by ingest key.
func (s *Server) registerProductLogRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/apps", s.listApps)
	frontend.POST("/apps", s.createApp)
	frontend.GET("/apps/:id", s.getApp)
	frontend.PATCH("/apps/:id", s.updateApp)
	frontend.DELETE("/apps/:id", s.deleteApp)
	frontend.GET("/apps/:id/keys", s.listIngestKeys)
	frontend.POST("/apps/:id/keys", s.createIngestKey)
	frontend.DELETE("/apps/:id/keys/:key_id", s.revokeIngestKey)
	frontend.POST("/apps/:id/test-log", s.sendAppTestLog)
	frontend.POST("/apps/:id/keys/:key_id/rotate", s.rotateIngestKey)
	frontend.POST("/apps/:id/purge", s.purgeAppLogs)
	frontend.POST("/apps/:id/redaction/test", s.testAppRedaction)

	frontend.GET("/logs", s.searchLogs)
	frontend.GET("/logs/histogram", s.getLogHistogram)
	frontend.GET("/logs/fields", s.getLogFieldCatalog)
	frontend.GET("/logs/stream", s.streamLogs)
	frontend.GET("/logs/views", s.listLogViews)
	frontend.POST("/logs/views", s.createLogView)
	frontend.DELETE("/logs/views/:id", s.deleteLogView)
	frontend.GET("/logs/alerts", s.listLogAlertRules)
	frontend.POST("/logs/alerts", s.createLogAlertRule)
	frontend.PATCH("/logs/alerts/:id", s.updateLogAlertRule)
	frontend.DELETE("/logs/alerts/:id", s.deleteLogAlertRule)
	frontend.GET("/incidents/:id/logs", s.getIncidentLogs)
	frontend.GET("/logs/facets", s.getLogFacets)
	frontend.GET("/logs/patterns", s.getLogPatterns)
	frontend.GET("/logs/ingest-status", s.getLogIngestStatus)
	frontend.GET("/logs/redaction-defaults", s.getRedactionDefaults)
	frontend.POST("/logs/export", s.createExport)
	frontend.GET("/exports/:id", s.getExport)
	frontend.GET("/exports/:id/download", s.downloadExport)
	frontend.GET("/logs/:id/related", s.getLogRelated)
	frontend.GET("/logs/:id", s.getLogEntry)
}
