package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type DeclareIncidentRequest struct {
	Title     string `json:"title" binding:"required"`
	Severity  string `json:"severity" binding:"required" enums:"low,medium,high,critical"`
	MonitorID string `json:"monitor_id,omitempty"`
	NodeID    string `json:"node_id,omitempty"`
	Summary   string `json:"summary,omitempty"`
	// Notify queues alerts through the channels the matching rules select.
	Notify bool `json:"notify"`
}

// declareIncident opens an incident by hand.
// @Summary      Declare incident
// @Description  Open an incident manually for something Orion has not noticed. Give a monitor or a server. A monitor can have only one active incident.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           declareIncident
// @Param        request  body      DeclareIncidentRequest  true  "Incident"
// @Success      201      {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/incidents [post]
func (s *Server) declareIncident(c *gin.Context) {
	var request DeclareIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "title and severity are required")
		return
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).DeclareIncident(service.DeclareIncidentInput{Title: request.Title, Severity: request.Severity, MonitorID: request.MonitorID, NodeID: request.NodeID, Summary: request.Summary, Notify: request.Notify}, s.incidentLifecycleActionMetadata(c, ""))
	switch {
	case err == nil:
		var node db.Node
		s.db.Where("id = ?", incident.NodeID).First(&node)
		var monitor db.Monitor
		s.db.Where("id = ?", incident.MonitorID).First(&monitor)
		utils.SuccessResponse(c, http.StatusCreated, "Incident declared", gin.H{"incident": incidentResponse(incident, node, monitor)})
	case errors.Is(err, service.ErrIncidentInvalid):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrIncidentMonitorMissing), errors.Is(err, service.ErrIncidentNodeMissing):
		utils.NotFound(c, err.Error())
	case errors.Is(err, service.ErrIncidentAlreadyActive):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	default:
		s.logger.Error("Failed to declare incident", "error", err)
		utils.InternalError(c, "Failed to declare incident", err)
	}
}

type IncidentNoteRequest struct {
	Text string `json:"text" binding:"required"`
}

// addIncidentNote adds a note to the incident timeline.
// @Summary      Add incident note
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           addIncidentNote
// @Param        id       path      string               true  "Incident ID"
// @Param        request  body      IncidentNoteRequest  true  "Note"
// @Success      201      {object}  utils.APIResponse
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/notes [post]
func (s *Server) addIncidentNote(c *gin.Context) {
	var request IncidentNoteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "text is required")
		return
	}
	err := service.NewIncidentService(s.db, s.logger, s.cfg).AddIncidentNote(c.Param("id"), request.Text, s.incidentLifecycleActionMetadata(c, ""))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusCreated, "Note added", gin.H{})
	case errors.Is(err, service.ErrIncidentInvalid):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrIncidentNotFound):
		utils.NotFound(c, "Incident not found")
	default:
		utils.InternalError(c, "Failed to add note", err)
	}
}

type IncidentResponderRequest struct {
	// Name of the person on point; empty clears it.
	Name string `json:"name"`
}

// setIncidentResponder assigns who is handling the incident.
// @Summary      Assign incident responder
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           setIncidentResponder
// @Param        id       path      string                    true  "Incident ID"
// @Param        request  body      IncidentResponderRequest  true  "Responder"
// @Success      200      {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/responder [put]
func (s *Server) setIncidentResponder(c *gin.Context) {
	var request IncidentResponderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid responder payload")
		return
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).SetIncidentResponder(c.Param("id"), request.Name, s.incidentLifecycleActionMetadata(c, ""))
	if errors.Is(err, service.ErrIncidentInvalid) {
		utils.BadRequest(c, err.Error())
		return
	}
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to set responder")
		return
	}
	s.writeIncidentActionResponse(c, "Responder updated", incident)
}

// coverIncidentWith resolves "covered by" (id or INC number) and covers the incident.
func (s *Server) coverIncidentWith(id string, request incidentCoverageRequest, meta service.IncidentLifecycleActionMetadata) (db.Incident, error) {
	var covering db.Incident
	if ref := strings.TrimSpace(request.CoveredBy); ref != "" {
		query := s.db.Where("id = ?", ref)
		if n, err := strconv.Atoi(strings.TrimLeft(strings.ToUpper(ref), "INC-# ")); err == nil && n > 0 {
			query = s.db.Where("id = ? OR number = ?", ref, n)
		}
		if err := query.First(&covering).Error; err != nil || covering.ID == id {
			return db.Incident{}, errors.Join(service.ErrIncidentInvalid, errors.New("covered_by must be another existing incident"))
		}
		if request.Note == "" {
			request.Note = "Covered by " + incidentDisplayID(covering.Number)
			meta.Note = request.Note
		}
	}
	return service.NewIncidentService(s.db, s.logger, s.cfg).CoverIncidentBy(id, request.CoveredUntil, request.Note, covering.ID, meta)
}

type BulkIncidentRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	Action string   `json:"action" binding:"required" enums:"acknowledge,cover,resolve"`
	Note   string   `json:"note,omitempty"`
	// CoveredBy applies to the cover action.
	CoveredBy string `json:"covered_by,omitempty"`
}

type BulkIncidentResult struct {
	ID       string            `json:"id"`
	OK       bool              `json:"ok"`
	Error    string            `json:"error,omitempty"`
	Incident *IncidentResponse `json:"incident,omitempty"`
}

const maxBulkIncidents = 100

// bulkIncidentAction applies one lifecycle action to many incidents.
// @Summary      Bulk incident action
// @Description  Acknowledge, cover or resolve up to 100 incidents. Each id succeeds or fails on its own; the response lists every result.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           bulkIncidentAction
// @Param        request  body      BulkIncidentRequest  true  "Bulk action"
// @Success      200      {object}  utils.APIResponse{data=object{results=[]BulkIncidentResult,succeeded=int,failed=int}}
// @Failure      400      {object}  utils.APIResponse
// @Router       /v1/incidents/bulk [post]
func (s *Server) bulkIncidentAction(c *gin.Context) {
	var request BulkIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil || len(request.IDs) == 0 || len(request.IDs) > maxBulkIncidents {
		utils.BadRequest(c, "ids is required (at most 100)")
		return
	}
	if request.Action != "acknowledge" && request.Action != "cover" && request.Action != "resolve" {
		utils.BadRequest(c, "action must be acknowledge, cover or resolve")
		return
	}
	svc := service.NewIncidentService(s.db, s.logger, s.cfg)
	meta := s.incidentLifecycleActionMetadata(c, request.Note)
	results := make([]BulkIncidentResult, 0, len(request.IDs))
	succeeded := 0
	seen := map[string]bool{}
	for _, id := range request.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var incident db.Incident
		var err error
		switch request.Action {
		case "acknowledge":
			incident, err = svc.AcknowledgeIncident(id, meta)
		case "resolve":
			incident, err = svc.ResolveIncident(id, meta)
		case "cover":
			incident, err = s.coverIncidentWith(id, incidentCoverageRequest{Note: request.Note, CoveredBy: request.CoveredBy}, meta)
		}
		result := BulkIncidentResult{ID: id, OK: err == nil}
		if err != nil {
			result.Error = bulkIncidentError(err)
		} else {
			succeeded++
			var node db.Node
			s.db.Where("id = ?", incident.NodeID).First(&node)
			var monitor db.Monitor
			s.db.Where("id = ?", incident.MonitorID).First(&monitor)
			r := incidentResponse(incident, node, monitor)
			result.Incident = &r
		}
		results = append(results, result)
	}
	utils.SuccessResponse(c, http.StatusOK, "Bulk action finished", gin.H{"results": results, "succeeded": succeeded, "failed": len(results) - succeeded})
}

func bulkIncidentError(err error) string {
	switch {
	case errors.Is(err, service.ErrIncidentNotFound):
		return "Incident not found"
	case errors.Is(err, service.ErrIncidentAlreadyResolved):
		return "Incident is already resolved"
	case errors.Is(err, service.ErrIncidentInvalid):
		return err.Error()
	default:
		return "Action failed"
	}
}
