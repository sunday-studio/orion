package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"orion/core/internal/buildinfo"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// loadInstanceSettings returns the single settings row, creating it with defaults on first use.
func (s *Server) loadInstanceSettings() (db.InstanceSettings, error) {
	row := db.InstanceSettings{ID: 1, Timezone: "UTC", TimeFormat: "24"}
	err := s.db.Where("id = 1").First(&row).Error
	if err == gorm.ErrRecordNotFound {
		err = s.db.Create(&row).Error
	}
	return row, err
}

// applyInstanceSettings makes the saved Public Core URL effective for this process.
func (s *Server) applyInstanceSettings() {
	if row, err := s.loadInstanceSettings(); err == nil {
		s.cfg.SetPublicURLOverride(row.PublicURL)
	}
}

// effectiveRegistrationToken is the secret agents register with: the one rotated in Settings, else ORION_ENROLLMENT_TOKEN.
func (s *Server) effectiveRegistrationToken() (token string, source string, rotatedAt *time.Time) {
	if row, err := s.loadInstanceSettings(); err == nil && row.RegistrationToken != "" {
		return row.RegistrationToken, "settings", row.RegistrationTokenRotatedAt
	}
	if s.cfg.EnrollmentToken != "" {
		return s.cfg.EnrollmentToken, "environment", nil
	}
	return "", "none", nil
}

func (s *Server) enrollmentTokenValid(presented string) bool {
	expected, _, _ := s.effectiveRegistrationToken()
	if expected == "" {
		return true
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}

// GeneralSettings are the instance-wide values shown in emails, status pages and the Console.
type GeneralSettings struct {
	InstanceName string `json:"instance_name" example:"Acme Orion"`
	PublicURL    string `json:"public_url" example:"https://orion.acme.test"`
	// PublicURLSource is settings, environment or empty.
	PublicURLSource string    `json:"public_url_source,omitempty" enums:"settings,environment"`
	Timezone        string    `json:"timezone" example:"UTC"`
	TimeFormat      string    `json:"time_format" enums:"12,24"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type GeneralSettingsRequest struct {
	InstanceName *string `json:"instance_name"`
	PublicURL    *string `json:"public_url"`
	Timezone     *string `json:"timezone"`
	TimeFormat   *string `json:"time_format"`
}

func (s *Server) generalSettingsResponse(row db.InstanceSettings) GeneralSettings {
	out := GeneralSettings{InstanceName: row.InstanceName, PublicURL: s.cfg.PublicBaseURL(), Timezone: row.Timezone, TimeFormat: row.TimeFormat, UpdatedAt: row.UpdatedAt}
	switch {
	case row.PublicURL != "":
		out.PublicURLSource = "settings"
	case s.cfg.PublicURL != "":
		out.PublicURLSource = "environment"
	}
	return out
}

func (s *Server) getGeneralSettingsImpl(c *gin.Context) {
	row, err := s.loadInstanceSettings()
	if err != nil {
		utils.InternalError(c, "Failed to load settings", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Settings retrieved", s.generalSettingsResponse(row))
}

func (s *Server) updateGeneralSettingsImpl(c *gin.Context) {
	var req GeneralSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid settings payload")
		return
	}
	row, err := s.loadInstanceSettings()
	if err != nil {
		utils.InternalError(c, "Failed to load settings", err)
		return
	}
	if req.InstanceName != nil {
		name := strings.TrimSpace(*req.InstanceName)
		if len([]rune(name)) > 80 {
			utils.BadRequest(c, "instance_name must be 80 characters or fewer")
			return
		}
		row.InstanceName = name
	}
	if req.PublicURL != nil {
		value := strings.TrimRight(strings.TrimSpace(*req.PublicURL), "/")
		if value != "" {
			u, err := url.Parse(value)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(value) > 300 {
				utils.BadRequest(c, "public_url must be an http or https address without credentials, query or fragment")
				return
			}
		}
		row.PublicURL = value
	}
	if req.Timezone != nil {
		zone := strings.TrimSpace(*req.Timezone)
		if _, err := time.LoadLocation(zone); err != nil || zone == "" || len(zone) > 64 {
			utils.BadRequest(c, "timezone must be an IANA time zone such as Europe/London")
			return
		}
		row.Timezone = zone
	}
	if req.TimeFormat != nil {
		if *req.TimeFormat != "12" && *req.TimeFormat != "24" {
			utils.BadRequest(c, "time_format must be 12 or 24")
			return
		}
		row.TimeFormat = *req.TimeFormat
	}
	row.UpdatedBy, row.UpdatedAt = frontendActorID(c), time.Now().UTC()
	if err := s.db.Save(&row).Error; err != nil {
		utils.InternalError(c, "Failed to save settings", err)
		return
	}
	s.cfg.SetPublicURLOverride(row.PublicURL)
	utils.SuccessResponse(c, http.StatusOK, "Settings saved", s.generalSettingsResponse(row))
}

// SystemSettings is the read-only System tab plus the registration token.
type SystemSettings struct {
	Version       string    `json:"version" example:"0.9.2"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
	SignIn        struct {
		AuthEnabled  bool   `json:"auth_enabled"`
		Username     string `json:"username"`
		SessionHours int    `json:"session_hours"`
	} `json:"sign_in"`
	Registration struct {
		// Configured is false when agents can register without a token.
		Configured bool `json:"configured"`
		// Token is the secret itself; it is returned to Console sessions only, never to API tokens.
		Token string `json:"token,omitempty"`
		// TokenMask is the token with its middle hidden, shown to API tokens.
		TokenMask string     `json:"token_mask,omitempty"`
		Source    string     `json:"source" enums:"settings,environment,none"`
		RotatedAt *time.Time `json:"rotated_at,omitempty"`
	} `json:"registration"`
	Worker struct {
		// Status is running, stale or missing.
		Status      string     `json:"status" enums:"running,stale,missing"`
		Healthy     bool       `json:"healthy"`
		LastTickAt  *time.Time `json:"last_tick_at,omitempty"`
		Version     string     `json:"version,omitempty"`
		WorkerCount int        `json:"worker_count"`
		LastError   string     `json:"last_error,omitempty"`
	} `json:"worker"`
	Database struct {
		Engine    string `json:"engine" example:"SQLite"`
		SizeBytes int64  `json:"size_bytes"`
	} `json:"database"`
	Backups struct {
		Note     string `json:"note"`
		GuideURL string `json:"guide_url"`
	} `json:"backups"`
}

func (s *Server) getSystemSettingsImpl(c *gin.Context) {
	now := time.Now().UTC()
	var out SystemSettings
	out.Version = buildinfo.Version
	if snap, err := s.runtimeDiagnosticsService.Snapshot(c.Request.Context()); err == nil {
		out.StartedAt = snap.StartedAt
		out.UptimeSeconds = int64(now.Sub(snap.StartedAt).Seconds())
	}
	out.SignIn.AuthEnabled, out.SignIn.Username, out.SignIn.SessionHours = s.cfg.FrontendAuthOn, s.cfg.AdminUsername, s.cfg.SessionHours
	token, source, rotated := s.effectiveRegistrationToken()
	out.Registration.Configured, out.Registration.Source, out.Registration.RotatedAt = token != "", source, rotated
	out.Registration.TokenMask = maskToken(token)
	if !isAPITokenCaller(c) {
		out.Registration.Token = token
	}
	out.Worker.Status = "missing"
	if diag, err := s.workerDiagnosticsService.GetDiagnostics(c.Request.Context(), time.Duration(s.cfg.CoreWorkerStaleSeconds)*time.Second, now); err == nil && len(diag.Workers) > 0 {
		out.Worker.WorkerCount = diag.WorkerCount
		latest := diag.Workers[0]
		for _, w := range diag.Workers {
			if w.LastHeartbeatAt.After(latest.LastHeartbeatAt) {
				latest = w
			}
		}
		at := latest.LastHeartbeatAt
		out.Worker.LastTickAt, out.Worker.Version, out.Worker.LastError = &at, latest.Version, latest.LastError
		out.Worker.Healthy = diag.OnlineCount > 0
		out.Worker.Status = "running"
		if diag.OnlineCount == 0 {
			out.Worker.Status = "stale"
		}
	}
	out.Database.Engine = "SQLite"
	for _, name := range []string{"orion.db", "orion.db-wal"} {
		if info, err := os.Stat(filepath.Join(s.cfg.DataDir, name)); err == nil {
			out.Database.SizeBytes += info.Size()
		}
	}
	out.Backups.Note = "Stop writes, copy orion.db, restart. See the backup guide."
	out.Backups.GuideURL = "https://github.com/sunday-studio/orion/blob/main/docs/deployment/core-docker.md#backups"
	utils.SuccessResponse(c, http.StatusOK, "System settings retrieved", out)
}

// RegistrationTokenResponse is returned when the registration token is rotated.
type RegistrationTokenResponse struct {
	Token     string    `json:"token" example:"orn_reg_3Fk9d2a7c1e5b8046a9d3f2e"`
	RotatedAt time.Time `json:"rotated_at"`
}

func (s *Server) rotateRegistrationTokenImpl(c *gin.Context) {
	if !s.requireSession(c) {
		return
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		utils.InternalError(c, "Failed to generate a token", err)
		return
	}
	row, err := s.loadInstanceSettings()
	if err != nil {
		utils.InternalError(c, "Failed to load settings", err)
		return
	}
	now := time.Now().UTC()
	row.RegistrationToken, row.RegistrationTokenRotatedAt, row.UpdatedBy, row.UpdatedAt = "orn_reg_"+hex.EncodeToString(raw[:]), &now, frontendActorID(c), now
	if err := s.db.Save(&row).Error; err != nil {
		utils.InternalError(c, "Failed to save the token", err)
		return
	}
	s.logger.Info("Registration token rotated", "actor", frontendActorID(c))
	utils.SuccessResponse(c, http.StatusOK, "Registration token rotated. Servers already connected keep working; new servers need the new token.", RegistrationTokenResponse{Token: row.RegistrationToken, RotatedAt: now})
}

// maskToken keeps the start and end of a secret so people can tell tokens apart.
func maskToken(token string) string {
	if len(token) <= 12 {
		return ""
	}
	return token[:8] + "••••" + token[len(token)-4:]
}
