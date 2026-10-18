package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func toStoreRules(rules []RedactionRule) []logstore.Rule {
	out := make([]logstore.Rule, 0, len(rules))
	for _, r := range rules {
		out = append(out, logstore.Rule{Type: r.Type, Pattern: r.Pattern, ReplaceWith: r.ReplaceWith})
	}
	return out
}

func (s *Server) rotateIngestKeyImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	var old db.AppIngestKey
	if err := s.db.Where("id = ? AND app_id = ? AND revoked_at IS NULL", c.Param("key_id"), app.ID).First(&old).Error; err != nil {
		utils.NotFound(c, "Ingest key not found")
		return
	}
	key, hash, prefix, err := logstore.GenerateKey()
	if err != nil {
		utils.InternalError(c, "Failed to generate a key", err)
		return
	}
	now := time.Now().UTC()
	row := db.AppIngestKey{ID: utils.GenerateID("ingest_key"), AppID: app.ID, Name: old.Name, Prefix: prefix, KeyHash: hash, CreatedAt: now}
	// The old key keeps working for ten minutes so a deploy can pick up the new one without dropping logs.
	revokeAt := now.Add(10 * time.Minute)
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return tx.Model(&db.AppIngestKey{}).Where("id = ?", old.ID).Update("revoked_at", revokeAt).Error
	})
	if err != nil {
		utils.InternalError(c, "Failed to rotate the key", err)
		return
	}
	s.logger.Info("Ingest key rotated", "app_id", app.ID, "old_key_id", old.ID, "key_id", row.ID, "actor", frontendActorID(c))
	utils.SuccessResponse(c, http.StatusCreated, "Ingest key rotated. Store the new key now: it cannot be shown again. The old key works for 10 more minutes.", gin.H{"key": IngestKeyCreated{IngestKey: toIngestKey(row), Key: key}, "previous_key_valid_until": revokeAt})
}

func (s *Server) purgeAppLogsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	deleted, err := s.logs.store.DeleteApp(app.ID)
	if err != nil {
		utils.InternalError(c, "Failed to delete the app's logs", err)
		return
	}
	s.db.Model(&db.App{}).Where("id = ?", app.ID).Update("last_ingest_at", nil)
	s.logger.Info("App logs purged", "app_id", app.ID, "deleted", deleted, "actor", frontendActorID(c))
	utils.SuccessResponse(c, http.StatusOK, "Logs deleted", gin.H{"deleted": deleted})
}

func (s *Server) testAppRedactionImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	var req RedactionTestRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Sample == nil {
		utils.BadRequest(c, "sample is required")
		return
	}
	var settings RedactionSettings
	if req.Redaction != nil {
		settings = *req.Redaction
		if err := logstore.ValidateRules(toStoreRules(settings.Rules)); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		if len(settings.ExtraKeys) > 50 {
			utils.BadRequest(c, "extra_keys is limited to 50 keys")
			return
		}
	} else {
		_ = json.Unmarshal([]byte(app.RedactionJSON), &settings)
	}
	if encoded, _ := json.Marshal(req.Sample); len(encoded) > 16<<10 {
		utils.BadRequest(c, "sample is limited to 16 KB")
		return
	}
	redactor := logstore.NewRedactorRules(settings.ExtraKeys, settings.DisableDefaults, toStoreRules(settings.Rules))
	result := redactor.Redact(deepCopyJSON(req.Sample))
	utils.SuccessResponse(c, http.StatusOK, "Redaction tested. Nothing was saved.", RedactionTestResponse{Redacted: result, Changed: !reflect.DeepEqual(result, req.Sample)})
}

// deepCopyJSON copies a decoded JSON value so redaction never touches the request.
func deepCopyJSON(v any) any {
	encoded, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if json.Unmarshal(encoded, &out) != nil {
		return v
	}
	return out
}

func (s *Server) getRedactionDefaultsImpl(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Default redaction keys retrieved", RedactionDefaultsResponse{Keys: logstore.DefaultKeys()})
}
