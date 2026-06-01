package api

import (
	"net/http"

	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// deleteStatusPage deletes a status page and all nested public configuration.
// @Summary      Delete status page
// @Description  Delete a status page, including sections, components, mappings, public incidents, subscriber preferences, subscribers, and delivery history
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPage
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{id=string}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id} [delete]
func (s *Server) deleteStatusPage(c *gin.Context) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", c.Param("id")).First(&page).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page not found")
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		summary, err := statusPageDeleteSummary(tx, page.ID)
		if err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionDeleted,
			StatusPageID:       page.ID,
			AffectedObjectType: "status_page",
			AffectedObjectID:   page.ID,
			Metadata:           summary.metadata(),
		}); err != nil {
			return err
		}
		if err := tx.Where("status_page_id = ?", page.ID).Delete(&db.StatusPageSubscriberDelivery{}).Error; err != nil {
			return err
		}
		if len(summary.SubscriberIDs) > 0 {
			if err := tx.Where("subscriber_id IN ?", summary.SubscriberIDs).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("status_page_id = ?", page.ID).Delete(&db.StatusPageSubscriber{}).Error; err != nil {
			return err
		}
		if len(summary.IncidentIDs) > 0 {
			if err := tx.Where("incident_id IN ?", summary.IncidentIDs).Delete(&db.StatusPageIncidentUpdate{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("status_page_id = ?", page.ID).Delete(&db.StatusPageIncident{}).Error; err != nil {
			return err
		}
		if len(summary.ComponentIDs) > 0 {
			if err := tx.Where("component_id IN ?", summary.ComponentIDs).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
				return err
			}
			if err := tx.Where("component_id IN ?", summary.ComponentIDs).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("status_page_id = ?", page.ID).Delete(&db.StatusPageComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("status_page_id = ?", page.ID).Delete(&db.StatusPageSection{}).Error; err != nil {
			return err
		}
		return tx.Delete(&page).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page deleted successfully", gin.H{"id": page.ID})
}

// deleteStatusPageSection deletes a section and its public components.
// @Summary      Delete status page section
// @Description  Delete a status page section, its public components, mappings, and subscriber component preferences
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageSection
// @Param        id          path      string  true  "Status page ID"
// @Param        section_id  path      string  true  "Section ID"
// @Success      200         {object}  utils.APIResponse{data=object{id=string}}
// @Failure      404         {object}  utils.APIResponse
// @Failure      500         {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections/{section_id} [delete]
func (s *Server) deleteStatusPageSection(c *gin.Context) {
	var section db.StatusPageSection
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("section_id"), c.Param("id")).First(&section).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page section not found")
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		componentIDs, err := statusPageSectionComponentIDs(tx, section.ID)
		if err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionSectionDeleted,
			StatusPageID:       section.StatusPageID,
			AffectedObjectType: "status_page_section",
			AffectedObjectID:   section.ID,
			Metadata: map[string]interface{}{
				"component_ids": componentIDs,
			},
		}); err != nil {
			return err
		}
		if err := pruneStatusPageIncidentComponentIDs(tx, section.StatusPageID, componentIDs); err != nil {
			return err
		}
		if len(componentIDs) > 0 {
			if err := tx.Where("component_id IN ?", componentIDs).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
				return err
			}
			if err := tx.Where("component_id IN ?", componentIDs).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("section_id = ? AND status_page_id = ?", section.ID, section.StatusPageID).Delete(&db.StatusPageComponent{}).Error; err != nil {
			return err
		}
		return tx.Delete(&section).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page section", "status_page_id", section.StatusPageID, "section_id", section.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page section", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page section deleted successfully", gin.H{"id": section.ID})
}

// deleteStatusPageComponent deletes a public status page component.
// @Summary      Delete status page component
// @Description  Delete a public status page component, mappings, and subscriber component preferences
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageComponent
// @Param        id            path      string  true  "Status page ID"
// @Param        component_id  path      string  true  "Component ID"
// @Success      200           {object}  utils.APIResponse{data=object{id=string}}
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id} [delete]
func (s *Server) deleteStatusPageComponent(c *gin.Context) {
	var component db.StatusPageComponent
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("component_id"), c.Param("id")).First(&component).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component not found")
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentDeleted,
			StatusPageID:       component.StatusPageID,
			AffectedObjectType: "status_page_component",
			AffectedObjectID:   component.ID,
			Metadata: map[string]interface{}{
				"section_id": component.SectionID,
			},
		}); err != nil {
			return err
		}
		if err := pruneStatusPageIncidentComponentIDs(tx, component.StatusPageID, []string{component.ID}); err != nil {
			return err
		}
		if err := tx.Where("component_id = ?", component.ID).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Where("component_id = ?", component.ID).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
			return err
		}
		return tx.Delete(&component).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page component", "status_page_id", component.StatusPageID, "component_id", component.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page component", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page component deleted successfully", gin.H{"id": component.ID})
}

// deleteStatusPageComponentMapping deletes a component mapping.
// @Summary      Delete status page component mapping
// @Description  Delete a public component mapping to an internal monitor or agent
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageComponentMapping
// @Param        id            path      string  true  "Status page ID"
// @Param        component_id  path      string  true  "Component ID"
// @Param        mapping_id    path      string  true  "Mapping ID"
// @Success      200           {object}  utils.APIResponse{data=object{id=string}}
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings/{mapping_id} [delete]
func (s *Server) deleteStatusPageComponentMapping(c *gin.Context) {
	var component db.StatusPageComponent
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("component_id"), c.Param("id")).First(&component).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component not found")
		return
	}
	var mapping db.StatusPageComponentMapping
	if err := s.db.Where("id = ? AND component_id = ?", c.Param("mapping_id"), component.ID).First(&mapping).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component mapping not found")
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentMappingDeleted,
			StatusPageID:       component.StatusPageID,
			AffectedObjectType: "status_page_component_mapping",
			AffectedObjectID:   mapping.ID,
			Metadata: map[string]interface{}{
				"component_id":  component.ID,
				"resource_type": mapping.ResourceType,
				"resource_id":   mapping.ResourceID,
			},
		}); err != nil {
			return err
		}
		return tx.Delete(&mapping).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page component mapping", "status_page_id", component.StatusPageID, "component_id", component.ID, "mapping_id", mapping.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page component mapping", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page component mapping deleted successfully", gin.H{"id": mapping.ID})
}

// deleteStatusPageIncident deletes a public incident and its timeline updates.
// @Summary      Delete status page incident
// @Description  Delete a manual public status page incident, public updates, and subscriber delivery history for that incident
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageIncident
// @Param        id           path      string  true  "Status page ID"
// @Param        incident_id  path      string  true  "Incident ID"
// @Success      200          {object}  utils.APIResponse{data=object{id=string}}
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/{incident_id} [delete]
func (s *Server) deleteStatusPageIncident(c *gin.Context) {
	var incident db.StatusPageIncident
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("incident_id"), c.Param("id")).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page incident not found")
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentDeleted,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "status_page_public_incident",
			AffectedObjectID:   incident.ID,
			Metadata: map[string]interface{}{
				"internal_incident_id": incident.InternalIncidentID,
			},
		}); err != nil {
			return err
		}
		if err := tx.Where("incident_id = ?", incident.ID).Delete(&db.StatusPageIncidentUpdate{}).Error; err != nil {
			return err
		}
		if err := tx.Where("public_incident_id = ?", incident.ID).Delete(&db.StatusPageSubscriberDelivery{}).Error; err != nil {
			return err
		}
		return tx.Delete(&incident).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page incident", "status_page_id", incident.StatusPageID, "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page incident", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page incident deleted successfully", gin.H{"id": incident.ID})
}

type statusPageDeletionSummary struct {
	SectionIDs    []string
	ComponentIDs  []string
	MappingIDs    []string
	IncidentIDs   []string
	SubscriberIDs []string
	DeliveryIDs   []string
}

func (summary statusPageDeletionSummary) metadata() map[string]interface{} {
	return map[string]interface{}{
		"section_ids":    summary.SectionIDs,
		"component_ids":  summary.ComponentIDs,
		"mapping_ids":    summary.MappingIDs,
		"incident_ids":   summary.IncidentIDs,
		"subscriber_ids": summary.SubscriberIDs,
		"delivery_ids":   summary.DeliveryIDs,
	}
}

func statusPageDeleteSummary(tx *gorm.DB, pageID string) (statusPageDeletionSummary, error) {
	summary := statusPageDeletionSummary{}
	if err := tx.Model(&db.StatusPageSection{}).Where("status_page_id = ?", pageID).Pluck("id", &summary.SectionIDs).Error; err != nil {
		return summary, err
	}
	if err := tx.Model(&db.StatusPageComponent{}).Where("status_page_id = ?", pageID).Pluck("id", &summary.ComponentIDs).Error; err != nil {
		return summary, err
	}
	if len(summary.ComponentIDs) > 0 {
		if err := tx.Model(&db.StatusPageComponentMapping{}).Where("component_id IN ?", summary.ComponentIDs).Pluck("id", &summary.MappingIDs).Error; err != nil {
			return summary, err
		}
	}
	if err := tx.Model(&db.StatusPageIncident{}).Where("status_page_id = ?", pageID).Pluck("id", &summary.IncidentIDs).Error; err != nil {
		return summary, err
	}
	if err := tx.Model(&db.StatusPageSubscriber{}).Where("status_page_id = ?", pageID).Pluck("id", &summary.SubscriberIDs).Error; err != nil {
		return summary, err
	}
	if err := tx.Model(&db.StatusPageSubscriberDelivery{}).Where("status_page_id = ?", pageID).Pluck("id", &summary.DeliveryIDs).Error; err != nil {
		return summary, err
	}
	return summary, nil
}

func statusPageSectionComponentIDs(tx *gorm.DB, sectionID string) ([]string, error) {
	var ids []string
	if err := tx.Model(&db.StatusPageComponent{}).Where("section_id = ?", sectionID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func pruneStatusPageIncidentComponentIDs(tx *gorm.DB, pageID string, deletedComponentIDs []string) error {
	if len(deletedComponentIDs) == 0 {
		return nil
	}
	deleted := make(map[string]struct{}, len(deletedComponentIDs))
	for _, id := range deletedComponentIDs {
		deleted[id] = struct{}{}
	}

	var incidents []db.StatusPageIncident
	if err := tx.Where("status_page_id = ?", pageID).Find(&incidents).Error; err != nil {
		return err
	}
	for _, incident := range incidents {
		componentIDs := decodeResponseList(incident.AffectedComponentIDs, nil)
		kept := make([]string, 0, len(componentIDs))
		changed := false
		for _, id := range componentIDs {
			if _, ok := deleted[id]; ok {
				changed = true
				continue
			}
			kept = append(kept, id)
		}
		if !changed {
			continue
		}
		if err := tx.Model(&incident).Update("affected_component_ids", encodeStringList(kept)).Error; err != nil {
			return err
		}
	}
	return nil
}
