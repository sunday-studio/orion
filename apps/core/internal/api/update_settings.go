package api

import (
	"context"
	"net/http"
	"orion/core/internal/buildinfo"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UpdateSettings says which release Core runs, whether a newer one exists, and how Core looks for it.
type UpdateSettings struct {
	CurrentVersion string `json:"current_version" example:"0.9.2"`
	AutoCheck      bool   `json:"auto_check"`
	Channel        string `json:"channel" enums:"stable,prerelease"`
	// State is the one word the Console shows: up_to_date, available, security (an update that fixes
	// a vulnerability, which cannot be skipped), skipped, error (the last check failed), off (checks
	// are off) or unchecked (nothing has been looked up yet).
	State         string     `json:"state" enums:"up_to_date,available,security,skipped,error,off,unchecked"`
	LatestVersion string     `json:"latest_version,omitempty" example:"0.9.3"`
	ReleasedAt    *time.Time `json:"released_at,omitempty"`
	ReleaseURL    string     `json:"release_url,omitempty"`
	// Highlights are the first bullet points of the release notes.
	Highlights     []string   `json:"highlights,omitempty"`
	Security       bool       `json:"security"`
	SkippedVersion string     `json:"skipped_version,omitempty"`
	CheckedAt      *time.Time `json:"checked_at,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// UpdateSettingsRequest changes how Core checks. Send only the fields to change.
type UpdateSettingsRequest struct {
	AutoCheck *bool   `json:"auto_check"`
	Channel   *string `json:"channel"`
	// SkippedVersion hides one release; send an empty string to stop skipping.
	SkippedVersion *string `json:"skipped_version"`
}

func updateSettingsResponse(row db.InstanceSettings) UpdateSettings {
	out := UpdateSettings{
		CurrentVersion: buildinfo.Version, AutoCheck: row.UpdateCheckEnabled, Channel: row.UpdateChannel,
		LatestVersion: row.UpdateLatestVersion, ReleasedAt: row.UpdateLatestReleasedAt, ReleaseURL: row.UpdateLatestURL,
		Security: row.UpdateLatestSecurity, SkippedVersion: row.UpdateSkippedVersion, CheckedAt: row.UpdateCheckedAt, Error: row.UpdateCheckError,
	}
	if row.UpdateLatestNotes != "" {
		out.Highlights = strings.Split(row.UpdateLatestNotes, "\n")
	}
	newer := false
	if row.UpdateLatestVersion != "" {
		if cmp, ok := service.CompareVersions(buildinfo.Version, row.UpdateLatestVersion); ok && cmp < 0 {
			newer = true
		}
	}
	switch {
	case newer && row.UpdateLatestSecurity:
		out.State = "security"
	case newer && row.UpdateSkippedVersion == row.UpdateLatestVersion:
		out.State = "skipped"
	case newer:
		out.State = "available"
	case row.UpdateCheckError != "":
		out.State = "error"
	case !row.UpdateCheckEnabled:
		out.State = "off"
	case row.UpdateCheckedAt == nil:
		out.State = "unchecked"
	default:
		out.State = "up_to_date"
	}
	return out
}

func (s *Server) getUpdateSettingsImpl(c *gin.Context) {
	row, err := s.loadInstanceSettings()
	if err != nil {
		utils.InternalError(c, "Failed to load settings", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Update settings retrieved", updateSettingsResponse(row))
}

func (s *Server) saveUpdateSettingsImpl(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid update settings payload")
		return
	}
	row, err := s.loadInstanceSettings()
	if err != nil {
		utils.InternalError(c, "Failed to load settings", err)
		return
	}
	if req.AutoCheck != nil {
		row.UpdateCheckEnabled = *req.AutoCheck
	}
	if req.Channel != nil {
		if *req.Channel != "stable" && *req.Channel != "prerelease" {
			utils.BadRequest(c, "channel must be stable or prerelease")
			return
		}
		if *req.Channel != row.UpdateChannel {
			// What was found belongs to the old channel; the next check fills it in again.
			row.UpdateChannel = *req.Channel
			row.UpdateLatestVersion, row.UpdateLatestReleasedAt, row.UpdateLatestSecurity, row.UpdateLatestURL, row.UpdateLatestNotes = "", nil, false, "", ""
			row.UpdateCheckedAt, row.UpdateCheckError, row.UpdateSkippedVersion = nil, "", ""
		}
	}
	if req.SkippedVersion != nil {
		skipped := strings.TrimSpace(*req.SkippedVersion)
		if skipped != "" && skipped != row.UpdateLatestVersion {
			utils.BadRequest(c, "skipped_version must be the latest version Core found")
			return
		}
		row.UpdateSkippedVersion = skipped
	}
	row.UpdatedBy, row.UpdatedAt = frontendActorID(c), time.Now().UTC()
	if err := s.saveUpdateColumns(row, "UpdatedBy", "UpdatedAt"); err != nil {
		utils.InternalError(c, "Failed to save settings", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Update settings saved", updateSettingsResponse(row))
}

// updateColumns are the only instance_settings columns the update settings and checks may write, so
// a check that waits on GitHub never puts back an old public URL, timezone or registration token.
var updateColumns = []string{
	"UpdateCheckEnabled", "UpdateChannel", "UpdateLatestVersion", "UpdateLatestReleasedAt",
	"UpdateLatestSecurity", "UpdateLatestURL", "UpdateLatestNotes", "UpdateCheckedAt",
	"UpdateCheckError", "UpdateSkippedVersion",
}

func (s *Server) saveUpdateColumns(row db.InstanceSettings, extra ...string) error {
	cols := append(append([]string{}, updateColumns...), extra...)
	return s.db.Model(&db.InstanceSettings{}).Where("id = ?", row.ID).Select(cols).Updates(&row).Error
}

// checkForUpdate asks the release source once and stores what it found, or why it could not.
func (s *Server) checkForUpdate(ctx context.Context) (db.InstanceSettings, error) {
	row, err := s.loadInstanceSettings()
	if err != nil {
		return row, err
	}
	channel := row.UpdateChannel
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	release, lookupErr := s.releases.Latest(ctx, channel)
	now := time.Now().UTC()
	// The lookup can take seconds. Start again from what is stored now, so nothing changed meanwhile is lost.
	row, err = s.loadInstanceSettings()
	if err != nil {
		return row, err
	}
	if row.UpdateChannel != channel {
		return row, nil // the channel changed while we looked; what we found belongs to the old one
	}
	if lookupErr != nil {
		row.UpdateCheckError = lookupErr.Error()
	} else {
		released := release.ReleasedAt
		row.UpdateLatestVersion, row.UpdateLatestReleasedAt, row.UpdateLatestSecurity, row.UpdateLatestURL = release.Version, &released, release.Security, release.URL
		row.UpdateLatestNotes = strings.Join(release.Highlights, "\n")
		row.UpdateCheckedAt, row.UpdateCheckError = &now, ""
	}
	return row, s.saveUpdateColumns(row)
}

func (s *Server) checkUpdatesNowImpl(c *gin.Context) {
	row, err := s.checkForUpdate(c.Request.Context())
	if err != nil {
		utils.InternalError(c, "Failed to save the update check", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Update check finished", updateSettingsResponse(row))
}

// RunUpdateChecks looks for a newer release about once a day while checks are on. It stops with ctx.
func (s *Server) RunUpdateChecks(ctx context.Context) {
	timer := time.NewTimer(2 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if row, err := s.loadInstanceSettings(); err == nil && row.UpdateCheckEnabled && (row.UpdateCheckedAt == nil || time.Since(*row.UpdateCheckedAt) >= 24*time.Hour) {
			if _, err := s.checkForUpdate(ctx); err != nil {
				s.logger.Warn("Update check could not be saved", "error", err)
			}
		}
		timer.Reset(time.Hour)
	}
}

// getUpdateSettings Release check and update settings.
// @Summary      Get update settings
// @Description  The release Core runs, the newest release found, whether checks are on, the channel, and one state word for the Console. Core looks for a newer release once a day when checks are on.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           getUpdateSettings
// @Success      200  {object}  utils.APIResponse{data=UpdateSettings}
// @Router       /v1/settings/updates [get]
func (s *Server) getUpdateSettings(c *gin.Context) { s.getUpdateSettingsImpl(c) }

// updateUpdateSettings Change how Core checks for releases.
// @Summary      Update the update settings
// @Description  Turn the daily check on or off, pick the stable or pre-release channel, or skip the latest release. Changing the channel clears what was found until the next check. A release that fixes a vulnerability cannot be skipped.
// @Tags         settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateUpdateSettings
// @Param        request  body      UpdateSettingsRequest  true  "Fields to change"
// @Success      200  {object}  utils.APIResponse{data=UpdateSettings}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/settings/updates [put]
func (s *Server) updateUpdateSettings(c *gin.Context) { s.saveUpdateSettingsImpl(c) }

// checkUpdatesNow Look for a newer release now.
// @Summary      Check for updates now
// @Description  Asks GitHub for the newest release on the channel and stores the answer. A failed check is returned as state error with the reason, not as an HTTP error.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           checkUpdatesNow
// @Success      200  {object}  utils.APIResponse{data=UpdateSettings}
// @Router       /v1/settings/updates/check [post]
func (s *Server) checkUpdatesNow(c *gin.Context) { s.checkUpdatesNowImpl(c) }
