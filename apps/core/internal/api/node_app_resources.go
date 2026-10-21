package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// getNodeAppResources returns what the node's host spends per app.
// @Summary      What apps cost on a server
// @Description  The latest passive measurements the node reported: CPU, memory, disk and network per process group, systemd unit and container, plus storage sizes for tracked paths, Docker volumes, images and build cache. app_resources is null until the node has reported some. Core keeps only the latest report; fields the node could not measure are absent.
// @Tags         nodes
// @Produce      json
// @ID           getNodeAppResources
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=object{app_resources=service.NodeAppResourcesView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/app-resources [get]
func (s *Server) getNodeAppResources(c *gin.Context) {
	if _, err := s.nodeService.GetNode(c.Param("id")); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	var view *service.NodeAppResourcesView
	view, err := s.nodeService.GetAppResources(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to load app resources", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "App resources retrieved", gin.H{"app_resources": view})
}
