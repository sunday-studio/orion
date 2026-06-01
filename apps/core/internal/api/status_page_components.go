package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// listStatusPageSections lists sections for a status page.
// @Summary      List status page sections
// @Description  Get sections for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageSections
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{sections=[]StatusPageSectionResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections [get]
func (s *Server) listStatusPageSections(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var sections []db.StatusPageSection
	if err := s.db.Where("status_page_id = ?", c.Param("id")).Order("sort_order ASC, name ASC").Find(&sections).Error; err != nil {
		s.logger.Error("Failed to list status page sections", "error", err)
		utils.InternalError(c, "Failed to list status page sections", err)
		return
	}
	responses := statusPageSectionResponses(sections)
	utils.SuccessResponse(c, http.StatusOK, "Status page sections retrieved successfully", gin.H{
		"sections": responses,
		"count":    len(responses),
	})
}

// createStatusPageSection creates a section.
// @Summary      Create status page section
// @Description  Create a section for grouping public components
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageSection
// @Param        id       path      string                    true  "Status page ID"
// @Param        request  body      statusPageSectionRequest  true  "Section payload"
// @Success      201      {object}  utils.APIResponse{data=object{section=StatusPageSectionResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections [post]
func (s *Server) createStatusPageSection(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageSectionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page section payload")
		return
	}
	section := db.StatusPageSection{
		ID:           utils.GenerateID("status_page_section"),
		StatusPageID: c.Param("id"),
	}
	if err := applyStatusPageSectionRequest(&section, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&section).Error; err != nil {
		s.logger.Error("Failed to create status page section", "error", err)
		utils.InternalError(c, "Failed to create status page section", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page section created successfully", gin.H{
		"section": statusPageSectionResponse(section),
	})
}

// updateStatusPageSection updates a section.
// @Summary      Update status page section
// @Description  Update a status page section
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageSection
// @Param        id          path      string                    true  "Status page ID"
// @Param        section_id  path      string                    true  "Section ID"
// @Param        request     body      statusPageSectionRequest  true  "Section payload"
// @Success      200         {object}  utils.APIResponse{data=object{section=StatusPageSectionResponse}}
// @Failure      400         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Failure      500         {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections/{section_id} [put]
func (s *Server) updateStatusPageSection(c *gin.Context) {
	var section db.StatusPageSection
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("section_id"), c.Param("id")).First(&section).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page section not found")
		return
	}
	var request statusPageSectionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page section payload")
		return
	}
	if err := applyStatusPageSectionRequest(&section, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Save(&section).Error; err != nil {
		s.logger.Error("Failed to update status page section", "error", err)
		utils.InternalError(c, "Failed to update status page section", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page section updated successfully", gin.H{
		"section": statusPageSectionResponse(section),
	})
}

// listStatusPageComponents lists components for a status page.
// @Summary      List status page components
// @Description  Get components and mappings for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageComponents
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{components=[]StatusPageComponentResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components [get]
func (s *Server) listStatusPageComponents(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	components, err := s.loadStatusPageComponents(c.Param("id"))
	if err != nil {
		s.logger.Error("Failed to list status page components", "error", err)
		utils.InternalError(c, "Failed to list status page components", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page components retrieved successfully", gin.H{
		"components": components,
		"count":      len(components),
	})
}

// createStatusPageComponent creates a component.
// @Summary      Create status page component
// @Description  Create a public component for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageComponent
// @Param        id       path      string                      true  "Status page ID"
// @Param        request  body      statusPageComponentRequest  true  "Component payload"
// @Success      201      {object}  utils.APIResponse{data=object{component=StatusPageComponentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components [post]
func (s *Server) createStatusPageComponent(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageComponentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component payload")
		return
	}
	component := db.StatusPageComponent{
		ID:           utils.GenerateID("status_page_component"),
		StatusPageID: c.Param("id"),
		DisplayMode:  "single_resource",
		Visible:      true,
	}
	if err := s.applyStatusPageComponentRequest(&component, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&component).Error; err != nil {
		s.logger.Error("Failed to create status page component", "error", err)
		utils.InternalError(c, "Failed to create status page component", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page component created successfully", gin.H{
		"component": statusPageComponentResponse(component, nil),
	})
}

// updateStatusPageComponent updates a component.
// @Summary      Update status page component
// @Description  Update a public component for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageComponent
// @Param        id            path      string                      true  "Status page ID"
// @Param        component_id  path      string                      true  "Component ID"
// @Param        request       body      statusPageComponentRequest  true  "Component payload"
// @Success      200           {object}  utils.APIResponse{data=object{component=StatusPageComponentResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id} [put]
func (s *Server) updateStatusPageComponent(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var request statusPageComponentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component payload")
		return
	}
	if err := s.applyStatusPageComponentRequest(&component, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Save(&component).Error; err != nil {
		s.logger.Error("Failed to update status page component", "error", err)
		utils.InternalError(c, "Failed to update status page component", err)
		return
	}
	mappings, err := s.statusPageComponentMappings(component.ID)
	if err != nil {
		s.logger.Error("Failed to load status page component mappings", "error", err)
		utils.InternalError(c, "Failed to update status page component", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component updated successfully", gin.H{
		"component": statusPageComponentResponse(component, mappings),
	})
}

// listStatusPageComponentMappings lists component mappings.
// @Summary      List status page component mappings
// @Description  Get internal resource mappings for a public component
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageComponentMappings
// @Param        id            path      string  true  "Status page ID"
// @Param        component_id  path      string  true  "Component ID"
// @Success      200           {object}  utils.APIResponse{data=object{mappings=[]StatusPageComponentMappingResponse,count=int}}
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings [get]
func (s *Server) listStatusPageComponentMappings(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	mappings, err := s.statusPageComponentMappings(component.ID)
	if err != nil {
		s.logger.Error("Failed to list status page component mappings", "error", err)
		utils.InternalError(c, "Failed to list status page component mappings", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component mappings retrieved successfully", gin.H{
		"mappings": mappings,
		"count":    len(mappings),
	})
}

// createStatusPageComponentMapping creates a component mapping.
// @Summary      Create status page component mapping
// @Description  Map a public component to an internal agent or monitor
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageComponentMapping
// @Param        id            path      string                            true  "Status page ID"
// @Param        component_id  path      string                            true  "Component ID"
// @Param        request       body      statusPageComponentMappingRequest  true  "Mapping payload"
// @Success      201           {object}  utils.APIResponse{data=object{mapping=StatusPageComponentMappingResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      409           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings [post]
func (s *Server) createStatusPageComponentMapping(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var request statusPageComponentMappingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component mapping payload")
		return
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   utils.GenerateID("status_page_mapping"),
		ComponentID:          component.ID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
	}
	if err := s.applyStatusPageComponentMappingRequest(&mapping, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mapping).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentMappingCreated,
			StatusPageID:       c.Param("id"),
			AffectedObjectType: "component_mapping",
			AffectedObjectID:   mapping.ID,
		})
	}); err != nil {
		writeStatusPageCreateError(c, err, "Status page component mapping already exists")
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page component mapping created successfully", gin.H{
		"mapping": statusPageComponentMappingResponse(mapping),
	})
}

// updateStatusPageComponentMapping updates a component mapping.
// @Summary      Update status page component mapping
// @Description  Update a public component internal resource mapping
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageComponentMapping
// @Param        id            path      string                            true  "Status page ID"
// @Param        component_id  path      string                            true  "Component ID"
// @Param        mapping_id    path      string                            true  "Mapping ID"
// @Param        request       body      statusPageComponentMappingRequest  true  "Mapping payload"
// @Success      200           {object}  utils.APIResponse{data=object{mapping=StatusPageComponentMappingResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      409           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings/{mapping_id} [put]
func (s *Server) updateStatusPageComponentMapping(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var mapping db.StatusPageComponentMapping
	if err := s.db.Where("id = ? AND component_id = ?", c.Param("mapping_id"), component.ID).First(&mapping).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component mapping not found")
		return
	}
	var request statusPageComponentMappingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component mapping payload")
		return
	}
	if err := s.applyStatusPageComponentMappingRequest(&mapping, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&mapping).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentMappingUpdated,
			StatusPageID:       c.Param("id"),
			AffectedObjectType: "component_mapping",
			AffectedObjectID:   mapping.ID,
		})
	}); err != nil {
		writeStatusPageCreateError(c, err, "Status page component mapping already exists")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component mapping updated successfully", gin.H{
		"mapping": statusPageComponentMappingResponse(mapping),
	})
}

func (s *Server) loadStatusPageComponents(pageID string) ([]StatusPageComponentResponse, error) {
	var components []db.StatusPageComponent
	if err := s.db.Where("status_page_id = ?", pageID).Order("sort_order ASC, public_name ASC").Find(&components).Error; err != nil {
		return nil, err
	}
	componentIDs := make([]string, 0, len(components))
	for _, component := range components {
		componentIDs = append(componentIDs, component.ID)
	}
	mappingsByComponent := map[string][]StatusPageComponentMappingResponse{}
	if len(componentIDs) > 0 {
		var mappings []db.StatusPageComponentMapping
		if err := s.db.Where("component_id IN ?", componentIDs).Order("resource_type ASC, resource_id ASC").Find(&mappings).Error; err != nil {
			return nil, err
		}
		for _, mapping := range mappings {
			mappingsByComponent[mapping.ComponentID] = append(mappingsByComponent[mapping.ComponentID], statusPageComponentMappingResponse(mapping))
		}
	}
	responses := make([]StatusPageComponentResponse, 0, len(components))
	for _, component := range components {
		responses = append(responses, statusPageComponentResponse(component, mappingsByComponent[component.ID]))
	}
	return responses, nil
}

func (s *Server) loadStatusPageComponentForRequest(c *gin.Context) (db.StatusPageComponent, bool) {
	var component db.StatusPageComponent
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("component_id"), c.Param("id")).First(&component).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component not found")
		return db.StatusPageComponent{}, false
	}
	return component, true
}

func (s *Server) statusPageComponentMappings(componentID string) ([]StatusPageComponentMappingResponse, error) {
	var mappings []db.StatusPageComponentMapping
	if err := s.db.Where("component_id = ?", componentID).Order("resource_type ASC, resource_id ASC").Find(&mappings).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageComponentMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, statusPageComponentMappingResponse(mapping))
	}
	return responses, nil
}

func applyStatusPageSectionRequest(section *db.StatusPageSection, request statusPageSectionRequest, create bool) error {
	if request.Name != nil {
		section.Name = strings.TrimSpace(*request.Name)
	}
	if request.SortOrder != nil {
		section.SortOrder = *request.SortOrder
	}
	if request.CollapsedByDefault != nil {
		section.CollapsedByDefault = *request.CollapsedByDefault
	}
	if create && section.Name == "" {
		return &requestValidationError{message: "status page section name is required"}
	}
	if strings.TrimSpace(section.Name) == "" {
		return &requestValidationError{message: "status page section name is required"}
	}
	return nil
}

func (s *Server) applyStatusPageComponentRequest(component *db.StatusPageComponent, request statusPageComponentRequest, create bool) error {
	if request.SectionID != nil {
		component.SectionID = strings.TrimSpace(*request.SectionID)
	}
	if request.PublicName != nil {
		component.PublicName = strings.TrimSpace(*request.PublicName)
	}
	if request.PublicDescription != nil {
		component.PublicDescription = strings.TrimSpace(*request.PublicDescription)
	}
	if request.DisplayMode != nil {
		component.DisplayMode = strings.TrimSpace(*request.DisplayMode)
	}
	if request.ManualStatus != nil {
		component.ManualStatus = strings.TrimSpace(*request.ManualStatus)
	}
	if request.ManualStatusReason != nil {
		component.ManualStatusReason = strings.TrimSpace(*request.ManualStatusReason)
	}
	if request.SortOrder != nil {
		component.SortOrder = *request.SortOrder
	}
	if request.Visible != nil {
		component.Visible = *request.Visible
	}
	if create && component.DisplayMode == "" {
		component.DisplayMode = "single_resource"
	}
	if strings.TrimSpace(component.SectionID) == "" {
		return &requestValidationError{message: "status page component section_id is required"}
	}
	if strings.TrimSpace(component.PublicName) == "" {
		return &requestValidationError{message: "status page component public_name is required"}
	}
	if !validStatusPageDisplayMode(component.DisplayMode) {
		return &requestValidationError{message: "unsupported status page component display_mode"}
	}
	if component.ManualStatus != "" && !validStatusPageComponentStatus(component.ManualStatus) {
		return &requestValidationError{message: "unsupported status page component manual_status"}
	}
	var count int64
	if err := s.db.Model(&db.StatusPageSection{}).Where("id = ? AND status_page_id = ?", component.SectionID, component.StatusPageID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return &requestValidationError{message: "status page component section_id must reference a section on this status page"}
	}
	return nil
}

func (s *Server) applyStatusPageComponentMappingRequest(mapping *db.StatusPageComponentMapping, request statusPageComponentMappingRequest, create bool) error {
	if request.ResourceType != nil {
		mapping.ResourceType = strings.TrimSpace(*request.ResourceType)
	}
	if request.ResourceID != nil {
		mapping.ResourceID = strings.TrimSpace(*request.ResourceID)
	}
	if request.HealthRollupStrategy != nil {
		mapping.HealthRollupStrategy = strings.TrimSpace(*request.HealthRollupStrategy)
	}
	if request.UptimeRollupStrategy != nil {
		mapping.UptimeRollupStrategy = strings.TrimSpace(*request.UptimeRollupStrategy)
	}
	if create && mapping.HealthRollupStrategy == "" {
		mapping.HealthRollupStrategy = "worst"
	}
	if create && mapping.UptimeRollupStrategy == "" {
		mapping.UptimeRollupStrategy = "worst"
	}
	if strings.TrimSpace(mapping.ResourceType) == "" || strings.TrimSpace(mapping.ResourceID) == "" {
		return &requestValidationError{message: "status page component mapping requires resource_type and resource_id"}
	}
	if !validStatusPageResourceType(mapping.ResourceType) {
		return &requestValidationError{message: "unsupported status page component mapping resource_type"}
	}
	if !validStatusPageRollupStrategy(mapping.HealthRollupStrategy) || !validStatusPageRollupStrategy(mapping.UptimeRollupStrategy) {
		return &requestValidationError{message: "unsupported status page component mapping rollup strategy"}
	}
	if err := s.ensureStatusPageMappingResourceExists(mapping.ResourceType, mapping.ResourceID); err != nil {
		return err
	}
	return nil
}

func (s *Server) ensureStatusPageMappingResourceExists(resourceType string, resourceID string) error {
	var count int64
	switch resourceType {
	case "agent":
		if err := s.db.Model(&db.Agent{}).Where("id = ?", resourceID).Count(&count).Error; err != nil {
			return err
		}
	case "monitor":
		if err := s.db.Model(&db.Monitor{}).Where("id = ?", resourceID).Count(&count).Error; err != nil {
			return err
		}
	}
	if count == 0 {
		return &requestValidationError{message: "status page component mapping resource_id must reference an existing resource"}
	}
	return nil
}
