package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = "app"
	}
	return slug
}

func (s *Server) uniqueSlug(tx *gorm.DB, name string, exceptID string) (string, error) {
	base := slugify(name)
	slug := base
	for i := 2; ; i++ {
		var count int64
		if err := tx.Model(&db.App{}).Where("slug = ? AND id <> ?", slug, exceptID).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func validateAppRequest(req AppRequest, creating bool) error {
	name := strings.TrimSpace(req.Name)
	if creating && (name == "" || len(name) > 80) {
		return errors.New("name is required and at most 80 characters")
	}
	if !creating && req.Name != "" && (name == "" || len(name) > 80) {
		return errors.New("name must be 1-80 characters")
	}
	if req.RetentionDays != nil && (*req.RetentionDays < 1 || *req.RetentionDays > 3650) {
		return errors.New("retention_days must be between 1 and 3650")
	}
	if req.MaxStorageMB != nil && (*req.MaxStorageMB < 0 || *req.MaxStorageMB > 102400) {
		return errors.New("max_storage_mb must be between 0 and 102400")
	}
	if req.DebugSampleRate != nil && (*req.DebugSampleRate < 0 || *req.DebugSampleRate > 1) {
		return errors.New("debug_sample_rate must be between 0 and 1")
	}
	if req.Redaction != nil && len(req.Redaction.ExtraKeys) > 50 {
		return errors.New("redaction.extra_keys is limited to 50 keys")
	}
	if req.Display != nil && (len(req.Display.Columns) > 50 || len(req.Display.Hidden) > 50) {
		return errors.New("display_profile lists are limited to 50 keys")
	}
	return nil
}

func (s *Server) appResponse(app db.App) App {
	out := App{
		ID: app.ID, Name: app.Name, Slug: app.Slug, Environment: app.Environment, Description: app.Description,
		RetentionDays: app.RetentionDays, MaxStorageMB: app.MaxStorageMB, DebugSampleRate: app.DebugSampleRate,
		CreatedAt: app.CreatedAt, LastIngestAt: app.LastIngestAt,
	}
	_ = json.Unmarshal([]byte(app.DisplayJSON), &out.Display)
	_ = json.Unmarshal([]byte(app.RedactionJSON), &out.Redaction)
	var keys int64
	s.db.Model(&db.AppIngestKey{}).Where("app_id = ? AND revoked_at IS NULL", app.ID).Count(&keys)
	out.KeyCount = int(keys)
	return out
}

func (s *Server) loadApp(c *gin.Context) (db.App, bool) {
	var app db.App
	if err := s.db.Where("id = ?", c.Param("id")).First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "App not found")
		} else {
			utils.InternalError(c, "Failed to load the app", err)
		}
		return db.App{}, false
	}
	return app, true
}

func (s *Server) listAppsImpl(c *gin.Context) {
	var apps []db.App
	if err := s.db.Order("name ASC").Find(&apps).Error; err != nil {
		utils.InternalError(c, "Failed to list apps", err)
		return
	}
	out := make([]App, 0, len(apps))
	for _, app := range apps {
		out = append(out, s.appResponse(app))
	}
	utils.SuccessResponse(c, http.StatusOK, "Apps retrieved successfully", gin.H{"apps": out})
}

func (s *Server) createAppImpl(c *gin.Context) {
	var req AppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	if err := validateAppRequest(req, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	now := time.Now().UTC()
	app := db.App{ID: utils.GenerateID("app"), Name: strings.TrimSpace(req.Name), Environment: strings.TrimSpace(req.Environment), Description: strings.TrimSpace(req.Description),
		RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: now, UpdatedAt: now}
	applyAppRequest(&app, req)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		slug, err := s.uniqueSlug(tx, app.Name, "")
		if err != nil {
			return err
		}
		app.Slug = slug
		return tx.Create(&app).Error
	})
	if err != nil {
		utils.InternalError(c, "Failed to create the app", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "App created", gin.H{"app": s.appResponse(app)})
}

func applyAppRequest(app *db.App, req AppRequest) {
	if req.RetentionDays != nil {
		app.RetentionDays = *req.RetentionDays
	}
	if req.MaxStorageMB != nil {
		app.MaxStorageMB = *req.MaxStorageMB
	}
	if req.DebugSampleRate != nil {
		app.DebugSampleRate = *req.DebugSampleRate
	}
	if req.Display != nil {
		encoded, _ := json.Marshal(req.Display)
		app.DisplayJSON = string(encoded)
	}
	if req.Redaction != nil {
		encoded, _ := json.Marshal(req.Redaction)
		app.RedactionJSON = string(encoded)
	}
}

func (s *Server) getAppImpl(c *gin.Context) {
	if app, ok := s.loadApp(c); ok {
		utils.SuccessResponse(c, http.StatusOK, "App retrieved successfully", gin.H{"app": s.appResponse(app)})
	}
}

func (s *Server) updateAppImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	var update AppUpdateRequest
	if err := c.ShouldBindJSON(&update); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	req := AppRequest(update)
	if err := validateAppRequest(req, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		app.Name = name
	}
	if req.Environment != "" {
		app.Environment = strings.TrimSpace(req.Environment)
	}
	if req.Description != "" {
		app.Description = strings.TrimSpace(req.Description)
	}
	applyAppRequest(&app, req)
	app.UpdatedAt = time.Now().UTC()
	if err := s.db.Save(&app).Error; err != nil {
		utils.InternalError(c, "Failed to update the app", err)
		return
	}
	if s.logs != nil {
		s.logs.ingester.Invalidate() // redaction and sampling changes apply immediately
	}
	utils.SuccessResponse(c, http.StatusOK, "App updated", gin.H{"app": s.appResponse(app)})
}

func (s *Server) deleteAppImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", app.ID).Delete(&db.AppIngestKey{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.App{}, "id = ?", app.ID).Error
	})
	if err != nil {
		utils.InternalError(c, "Failed to delete the app", err)
		return
	}
	if s.logs != nil {
		s.logs.ingester.Invalidate()
		go func(id string) { // entries are removed in the background in short batches
			if _, err := s.logs.store.DeleteApp(id); err != nil {
				s.logger.Error("Failed to delete app logs", "app_id", id, "error", err)
			}
		}(app.ID)
	}
	utils.SuccessResponse(c, http.StatusOK, "App deleted", nil)
}

func toIngestKey(k db.AppIngestKey) IngestKey {
	return IngestKey{ID: k.ID, AppID: k.AppID, Name: k.Name, Prefix: k.Prefix, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt, RevokedAt: k.RevokedAt}
}

func (s *Server) listIngestKeysImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	var keys []db.AppIngestKey
	if err := s.db.Where("app_id = ?", app.ID).Order("created_at DESC").Find(&keys).Error; err != nil {
		utils.InternalError(c, "Failed to list ingest keys", err)
		return
	}
	out := make([]IngestKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, toIngestKey(k))
	}
	utils.SuccessResponse(c, http.StatusOK, "Ingest keys retrieved successfully", gin.H{"keys": out})
}

func (s *Server) createIngestKeyImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	var req IngestKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || len(req.Name) > 80 {
		utils.BadRequest(c, "name is required (at most 80 characters)")
		return
	}
	key, hash, prefix, err := logstore.GenerateKey()
	if err != nil {
		utils.InternalError(c, "Failed to generate a key", err)
		return
	}
	row := db.AppIngestKey{ID: utils.GenerateID("ingest_key"), AppID: app.ID, Name: strings.TrimSpace(req.Name), Prefix: prefix, KeyHash: hash, CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&row).Error; err != nil {
		utils.InternalError(c, "Failed to store the key", err)
		return
	}
	s.logger.Info("Ingest key created", "app_id", app.ID, "key_id", row.ID, "actor", frontendActorID(c))
	utils.SuccessResponse(c, http.StatusCreated, "Ingest key created. Store it now: it cannot be shown again.", gin.H{"key": IngestKeyCreated{IngestKey: toIngestKey(row), Key: key}})
}

func (s *Server) revokeIngestKeyImpl(c *gin.Context) {
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	now := time.Now().UTC()
	res := s.db.Model(&db.AppIngestKey{}).Where("id = ? AND app_id = ? AND revoked_at IS NULL", c.Param("key_id"), app.ID).Update("revoked_at", now)
	if res.Error != nil {
		utils.InternalError(c, "Failed to revoke the key", res.Error)
		return
	}
	if res.RowsAffected == 0 {
		utils.NotFound(c, "Ingest key not found")
		return
	}
	if s.logs != nil {
		s.logs.ingester.Invalidate()
	}
	s.logger.Info("Ingest key revoked", "app_id", app.ID, "key_id", c.Param("key_id"), "actor", frontendActorID(c))
	utils.SuccessResponse(c, http.StatusOK, "Ingest key revoked", nil)
}

func (s *Server) sendAppTestLogImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, ok := s.loadApp(c)
	if !ok {
		return
	}
	now := time.Now().UTC()
	entry := logstore.Entry{
		ID: logstore.NewEntryID(now), AppID: app.ID, Timestamp: now, IngestedAt: now, Severity: "info", SeverityNumber: 9,
		Event: "orion.test", Body: "Test log from Orion: setup works.", Attributes: map[string]any{"sent_by": frontendActorID(c)}, Size: 128,
	}
	if err := s.logs.store.Enqueue([]logstore.Entry{entry}); err != nil {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest queue is full; try again shortly", nil)
		return
	}
	utils.SuccessResponse(c, http.StatusAccepted, "Test log queued", TestLogResponse{Accepted: true, EntryID: entry.ID})
}
