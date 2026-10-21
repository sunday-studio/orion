package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// maxDiscoveryBody bounds one discovery report.
const maxDiscoveryBody = 512 << 10

// receiveAgentDiscovery stores what an agent found on its own host.
// @Summary      Agent: report what was found on the host
// @Description  Agent-to-Core. The agent scans its host read-only for services, containers, ports, disks and log files that could be monitored and reports the result after each scan. The latest report replaces the previous one. Core only stores and shows it; nothing is monitored until a person accepts a suggestion.
// @Tags         agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveAgentDiscovery
// @Param        agent_id  path      string                       true  "Agent ID"
// @Param        data      body      service.AgentDiscoveryInput  true  "Latest scan"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/discovery [post]
func (s *Server) receiveAgentDiscovery(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDiscoveryBody)
	var in service.AgentDiscoveryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "Invalid discovery report")
		return
	}
	if err := s.agentService.SaveAgentDiscovery(c.Param("agent_id"), in); err != nil {
		if errors.Is(err, service.ErrDiscoveryInvalid) {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.InternalError(c, "Failed to store the discovery report", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Discovery stored", gin.H{})
}

// getAgentDiscovery returns the latest scan for the Console's "Detected on this server" panel.
// @Summary      What the agent found on its host
// @Description  The latest read-only scan the agent reported: things that could be monitored, which are already monitored, which are new, and kinds that could not be scanned with the exact fix. discovery is null until the agent has reported one.
// @Tags         agents
// @Produce      json
// @ID           getAgentDiscovery
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{discovery=service.AgentDiscoveryView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/discovery [get]
func (s *Server) getAgentDiscovery(c *gin.Context) {
	if _, err := s.agentService.GetAgent(c.Param("id")); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	view, err := s.agentService.GetAgentDiscovery(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to load the discovery report", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Discovery retrieved", gin.H{"discovery": view})
}
