package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// getAgentAppResources returns what the agent's host spends per app.
// @Summary      What apps cost on a server
// @Description  The latest passive measurements the agent reported: CPU, memory, disk and network per process group, systemd unit and container, plus storage sizes for tracked paths, Docker volumes, images and build cache. app_resources is null until the agent has reported some. Core keeps only the latest report; fields the agent could not measure are absent.
// @Tags         agents
// @Produce      json
// @ID           getAgentAppResources
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{app_resources=service.AgentAppResourcesView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/app-resources [get]
func (s *Server) getAgentAppResources(c *gin.Context) {
	if _, err := s.agentService.GetAgent(c.Param("id")); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	var view *service.AgentAppResourcesView
	view, err := s.agentService.GetAppResources(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to load app resources", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "App resources retrieved", gin.H{"app_resources": view})
}
