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

// receiveNodeDiscovery stores what a node found on its own host.
// @Summary      Node: report what was found on the host
// @Description  Node-to-Core. The node scans its host read-only for services, containers, ports, disks and log files that could be monitored and reports the result after each scan. The latest report replaces the previous one. Core only stores and shows it; nothing is monitored until a person accepts a suggestion.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveNodeDiscovery
// @Param        node_id  path      string                       true  "Node ID"
// @Param        data      body      service.NodeDiscoveryInput  true  "Latest scan"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/discovery [post]
func (s *Server) receiveNodeDiscovery(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDiscoveryBody)
	var in service.NodeDiscoveryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "Invalid discovery report")
		return
	}
	if err := s.nodeService.SaveNodeDiscovery(c.Param("node_id"), in); err != nil {
		if errors.Is(err, service.ErrDiscoveryInvalid) {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.InternalError(c, "Failed to store the discovery report", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Discovery stored", gin.H{})
}

// getNodeDiscovery returns the latest scan for the Console's "Detected on this server" panel.
// @Summary      What the node found on its host
// @Description  The latest read-only scan the node reported: things that could be monitored, which are already monitored, which are new, and kinds that could not be scanned with the exact fix. discovery is null until the node has reported one.
// @Tags         nodes
// @Produce      json
// @ID           getNodeDiscovery
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=object{discovery=service.NodeDiscoveryView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/discovery [get]
func (s *Server) getNodeDiscovery(c *gin.Context) {
	if _, err := s.nodeService.GetNode(c.Param("id")); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	view, err := s.nodeService.GetNodeDiscovery(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to load the discovery report", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Discovery retrieved", gin.H{"discovery": view})
}
