package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// listAllMonitors retrieves a paginated list of monitors across nodes.
// @Summary      List monitors
// @Description  Get a paginated list of all monitors with optional filters
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitors
// @Param        limit          query     int     false  "Maximum number of monitors to return" default(50)
// @Param        offset         query     int     false  "Number of monitors to skip" default(0)
// @Param        search         query     string  false  "Search by monitor, node, or type"
// @Param        health         query     string  false  "Filter by health status (up|down|degraded|unknown)"
// @Param        type           query     string  false  "Filter by monitor type"
// @Param        owner_kind     query     string  false  "Filter by owner kind (node|core)"
// @Param        owner_name     query     string  false  "Filter by owner name, id, or machine id"
// @Param        source         query     string  false  "Filter by monitor source (node|core)"
// @Param        lifecycle      query     string  false  "Filter by lifecycle status (active|disabled|deleted)"
// @Param        stale_only     query     bool    false  "Only return stale monitors"
// @Param        has_incidents  query     bool    false  "Only return monitors with active incidents"
// @Param        sort           query     string  false  "Sort column" default(updated_at)
// @Param        order          query     string  false  "Sort order (asc|desc)" default(desc)
// @Param        include_system query  bool    false  "Include Core self-monitors such as worker liveness (default false)"
// @Param        status     query     string  false  "Comma-separated computed statuses to show: up,down,degraded,stale,unknown; total and hidden_count report what the filter hides"
// @Param        node_id   query     string  false  "Only monitors owned by this server"
// @Param        q          query     string  false  "Alias of search"
// @Param        paused     query     bool    false  "Only paused (true) or only running (false) Core monitors"
// @Success      200            {object}  utils.APIResponse{data=object{monitors=[]MonitorResponse,count=int64,total=int64,hidden_count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/monitors [get]
func (s *Server) listAllMonitors(c *gin.Context) {
	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)

	search := c.Query("search")
	if search == "" {
		search = c.Query("q")
	}
	var paused *bool
	if c.Query("paused") != "" {
		v := utils.QueryBool(c, "paused", false)
		paused = &v
	}
	monitors, count, total, err := s.monitorService.ListAllMonitorsPage(service.ListAllMonitorsOpts{IncludeSystem: c.Query("include_system") == "true",
		Statuses: parseStatusSet(c.Query("status")), NodeID: strings.TrimSpace(c.Query("node_id")), Paused: paused,
		Limit:        limit,
		Offset:       offset,
		Search:       search,
		Health:       c.Query("health"),
		Type:         c.Query("type"),
		OwnerKind:    c.Query("owner_kind"),
		OwnerName:    c.Query("owner_name"),
		Source:       c.Query("source"),
		Lifecycle:    c.Query("lifecycle"),
		StaleOnly:    c.Query("stale_only") == "true",
		HasIncidents: c.Query("has_incidents") == "true",
		Sort:         c.DefaultQuery("sort", "updated_at"),
		Order:        c.DefaultQuery("order", "desc"),
	})
	if err != nil {
		s.logger.Error("Failed to list all monitors", "error", err)
		utils.InternalError(c, "Failed to list monitors", err)
		return
	}

	responses := monitorResponsesWithNodes(monitors, s.monitorNodesByID(monitors), s.coreMonitorIDsByID(monitors))
	s.enrichMonitorRows(responses, monitors)
	utils.SuccessResponse(c, 200, "Monitors retrieved successfully", gin.H{
		"monitors":     responses,
		"total":        total,
		"hidden_count": total - count,
		"count":        count,
		"limit":        limit,
		"offset":       offset,
		"pagination":   utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}
