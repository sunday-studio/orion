package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) serviceCatalog() *service.ServiceCatalog {
	var names service.ServiceNameSource
	if s.logs != nil { // a typed nil must not become a non-nil interface
		names = s.logs.store
	}
	return service.NewServiceCatalog(s.db, names)
}

// respondServiceError answers a catalog failure: the caller's mistakes keep their message, anything else is a 500.
func respondServiceError(c *gin.Context, err error, internalMessage string) {
	var se *service.ServiceError
	if errors.As(err, &se) {
		switch se.Kind {
		case service.ServiceErrNotFound:
			utils.NotFound(c, se.Message)
		case service.ServiceErrConflict:
			utils.ErrorResponse(c, http.StatusConflict, se.Message, nil)
		default:
			utils.BadRequest(c, se.Message)
		}
		return
	}
	utils.InternalError(c, internalMessage, err)
}

func (s *Server) listServicesImpl(c *gin.Context) {
	services, err := s.serviceCatalog().List()
	if err != nil {
		respondServiceError(c, err, "Failed to list services")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Services retrieved successfully", gin.H{"services": services})
}

func (s *Server) createServiceImpl(c *gin.Context) {
	var in service.ServiceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	created, err := s.serviceCatalog().Create(in)
	if err != nil {
		respondServiceError(c, err, "Failed to create the service")
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Service created", gin.H{"service": created})
}

func (s *Server) getServiceImpl(c *gin.Context) {
	detail, err := s.serviceCatalog().Get(c.Param("id"))
	if err != nil {
		respondServiceError(c, err, "Failed to load the service")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Service retrieved successfully", gin.H{"service": detail})
}

func (s *Server) updateServiceImpl(c *gin.Context) {
	var patch service.ServicePatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	updated, err := s.serviceCatalog().Update(c.Param("id"), patch)
	if err != nil {
		respondServiceError(c, err, "Failed to update the service")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Service updated", gin.H{"service": updated})
}

func (s *Server) deleteServiceImpl(c *gin.Context) {
	// The audit entry is written after this handler, when the row is already gone: keep its name now.
	var existing db.Service
	if s.db.Select("name").Where("id = ?", c.Param("id")).Limit(1).Find(&existing).RowsAffected > 0 {
		setAuditDetail(c, map[string]any{"target_label": existing.Name})
	}
	unlinked, err := s.serviceCatalog().Delete(c.Param("id"))
	if err != nil {
		respondServiceError(c, err, "Failed to delete the service")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Service deleted", gin.H{"unlinked": unlinked})
}

func (s *Server) linkServiceItemImpl(c *gin.Context) {
	var ref service.ServiceLinkRef
	if err := c.ShouldBindJSON(&ref); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	result, err := s.serviceCatalog().Link(c.Param("id"), ref)
	if err != nil {
		respondServiceError(c, err, "Failed to link to the service")
		return
	}
	detail := map[string]any{"link_kind": result.Kind, "link_target": linkTargetOf(result.ServiceLinkRef)}
	if result.PreviousServiceID != "" {
		detail["previous_service_id"] = result.PreviousServiceID
	}
	setAuditDetail(c, detail)
	utils.SuccessResponse(c, http.StatusOK, "Linked to service", gin.H{"link": result})
}

func (s *Server) unlinkServiceItemImpl(c *gin.Context) {
	ref := service.ServiceLinkRef{Kind: strings.TrimSpace(c.Query("kind")), ID: strings.TrimSpace(c.Query("target_id")),
		NodeID: strings.TrimSpace(c.Query("node_id")), Key: strings.TrimSpace(c.Query("key"))}
	if err := s.serviceCatalog().Unlink(c.Param("id"), ref); err != nil {
		respondServiceError(c, err, "Failed to unlink from the service")
		return
	}
	setAuditDetail(c, map[string]any{"link_kind": ref.Kind, "link_target": linkTargetOf(ref)})
	utils.SuccessResponse(c, http.StatusOK, "Unlinked from service", nil)
}

func (s *Server) listServiceSuggestionsImpl(c *gin.Context) {
	suggestions, err := s.serviceCatalog().Suggest(c.Query("service_id"))
	if err != nil {
		respondServiceError(c, err, "Failed to build suggestions")
		return
	}
	if suggestions == nil {
		suggestions = []service.ServiceSuggestion{}
	}
	utils.SuccessResponse(c, http.StatusOK, "Service suggestions retrieved successfully", gin.H{"suggestions": suggestions})
}

// linkTargetOf names what a link points at for the audit trail: the id, or "node/key" for a resource.
func linkTargetOf(ref service.ServiceLinkRef) string {
	if ref.Kind == service.ServiceLinkResource {
		return ref.NodeID + "/" + ref.Key
	}
	return ref.ID
}
