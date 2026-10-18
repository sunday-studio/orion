package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditEntry is one row of the Audit log: who changed what.
type AuditEntry struct {
	ID    string    `json:"id"`
	At    time.Time `json:"at"`
	Actor string    `json:"actor" example:"admin"`
	// ActorType is user, api_token or system.
	ActorType string `json:"actor_type" enums:"user,api_token,system"`
	// Action is the phrase shown in the log, for example "Resolved incident".
	Action    string `json:"action"`
	ActionKey string `json:"action_key" example:"incident_resolved"`
	// Target names what was changed, for example "INC-0231" or "api-https".
	Target     string `json:"target"`
	ObjectType string `json:"object_type,omitempty"`
	ObjectID   string `json:"object_id,omitempty"`
}

type AuditActor struct {
	Actor string `json:"actor"`
	Count int64  `json:"count"`
}

type AuditListResponse struct {
	Entries    []AuditEntry `json:"entries"`
	NextCursor string       `json:"next_cursor,omitempty"`
	// Actors lists who appears in the log, for the Actor filter.
	Actors []AuditActor `json:"actors"`
}

type auditFilter struct {
	Actor, Text, Action, Object string
	From, To                    time.Time
}

func parseAuditFilter(values url.Values, now time.Time) (auditFilter, error) {
	f := auditFilter{Actor: values.Get("actor"), Text: strings.TrimSpace(values.Get("q")), Action: values.Get("action"), Object: values.Get("object_type")}
	var ok bool
	if f.From, ok = parseLogTime(values.Get("from"), now); !ok {
		return f, errBadTime
	}
	if f.To, ok = parseLogTime(values.Get("to"), now); !ok {
		return f, errBadTime
	}
	return f, nil
}

type timeError string

func (e timeError) Error() string { return string(e) }

const errBadTime = timeError("from and to must be RFC 3339 times or relative offsets like -1h")

func (f auditFilter) apply(q *gorm.DB) *gorm.DB {
	if f.Actor != "" {
		q = q.Where("actor_id = ?", f.Actor)
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.Object != "" {
		q = q.Where("affected_object_type = ?", f.Object)
	}
	if !f.From.IsZero() {
		q = q.Where("created_at >= ?", f.From.UTC())
	}
	if !f.To.IsZero() {
		q = q.Where("created_at <= ?", f.To.UTC())
	}
	if f.Text != "" {
		like := "%" + strings.ToLower(escapeStatusLike(f.Text)) + "%"
		q = q.Where(`(lower(actor_id) LIKE ? ESCAPE '\' OR lower(action) LIKE ? ESCAPE '\' OR lower(metadata_json) LIKE ? ESCAPE '\' OR lower(affected_object_id) LIKE ? ESCAPE '\')`, like, like, like, like)
	}
	return q
}

func (s *Server) auditEntry(e db.AuditEvent) AuditEntry {
	label := ""
	var meta struct {
		TargetLabel string `json:"target_label"`
	}
	_ = json.Unmarshal([]byte(e.MetadataJSON), &meta)
	label = meta.TargetLabel
	if label == "" && e.StatusPageID != "" {
		label = s.auditTargetLabel("status_page", e.StatusPageID)
	}
	if label == "" {
		label = s.auditTargetLabel(e.AffectedObjectType, e.AffectedObjectID)
	}
	object := e.AffectedObjectType
	if e.AffectedObjectType == "" && e.StatusPageID != "" {
		object = "status_page"
	}
	return AuditEntry{ID: e.ID, At: e.CreatedAt.UTC(), Actor: e.ActorID, ActorType: firstNonEmpty(e.ActorType, "system"), Action: auditText(e.Action), ActionKey: e.Action, Target: label, ObjectType: object, ObjectID: e.AffectedObjectID}
}

func (s *Server) listAuditLogImpl(c *gin.Context) {
	f, err := parseAuditFilter(c.Request.URL.Query(), time.Now().UTC())
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	limit := utils.QueryInt(c, "limit", 50)
	if limit < 1 {
		limit = 50
	}
	query := f.apply(s.db.Model(&db.AuditEvent{}))
	if raw := c.Query("cursor"); raw != "" {
		ms, id, err := logstore.DecodeCursor(raw)
		if err != nil {
			utils.BadRequest(c, "Invalid cursor")
			return
		}
		at := time.UnixMilli(ms).UTC()
		query = query.Where("(created_at < ? OR (created_at = ? AND id < ?))", at, at, id)
	}
	var rows []db.AuditEvent
	if err := query.Order("created_at DESC, id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to list the audit log", err)
		return
	}
	out := AuditListResponse{Entries: make([]AuditEntry, 0, len(rows)), Actors: []AuditActor{}}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	for _, r := range rows {
		out.Entries = append(out.Entries, s.auditEntry(r))
	}
	if more && len(rows) > 0 {
		last := rows[len(rows)-1]
		out.NextCursor = logstore.EncodeCursor(last.CreatedAt.UnixMilli(), last.ID)
	}
	var actors []AuditActor
	s.db.Model(&db.AuditEvent{}).Select("actor_id AS actor, COUNT(*) AS count").Where("actor_id <> ''").Group("actor_id").Order("count DESC, actor ASC").Limit(50).Scan(&actors)
	if actors != nil {
		out.Actors = actors
	}
	utils.SuccessResponse(c, http.StatusOK, "Audit log retrieved", out)
}

// auditExportSource streams the audit log for the export job.
func (s *Server) auditExportSource() exportSource {
	return exportSource{
		header: []string{"timestamp", "actor", "actor_type", "action", "target"},
		run: func(ctx context.Context, values url.Values, limit int, emit func([]string, map[string]any) error) error {
			f, err := parseAuditFilter(values, time.Now().UTC())
			if err != nil {
				return err
			}
			var rows []db.AuditEvent
			if err := f.apply(s.db.WithContext(ctx).Model(&db.AuditEvent{})).Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				e := s.auditEntry(r)
				raw, _ := json.Marshal(e)
				full := map[string]any{}
				_ = json.Unmarshal(raw, &full)
				if err := emit([]string{e.At.Format(time.RFC3339Nano), e.Actor, e.ActorType, e.Action, e.Target}, full); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
