package api

import (
	"bytes"
	"encoding/json"
	"io"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// auditRoute describes one admin action worth recording.
type auditRoute struct {
	Action string // stored action key
	Text   string // past tense phrase shown in the Audit log
	Object string // what it acts on: incident, monitor, server, alert_channel, ...
	Param  string // path parameter holding the object id ("" when the id is unknown until creation)
}

const auditMaxBody = 1 << 20

// auditMiddleware records mapped admin actions after they succeeded.
func (s *Server) auditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.auditRouteProbe && c.GetHeader("X-Orion-Route-Probe") == "1" {
			c.AbortWithStatusJSON(200, gin.H{"admin_route": c.FullPath()})
			return
		}
		method := c.Request.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			c.Next()
			return
		}
		route, mapped := auditRoutes[method+" "+c.FullPath()]
		var body []byte
		var capture *captureWriter
		if mapped && route.Param == "" {
			// Creates have no id yet: keep the request body to name the target and the response to read the new id.
			if c.Request.Body != nil {
				original := c.Request.Body
				body, _ = io.ReadAll(io.LimitReader(original, auditMaxBody))
				// Anything past the cap still reaches the handler, so its own body limit answers (413), not a truncated JSON error.
				c.Request.Body = struct {
					io.Reader
					io.Closer
				}{io.MultiReader(bytes.NewReader(body), original), original}
			}
			capture = &captureWriter{ResponseWriter: c.Writer}
			c.Writer = capture
		}
		c.Next()
		if !mapped || c.Writer.Status() >= 400 {
			return
		}
		createdID := ""
		if capture != nil {
			createdID = capture.createdID()
		}
		s.recordAudit(c, route, body, createdID)
	}
}

func (s *Server) recordAudit(c *gin.Context, route auditRoute, body []byte, createdID string) {
	id, label := "", ""
	if route.Param != "" {
		id = c.Param(route.Param)
		label = s.auditTargetLabel(route.Object, id)
	} else {
		var named struct {
			Name  string `json:"name"`
			Title string `json:"title"`
		}
		_ = json.Unmarshal(body, &named)
		id = createdID
		label = firstNonEmpty(named.Name, named.Title, s.auditTargetLabel(route.Object, createdID))
		if id == "" {
			id = firstNonEmpty(label, route.Action)
		}
	}
	actorType := "user"
	if kind, _ := c.Get(apiTokenKindKey); kind == "api_token" {
		actorType = "api_token"
	}
	metadata, _ := json.Marshal(map[string]any{"target_label": label, "method": c.Request.Method, "route": c.FullPath()})
	pageID := ""
	if route.Object == "status_page" {
		pageID = id
	}
	event := db.AuditEvent{ID: utils.GenerateID("audit"), Action: route.Action, StatusPageID: pageID, AffectedObjectType: route.Object, AffectedObjectID: id, ActorType: actorType, ActorID: frontendActorID(c), MetadataJSON: string(metadata), CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&event).Error; err != nil {
		s.logger.Error("Failed to record audit event", "action", route.Action, "error", err)
	}
}

// auditTargetLabel names an object for display ("INC-0231", "api-https", "edge-01").
func (s *Server) auditTargetLabel(object, id string) string {
	if id == "" {
		return ""
	}
	var label string
	switch object {
	case "incident":
		var inc db.Incident
		if s.db.Select("number", "title").Where("id = ?", id).Limit(1).Find(&inc).RowsAffected > 0 {
			label = firstNonEmpty(incidentDisplayID(inc.Number), inc.Title)
		}
	case "monitor":
		var m db.Monitor
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&m).RowsAffected > 0 {
			label = m.Name
		}
	case "node":
		var a db.Node
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&a).RowsAffected > 0 {
			label = a.Name
		}
	case "alert_channel":
		var ch db.AlertChannel
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&ch).RowsAffected > 0 {
			label = ch.Name
		}
	case "alert_rule":
		var r db.AlertRoute
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&r).RowsAffected > 0 {
			label = r.Name
		}
	case "status_page":
		var p db.StatusPage
		if s.db.Select("title").Where("id = ?", id).Limit(1).Find(&p).RowsAffected > 0 {
			label = p.Title
		}
	case "app":
		var a db.App
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&a).RowsAffected > 0 {
			label = a.Name
		}
	case "alert_route":
		var r db.AlertLabelRoute
		if s.db.Select("name", "matchers").Where("id = ?", id).Limit(1).Find(&r).RowsAffected > 0 {
			label = firstNonEmpty(r.Name, "Route "+id)
		}
	case "log_alert":
		var r db.LogAlertRule
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&r).RowsAffected > 0 {
			label = r.Name
		}
	case "api_token":
		var t db.APIToken
		if s.db.Select("name").Where("id = ?", id).Limit(1).Find(&t).RowsAffected > 0 {
			label = t.Name
		}
	}
	return firstNonEmpty(label, id)
}

// auditTexts maps stored action keys (including those written by services) to the phrase shown to people.
var auditTexts = map[string]string{
	"node_token_rotated":              "Rotated server token",
	"node_token_revoked":              "Revoked server token",
	"node_token_reissued":             "Reissued server token",
	"node_decommissioned":             "Removed server",
	"data_lifecycle_settings_updated": "Updated data lifecycle settings",
	"data_lifecycle_rollup_run":       "Ran rollup",
	"data_lifecycle_archive_run":      "Ran archive",
	"status_page_published":           "Published status page",
	"status_page_unpublished":         "Unpublished status page",
	"status_page_deleted":             "Deleted status page",
}

func auditText(action string) string {
	if t, ok := auditTexts[action]; ok {
		return t
	}
	for _, r := range auditRoutes {
		if r.Action == action {
			return r.Text
		}
	}
	text := strings.ReplaceAll(action, "_", " ")
	if text == "" {
		return "Changed something"
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// captureWriter keeps the start of a response so the id of a created object can be read from it.
type captureWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.buf.Len() < 64<<10 {
		w.buf.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	if w.buf.Len() < 64<<10 {
		w.buf.WriteString(s)
	}
	return w.ResponseWriter.WriteString(s)
}

// createdID finds data.id, or the id of the first object inside data (data.monitor.id, data.page.id, ...).
func (w *captureWriter) createdID() string {
	var env struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if json.Unmarshal(w.buf.Bytes(), &env) != nil {
		return ""
	}
	if raw, ok := env.Data["id"]; ok {
		var id string
		if json.Unmarshal(raw, &id) == nil {
			return id
		}
	}
	keys := make([]string, 0, len(env.Data))
	for k := range env.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var inner struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(env.Data[k], &inner) == nil && inner.ID != "" {
			return inner.ID
		}
	}
	return ""
}
