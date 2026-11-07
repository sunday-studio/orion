package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves these types through this file's imports.
var (
	_ utils.APIResponse
	_ service.ServiceSummary
)

// registerServiceRoutes wires the Services routes (admin JWT).
func (s *Server) registerServiceRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/services", s.listServices)
	frontend.POST("/services", s.createService)
	frontend.GET("/services/suggestions", s.listServiceSuggestions)
	frontend.GET("/services/:id", s.getService)
	frontend.PATCH("/services/:id", s.updateService)
	frontend.DELETE("/services/:id", s.deleteService)
	frontend.POST("/services/:id/links", s.linkServiceItem)
	frontend.DELETE("/services/:id/links", s.unlinkServiceItem)
}

// listServices lists services.
// @Summary      List services
// @Description  Every service with how many apps, monitors and resources are linked to it, the worst known health among its monitors, and its open incident count.
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @ID           listServices
// @Success      200  {object}  utils.APIResponse{data=object{services=[]service.ServiceSummary}}
// @Failure      401  {object}  utils.APIResponse
// @Router       /v1/services [get]
func (s *Server) listServices(c *gin.Context) { s.listServicesImpl(c) }

// createService creates a service.
// @Summary      Create a service
// @Description  A service is one named thing that ties monitors, an app's logs and measured resources together. Creating it links nothing; link things with POST /v1/services/{id}/links. slug is derived from the name when omitted.
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createService
// @Param        request  body      service.ServiceInput  true  "Service"
// @Success      201  {object}  utils.APIResponse{data=object{service=service.ServiceSummary}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Router       /v1/services [post]
func (s *Server) createService(c *gin.Context) { s.createServiceImpl(c) }

// listServiceSuggestions proposes links.
// @Summary      Suggest service links
// @Description  Proposes links with the reason for each. Without service_id: unlinked things that match an existing service, and unlinked apps that could start a new service (service_id empty on those). With service_id: only what could join that service. A suggestion is advice only; nothing is linked until you call POST /v1/services/{id}/links. Rules: an app's slug or OpenTelemetry service.name (seen in its logs in the last 24 hours) against monitor names, the host a Core monitor checks, and resource (process, unit, container) names. Monitors the Node runs are matched by name only.
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @ID           listServiceSuggestions
// @Param        service_id  query     string  false  "Only suggest links for this service"
// @Success      200  {object}  utils.APIResponse{data=object{suggestions=[]service.ServiceSuggestion}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/services/suggestions [get]
func (s *Server) listServiceSuggestions(c *gin.Context) { s.listServiceSuggestionsImpl(c) }

// getService returns a service with what is linked to it.
// @Summary      Get a service
// @Description  The service with its linked apps, monitors and resources.
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @ID           getService
// @Param        id   path      string  true  "Service ID"
// @Success      200  {object}  utils.APIResponse{data=object{service=service.ServiceDetail}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/services/{id} [get]
func (s *Server) getService(c *gin.Context) { s.getServiceImpl(c) }

// updateService changes a service.
// @Summary      Update a service
// @Description  Omitted fields are unchanged; an empty node_id, environment or runbook_url clears it.
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateService
// @Param        id       path      string                true  "Service ID"
// @Param        request  body      service.ServicePatch  true  "Fields to change"
// @Success      200  {object}  utils.APIResponse{data=object{service=service.ServiceSummary}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Router       /v1/services/{id} [patch]
func (s *Server) updateService(c *gin.Context) { s.updateServiceImpl(c) }

// deleteService deletes a service.
// @Summary      Delete a service
// @Description  Removes the service and clears every link to it. The apps, monitors, resources, logs, reports and incidents that were linked are kept untouched.
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @ID           deleteService
// @Param        id   path      string  true  "Service ID"
// @Success      200  {object}  utils.APIResponse{data=object{unlinked=service.ServiceDeleteResult}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/services/{id} [delete]
func (s *Server) deleteService(c *gin.Context) { s.deleteServiceImpl(c) }

// linkServiceItem links an app, monitor or resource to a service.
// @Summary      Link to a service
// @Description  kind is app or monitor (with id) or resource (with node_id and key). A thing belongs to at most one service: linking one that already belongs to another service moves it, and previous_service_id says from where. Linking it to the same service again changes nothing. System monitors cannot be linked.
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           linkServiceItem
// @Param        id       path      string                 true  "Service ID"
// @Param        request  body      service.ServiceLinkRef  true  "What to link"
// @Success      200  {object}  utils.APIResponse{data=object{link=service.ServiceLinkResult}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/services/{id}/links [post]
func (s *Server) linkServiceItem(c *gin.Context) { s.linkServiceItemImpl(c) }

// unlinkServiceItem unlinks an app, monitor or resource from a service.
// @Summary      Unlink from a service
// @Description  Clears only the link. Logs, monitor reports, incidents and resource data stay as they are. 404 when the thing does not exist or is not linked to this service.
// @Tags         services
// @Produce      json
// @Security     BearerAuth
// @ID           unlinkServiceItem
// @Param        id       path      string  true   "Service ID"
// @Param        kind     query     string  true   "app, monitor or resource"  Enums(app, monitor, resource)
// @Param        target_id query    string  false  "App or monitor ID"
// @Param        node_id  query     string  false  "Resource node ID"
// @Param        key      query     string  false  "Resource key"
// @Success      200  {object}  utils.APIResponse
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/services/{id}/links [delete]
func (s *Server) unlinkServiceItem(c *gin.Context) { s.unlinkServiceItemImpl(c) }
