package api

import "github.com/gin-gonic/gin"

// registerTraceRoutes wires the trace read routes (admin JWT). The ingest route is a public-group
// route authenticated by ingest key, next to the other OTLP routes.
func (s *Server) registerTraceRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/traces", s.listTraces)
	frontend.GET("/traces/facets", s.getTraceFacets)
	frontend.GET("/traces/:trace_id", s.getTrace)
	frontend.GET("/services/:id/traces/summary", s.getServiceTraceSummary)
}
