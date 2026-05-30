package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// listStatusPageIncidents lists manual public incidents.
// @Summary      List status page incidents
// @Description  Get manual public incident records for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageIncidents
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{incidents=[]StatusPageIncidentResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents [get]
func (s *Server) listStatusPageIncidents(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	incidents, err := s.loadStatusPageIncidents(c.Param("id"))
	if err != nil {
		s.logger.Error("Failed to list status page incidents", "error", err)
		utils.InternalError(c, "Failed to list status page incidents", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page incidents retrieved successfully", gin.H{
		"incidents": incidents,
		"count":     len(incidents),
	})
}

// suggestStatusPageIncidentComponents suggests public components affected by an internal incident.
// @Summary      Suggest status page incident components
// @Description  Suggest public status page components mapped to the internal incident monitor or agent without exposing internal incident details
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           suggestStatusPageIncidentComponents
// @Param        id           path      string  true  "Status page ID"
// @Param        incident_id  query     string  true  "Internal incident ID"
// @Success      200          {object}  utils.APIResponse{data=object{suggestions=[]StatusPageIncidentComponentSuggestionResponse,count=int}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/suggestions [get]
func (s *Server) suggestStatusPageIncidentComponents(c *gin.Context) {
	pageID := c.Param("id")
	if !s.statusPageExists(c, pageID) {
		return
	}

	incidentID := strings.TrimSpace(c.Query("incident_id"))
	if incidentID == "" {
		utils.BadRequest(c, "incident_id is required")
		return
	}

	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Incident not found")
		return
	}

	suggestions, err := s.statusPageIncidentComponentSuggestions(pageID, incident)
	if err != nil {
		s.logger.Error("Failed to suggest status page incident components", "status_page_id", pageID, "incident_id", incidentID, "error", err)
		utils.InternalError(c, "Failed to suggest status page incident components", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page incident component suggestions retrieved successfully", gin.H{
		"suggestions": suggestions,
		"count":       len(suggestions),
	})
}

// createStatusPageIncident creates a manual public incident.
// @Summary      Create status page incident
// @Description  Create a manual public incident for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageIncident
// @Param        id       path      string                     true  "Status page ID"
// @Param        request  body      statusPageIncidentRequest  true  "Incident payload"
// @Success      201      {object}  utils.APIResponse{data=object{incident=StatusPageIncidentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents [post]
func (s *Server) createStatusPageIncident(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident payload")
		return
	}
	incident := db.StatusPageIncident{
		ID:                   utils.GenerateID("status_page_incident"),
		StatusPageID:         c.Param("id"),
		PublicStatus:         "investigating",
		Severity:             "medium",
		Visibility:           statusPageIncidentVisibilityDraft,
		AffectedComponentIDs: "[]",
	}
	if err := s.applyStatusPageIncidentRequest(&incident, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentCreated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident",
			AffectedObjectID:   incident.ID,
		})
	}); err != nil {
		s.logger.Error("Failed to create status page incident", "status_page_id", incident.StatusPageID, "error", err)
		utils.InternalError(c, "Failed to create status page incident", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page incident created successfully", gin.H{
		"incident": statusPageIncidentResponse(incident, nil),
	})
}

// updateStatusPageIncident updates a manual public incident.
// @Summary      Update status page incident
// @Description  Update a manual public incident for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageIncident
// @Param        id           path      string                     true  "Status page ID"
// @Param        incident_id  path      string                     true  "Incident ID"
// @Param        request      body      statusPageIncidentRequest  true  "Incident payload"
// @Success      200          {object}  utils.APIResponse{data=object{incident=StatusPageIncidentResponse}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/{incident_id} [put]
func (s *Server) updateStatusPageIncident(c *gin.Context) {
	var incident db.StatusPageIncident
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("incident_id"), c.Param("id")).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page incident not found")
		return
	}
	var request statusPageIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident payload")
		return
	}
	wasResolved := incident.PublicStatus == "resolved" || incident.ResolvedAt != nil
	if err := s.applyStatusPageIncidentRequest(&incident, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&incident).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentUpdated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident",
			AffectedObjectID:   incident.ID,
		}); err != nil {
			return err
		}
		if !wasResolved && (incident.PublicStatus == "resolved" || incident.ResolvedAt != nil) {
			return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
				Action:             service.StatusPageAuditActionPublicIncidentResolved,
				StatusPageID:       incident.StatusPageID,
				AffectedObjectType: "public_incident",
				AffectedObjectID:   incident.ID,
			})
		}
		return nil
	}); err != nil {
		s.logger.Error("Failed to update status page incident", "status_page_id", incident.StatusPageID, "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to update status page incident", err)
		return
	}
	updates, err := s.statusPageIncidentUpdates(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load status page incident updates", "error", err)
		utils.InternalError(c, "Failed to update status page incident", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page incident updated successfully", gin.H{
		"incident": statusPageIncidentResponse(incident, updates),
	})
}

// createStatusPageIncidentUpdate creates a public incident update.
// @Summary      Create status page incident update
// @Description  Create a public timeline update for a manual status page incident
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageIncidentUpdate
// @Param        id           path      string                           true  "Status page ID"
// @Param        incident_id  path      string                           true  "Incident ID"
// @Param        request      body      statusPageIncidentUpdateRequest  true  "Incident update payload"
// @Success      201          {object}  utils.APIResponse{data=object{update=StatusPageIncidentUpdateResponse,incident=StatusPageIncidentResponse}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/{incident_id}/updates [post]
func (s *Server) createStatusPageIncidentUpdate(c *gin.Context) {
	var incident db.StatusPageIncident
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("incident_id"), c.Param("id")).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page incident not found")
		return
	}
	var request statusPageIncidentUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident update payload")
		return
	}
	update := db.StatusPageIncidentUpdate{
		ID:         utils.GenerateID("status_page_update"),
		IncidentID: incident.ID,
	}
	if err := applyStatusPageIncidentUpdateRequest(&update, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	wasResolved := incident.PublicStatus == "resolved" || incident.ResolvedAt != nil
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&update).Error; err != nil {
			return err
		}
		incident.PublicStatus = update.Status
		if update.PublishedAt != nil && incident.Visibility == statusPageIncidentVisibilityDraft {
			incident.Visibility = statusPageIncidentVisibilityPublished
			incident.PublishedAt = update.PublishedAt
		}
		if update.Status == "resolved" && incident.ResolvedAt == nil {
			now := time.Now().UTC()
			incident.ResolvedAt = &now
		}
		if err := tx.Save(&incident).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentUpdateCreated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident_update",
			AffectedObjectID:   update.ID,
		}); err != nil {
			return err
		}
		if err := s.enqueueStatusPageSubscriberIncidentUpdateDeliveries(tx, incident, update); err != nil {
			return err
		}
		if !wasResolved && (incident.PublicStatus == "resolved" || incident.ResolvedAt != nil) {
			return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
				Action:             service.StatusPageAuditActionPublicIncidentResolved,
				StatusPageID:       incident.StatusPageID,
				AffectedObjectType: "public_incident",
				AffectedObjectID:   incident.ID,
			})
		}
		return nil
	}); err != nil {
		s.logger.Error("Failed to create status page incident update", "error", err)
		utils.InternalError(c, "Failed to create status page incident update", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page incident update created successfully", gin.H{
		"update":   statusPageIncidentUpdateResponse(update),
		"incident": statusPageIncidentResponse(incident, []StatusPageIncidentUpdateResponse{statusPageIncidentUpdateResponse(update)}),
	})
}

func (s *Server) loadStatusPageIncidents(pageID string) ([]StatusPageIncidentResponse, error) {
	var incidents []db.StatusPageIncident
	if err := s.db.Where("status_page_id = ?", pageID).Order("created_at DESC").Find(&incidents).Error; err != nil {
		return nil, err
	}
	incidentIDs := make([]string, 0, len(incidents))
	for _, incident := range incidents {
		incidentIDs = append(incidentIDs, incident.ID)
	}
	updatesByIncident := map[string][]StatusPageIncidentUpdateResponse{}
	if len(incidentIDs) > 0 {
		var updates []db.StatusPageIncidentUpdate
		if err := s.db.Where("incident_id IN ?", incidentIDs).Order("created_at ASC").Find(&updates).Error; err != nil {
			return nil, err
		}
		for _, update := range updates {
			updatesByIncident[update.IncidentID] = append(updatesByIncident[update.IncidentID], statusPageIncidentUpdateResponse(update))
		}
	}
	responses := make([]StatusPageIncidentResponse, 0, len(incidents))
	for _, incident := range incidents {
		responses = append(responses, statusPageIncidentResponse(incident, updatesByIncident[incident.ID]))
	}
	return responses, nil
}

func (s *Server) statusPageIncidentComponentSuggestions(pageID string, incident db.Incident) ([]StatusPageIncidentComponentSuggestionResponse, error) {
	type suggestionRow struct {
		ComponentID   string
		ComponentName string
		ResourceType  string
	}

	query := s.db.Table("status_page_components AS components").
		Select("components.id AS component_id, components.public_name AS component_name, mappings.resource_type AS resource_type").
		Joins("JOIN status_page_component_mappings AS mappings ON mappings.component_id = components.id").
		Where("components.status_page_id = ? AND components.visible = ?", pageID, true)

	matchClauses := make([]string, 0, 2)
	matchArgs := make([]interface{}, 0, 4)
	if incident.MonitorID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "monitor", incident.MonitorID)
	}
	if incident.AgentID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "agent", incident.AgentID)
	}
	if len(matchClauses) == 0 {
		return []StatusPageIncidentComponentSuggestionResponse{}, nil
	}

	var rows []suggestionRow
	if err := query.
		Where(strings.Join(matchClauses, " OR "), matchArgs...).
		Order("components.sort_order ASC, components.public_name ASC, mappings.resource_type ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	suggestionsByComponent := map[string]*StatusPageIncidentComponentSuggestionResponse{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, ok := suggestionsByComponent[row.ComponentID]; !ok {
			suggestionsByComponent[row.ComponentID] = &StatusPageIncidentComponentSuggestionResponse{
				ComponentID:   row.ComponentID,
				ComponentName: row.ComponentName,
				Matches:       []StatusPageIncidentComponentSuggestionMatchResponse{},
			}
			order = append(order, row.ComponentID)
		}
		suggestion := suggestionsByComponent[row.ComponentID]
		suggestion.Matches = append(suggestion.Matches, StatusPageIncidentComponentSuggestionMatchResponse{
			ResourceType: row.ResourceType,
			MatchReason:  statusPageIncidentSuggestionMatchReason(row.ResourceType),
		})
	}

	suggestions := make([]StatusPageIncidentComponentSuggestionResponse, 0, len(order))
	for _, componentID := range order {
		suggestions = append(suggestions, *suggestionsByComponent[componentID])
	}
	return suggestions, nil
}

func (s *Server) statusPageIncidentUpdates(incidentID string) ([]StatusPageIncidentUpdateResponse, error) {
	var updates []db.StatusPageIncidentUpdate
	if err := s.db.Where("incident_id = ?", incidentID).Order("created_at ASC").Find(&updates).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageIncidentUpdateResponse, 0, len(updates))
	for _, update := range updates {
		responses = append(responses, statusPageIncidentUpdateResponse(update))
	}
	return responses, nil
}

func (s *Server) applyStatusPageIncidentRequest(incident *db.StatusPageIncident, request statusPageIncidentRequest, create bool) error {
	if request.InternalIncidentID != nil {
		incident.InternalIncidentID = strings.TrimSpace(*request.InternalIncidentID)
	}
	if request.Title != nil {
		incident.Title = strings.TrimSpace(*request.Title)
	}
	if request.PublicStatus != nil {
		incident.PublicStatus = strings.TrimSpace(*request.PublicStatus)
	}
	if request.Severity != nil {
		incident.Severity = strings.TrimSpace(*request.Severity)
	}
	if request.ImpactSummary != nil {
		incident.ImpactSummary = strings.TrimSpace(*request.ImpactSummary)
	}
	if request.Visibility != nil {
		incident.Visibility = strings.TrimSpace(*request.Visibility)
	}
	if request.AffectedComponentIDs != nil {
		componentIDs := normalizeStringList(request.AffectedComponentIDs)
		if err := s.ensureStatusPageComponentsExist(incident.StatusPageID, componentIDs); err != nil {
			return err
		}
		incident.AffectedComponentIDs = encodeStringList(componentIDs)
	}
	if request.PublishedAt != nil {
		incident.PublishedAt = request.PublishedAt
	}
	if request.ResolvedAt != nil {
		incident.ResolvedAt = request.ResolvedAt
	}
	if request.ScheduledStartAt != nil {
		incident.ScheduledStartAt = request.ScheduledStartAt
	}
	if request.ScheduledEndAt != nil {
		incident.ScheduledEndAt = request.ScheduledEndAt
	}
	if create && incident.PublicStatus == "" {
		incident.PublicStatus = "investigating"
	}
	if create && incident.Severity == "" {
		incident.Severity = "medium"
	}
	if create && incident.Visibility == "" {
		incident.Visibility = statusPageIncidentVisibilityDraft
	}
	if strings.TrimSpace(incident.Title) == "" {
		return &requestValidationError{message: "status page incident title is required"}
	}
	if !validStatusPageIncidentStatus(incident.PublicStatus) {
		return &requestValidationError{message: "unsupported status page incident public_status"}
	}
	if !validStatusPageSeverity(incident.Severity) {
		return &requestValidationError{message: "unsupported status page incident severity"}
	}
	if !validStatusPageIncidentVisibility(incident.Visibility) {
		return &requestValidationError{message: "unsupported status page incident visibility"}
	}
	if incident.InternalIncidentID != "" {
		var count int64
		if err := s.db.Model(&db.Incident{}).Where("id = ?", incident.InternalIncidentID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return &requestValidationError{message: "status page incident internal_incident_id must reference an existing incident"}
		}
	}
	return nil
}

func applyStatusPageIncidentUpdateRequest(update *db.StatusPageIncidentUpdate, request statusPageIncidentUpdateRequest, create bool) error {
	if request.Status != nil {
		update.Status = strings.TrimSpace(*request.Status)
	}
	if request.Message != nil {
		update.Message = strings.TrimSpace(*request.Message)
	}
	if request.CreatedBy != nil {
		update.CreatedBy = strings.TrimSpace(*request.CreatedBy)
	}
	if request.PublishedAt != nil {
		update.PublishedAt = request.PublishedAt
	}
	if create && update.Status == "" {
		update.Status = "investigating"
	}
	if strings.TrimSpace(update.Message) == "" {
		return &requestValidationError{message: "status page incident update message is required"}
	}
	if !validStatusPageIncidentStatus(update.Status) {
		return &requestValidationError{message: "unsupported status page incident update status"}
	}
	return nil
}

func (s *Server) ensureStatusPageComponentsExist(pageID string, componentIDs []string) error {
	if len(componentIDs) == 0 {
		return nil
	}
	var count int64
	if err := s.db.Model(&db.StatusPageComponent{}).Where("status_page_id = ? AND id IN ?", pageID, componentIDs).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(componentIDs) {
		return &requestValidationError{message: "affected_component_ids must reference components on this status page"}
	}
	return nil
}

func statusPageIncidentSuggestionMatchReason(resourceType string) string {
	switch resourceType {
	case "monitor":
		return "internal incident monitor matched a public component mapping"
	case "agent":
		return "internal incident agent matched a public component mapping"
	default:
		return "internal incident resource matched a public component mapping"
	}
}
