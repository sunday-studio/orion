package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var errPublicTitleRequired = errors.New("public title required")

// IncidentPublicTarget is one status page an incident can be posted to.
type IncidentPublicTarget struct {
	StatusPageID    string `json:"status_page_id"`
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	Visibility      string `json:"visibility"`
	SubscriberCount int64  `json:"subscriber_count"`
	// LinkedIncidentID is the status page incident already tied to this incident, if any.
	LinkedIncidentID string `json:"linked_incident_id,omitempty"`
	PublishedUpdates int64  `json:"published_updates"`
	LatestStatus     string `json:"latest_status,omitempty"`
	// TitleRequired is true when the first post must carry a public title.
	TitleRequired bool `json:"title_required"`
}

// getIncidentPublicTargets lists the status pages for the "Post public update" dialog.
// @Summary      Status pages for an incident public update
// @Description  Lists status pages with confirmed subscriber counts and the update state already linked to this incident.
// @Tags         incidents
// @Produce      json
// @ID           getIncidentPublicTargets
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{targets=[]IncidentPublicTarget}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/public-update [get]
func (s *Server) getIncidentPublicTargets(c *gin.Context) {
	incident, _, _, ok := s.loadIncidentContext(c)
	if !ok {
		return
	}
	var pages []db.StatusPage
	if err := s.db.Order("title ASC").Find(&pages).Error; err != nil {
		utils.InternalError(c, "Failed to load status pages", err)
		return
	}
	targets := make([]IncidentPublicTarget, 0, len(pages))
	for _, page := range pages {
		t := IncidentPublicTarget{StatusPageID: page.ID, Title: page.Title, Slug: page.Slug, Visibility: page.Visibility}
		s.db.Model(&db.StatusPageSubscriber{}).Where("status_page_id = ? AND state = ?", page.ID, statusPageSubscriberStateConfirmed).Count(&t.SubscriberCount)
		var linked db.StatusPageIncident
		if s.db.Where("status_page_id = ? AND internal_incident_id = ?", page.ID, incident.ID).Limit(1).Find(&linked).RowsAffected > 0 {
			t.LinkedIncidentID, t.LatestStatus = linked.ID, linked.PublicStatus
			s.db.Model(&db.StatusPageIncidentUpdate{}).Where("incident_id = ? AND published_at IS NOT NULL", linked.ID).Count(&t.PublishedUpdates)
		}
		t.TitleRequired = t.LinkedIncidentID == "" && incident.Source != "manual"
		targets = append(targets, t)
	}
	utils.SuccessResponse(c, http.StatusOK, "Status pages retrieved", gin.H{"targets": targets})
}

type IncidentPublicUpdateRequest struct {
	StatusPageID string `json:"status_page_id" binding:"required"`
	Status       string `json:"status" binding:"required" enums:"investigating,identified,monitoring,resolved"`
	Message      string `json:"message" binding:"required"`
	// Title is the public headline, used when the first update creates the status page incident.
	// Required for incidents Orion opened itself, because their titles name internal monitors and hosts.
	Title string `json:"title,omitempty" example:"Elevated API errors"`
	// NotifySubscribers emails confirmed subscribers whose component preferences match.
	NotifySubscribers bool `json:"notify_subscribers"`
}

// postIncidentPublicUpdate publishes an update for an incident on a status page.
// @Summary      Post a public update for an incident
// @Description  Creates (or reuses) the status page incident linked to this incident and publishes an update. Affected components come from the incident's impacted components that belong to that page.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           postIncidentPublicUpdate
// @Param        id       path      string                       true  "Incident ID"
// @Param        request  body      IncidentPublicUpdateRequest  true  "Update"
// @Success      201      {object}  utils.APIResponse{data=object{update=StatusPageIncidentUpdateResponse,status_page_incident_id=string,subscribers_notified=int}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/public-updates [post]
func (s *Server) postIncidentPublicUpdate(c *gin.Context) {
	incident, _, _, ok := s.loadIncidentContext(c)
	if !ok {
		return
	}
	var request IncidentPublicUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "status_page_id, status and message are required")
		return
	}
	status, message := strings.TrimSpace(request.Status), strings.TrimSpace(request.Message)
	if !validStatusPageIncidentStatus(status) || status == "scheduled" || message == "" || len(message) > 8000 {
		utils.BadRequest(c, "status must be investigating, identified, monitoring or resolved and message is required")
		return
	}
	publicTitle := strings.TrimSpace(request.Title)
	if len(publicTitle) > 200 {
		utils.BadRequest(c, "title is too long")
		return
	}
	var page db.StatusPage
	if err := s.db.Where("id = ?", request.StatusPageID).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "Status page not found")
			return
		}
		utils.InternalError(c, "Failed to load status page", err)
		return
	}

	now := time.Now().UTC()
	var update db.StatusPageIncidentUpdate
	var public db.StatusPageIncident
	var notified int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Where("status_page_id = ? AND internal_incident_id = ?", page.ID, incident.ID).Limit(1).Find(&public).RowsAffected == 0 {
			if publicTitle == "" && incident.Source != "manual" {
				return errPublicTitleRequired
			}
			if publicTitle == "" {
				publicTitle = incident.Title
			}
			severity := incident.Severity
			if !validStatusPageSeverity(severity) {
				severity = "medium"
			}
			public = db.StatusPageIncident{ID: utils.GenerateID("status_page_incident"), StatusPageID: page.ID, InternalIncidentID: incident.ID, Title: publicTitle,
				PublicStatus: status, Severity: severity, ImpactSummary: message, Visibility: statusPageIncidentVisibilityDraft, AffectedComponentIDs: s.pageComponentIDsFor(tx, page.ID, incident)}
			if err := tx.Create(&public).Error; err != nil {
				return err
			}
		}
		update = db.StatusPageIncidentUpdate{ID: utils.GenerateID("status_page_update"), IncidentID: public.ID, Status: status, Message: message, CreatedBy: frontendActorID(c), PublishedAt: &now}
		if err := tx.Create(&update).Error; err != nil {
			return err
		}
		public.PublicStatus = status
		if public.Visibility == statusPageIncidentVisibilityDraft {
			public.Visibility, public.PublishedAt = statusPageIncidentVisibilityPublished, &now
		}
		if status == "resolved" && public.ResolvedAt == nil {
			public.ResolvedAt = &now
		} else if status != "resolved" {
			public.ResolvedAt = nil
		}
		if err := tx.Save(&public).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{Action: service.StatusPageAuditActionPublicIncidentUpdateCreated, StatusPageID: page.ID, AffectedObjectType: "public_incident_update", AffectedObjectID: update.ID}); err != nil {
			return err
		}
		if request.NotifySubscribers {
			if err := s.enqueueStatusPageSubscriberIncidentUpdateDeliveries(tx, public, update); err != nil {
				return err
			}
			tx.Model(&db.StatusPageSubscriberDelivery{}).Where("public_incident_update_id = ?", update.ID).Count(&notified)
		}
		return service.NewIncidentService(tx, s.logger, s.cfg).RecordPublicUpdate(incident.ID, page.Title, status, s.incidentLifecycleActionMetadata(c, message))
	})
	if errors.Is(err, errPublicTitleRequired) {
		utils.BadRequest(c, "title is required for the first public update of an incident Orion opened itself, because its title can name internal systems")
		return
	}
	if err != nil {
		s.logger.Error("Failed to post incident public update", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to post the public update", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Public update posted", gin.H{"update": statusPageIncidentUpdateResponse(update), "status_page_incident_id": public.ID, "subscribers_notified": notified})
}

// pageComponentIDsFor returns the incident's impacted components that exist on the page, as the JSON the model stores.
func (s *Server) pageComponentIDsFor(tx *gorm.DB, pageID string, incident db.Incident) string {
	var wanted []string
	for _, impact := range incidentComponentImpactResponses(incident.ImpactedComponents) {
		if impact.ComponentID != "" {
			wanted = append(wanted, impact.ComponentID)
		}
	}
	ids := []string{}
	if len(wanted) > 0 {
		tx.Model(&db.StatusPageComponent{}).Where("status_page_id = ? AND id IN ?", pageID, wanted).Pluck("id", &ids)
	}
	raw, _ := json.Marshal(ids)
	return string(raw)
}
