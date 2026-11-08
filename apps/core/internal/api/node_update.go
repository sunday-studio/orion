package api

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/buildinfo"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// requestNodeUpdate asks one node to update itself to the release Core runs.
// @Summary      Update a node from the Console
// @Description  Queues an update_node request that tells the node to update itself to the release this Core runs (nodes should never be newer than Core, so update Core first). The node downloads the file for its own platform from the GitHub release and installs it only if the checksum Core read from the release's SHA256SUMS matches the file and the release's own list. The node must advertise the self_update capability (a release build, not in a container, installed where it can replace its own binary, with remote_update on). Follow the request with GET /v1/nodes/{id}/discovery/requests: the node reports applied once the new version has stayed up, or failed with the reason.
// @Tags         nodes
// @Produce      json
// @ID           requestNodeUpdate
// @Param        node_id  path      string  true  "Node ID"
// @Success      201  {object}  utils.APIResponse{data=object{request=service.NodeDiscoveryRequestView}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Failure      502  {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/update [post]
func (s *Server) requestNodeUpdate(c *gin.Context) {
	node, err := s.nodeService.GetNode(c.Param("node_id"))
	if err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	version := buildinfo.Version
	if _, ok := service.CompareVersions(version, version); !ok {
		utils.ErrorResponse(c, http.StatusConflict, "This Core is not a release build, so there is no release to update the node to", nil)
		return
	}
	asset, err := service.NodeAssetName(node.OS, node.Arch)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	tag, err := s.releases.ReleaseTag(ctx, version)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadGateway, "Could not find release "+version+": "+err.Error(), nil)
		return
	}
	sums, err := s.releases.Checksums(ctx, tag)
	if err != nil {
		utils.ErrorResponse(c, http.StatusBadGateway, "Could not read the checksums of release "+version+": "+err.Error(), nil)
		return
	}
	sum := sums[asset]
	if sum == "" {
		utils.ErrorResponse(c, http.StatusBadGateway, "Release "+version+" has no file named "+asset, nil)
		return
	}
	v, err := s.nodeService.RequestNodeUpdate(node.ID, service.NodeUpdateTarget{Version: version, Tag: tag, SHA256: sum}, frontendActorID(c))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusCreated, "Update queued", gin.H{"request": v})
	case errors.Is(err, service.ErrNodeNotFound):
		utils.NotFound(c, "Node not found")
	case errors.Is(err, service.ErrNodeCannotSelfUpdate), errors.Is(err, service.ErrNodeVersionUnknown):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrNodeUpToDate), errors.Is(err, service.ErrUpdateInFlight):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to queue the update", err)
	}
}
