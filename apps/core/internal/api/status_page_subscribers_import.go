package api

import (
	"encoding/csv"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxSubscriberImportRows = 5000

type StatusPageSubscriberImportRequest struct {
	// CSV is the file text; the first column is the email. A header row named "email" is skipped.
	CSV string `json:"csv" binding:"required"`
}

type StatusPageSubscriberImportResponse struct {
	Imported int `json:"imported"`
	// Skipped counts addresses that were already subscribed.
	Skipped int `json:"skipped"`
	// Invalid lists rows that are not valid emails (at most 20 shown).
	Invalid       []string `json:"invalid"`
	InvalidCount  int      `json:"invalid_count"`
	ConfirmMailed int      `json:"confirm_mailed"`
}

// importStatusPageSubscribers adds subscribers from CSV text.
// @Summary      Import status page subscribers
// @Description  Imported addresses start pending and get the usual confirmation email, so nobody is subscribed without consent. The page must be published and the subscriber secret configured.
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           importStatusPageSubscribers
// @Param        id       path      string                             true  "Status page ID"
// @Param        request  body      StatusPageSubscriberImportRequest  true  "CSV text"
// @Success      200      {object}  utils.APIResponse{data=StatusPageSubscriberImportResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers/import [post]
func (s *Server) importStatusPageSubscribers(c *gin.Context) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", c.Param("id")).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "Status page not found")
		} else {
			utils.InternalError(c, "Failed to load status page", err)
		}
		return
	}
	if page.Visibility != statusPageVisibilityPublic && page.Visibility != statusPageVisibilityUnlisted {
		utils.BadRequest(c, "Publish the page before importing subscribers, so they can confirm")
		return
	}
	var req StatusPageSubscriberImportRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20+4096)
	if err := c.ShouldBindJSON(&req); err != nil || len(req.CSV) > 1<<20 {
		utils.BadRequest(c, "csv is required and must be 1 MB or smaller")
		return
	}
	if s.cfg == nil || strings.TrimSpace(s.cfg.PublicStatusSubscriberSecret) == "" {
		utils.BadRequest(c, "Set ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET before importing subscribers")
		return
	}
	records, err := csv.NewReader(strings.NewReader(req.CSV)).ReadAll()
	if err != nil {
		utils.BadRequest(c, "That is not a readable CSV file")
		return
	}
	if len(records) > maxSubscriberImportRows+1 {
		utils.BadRequest(c, "At most 5000 rows can be imported at once")
		return
	}
	out := StatusPageSubscriberImportResponse{Invalid: []string{}}
	for i, row := range records {
		if len(row) == 0 {
			continue
		}
		value := strings.TrimSpace(row[0])
		if i == 0 && strings.EqualFold(value, "email") {
			continue
		}
		if value == "" {
			continue
		}
		s.importOneSubscriber(page, value, &out)
	}
	utils.SuccessResponse(c, http.StatusOK, "Subscribers imported", out)
}

func (s *Server) importOneSubscriber(page db.StatusPage, value string, out *StatusPageSubscriberImportResponse) {
	invalid := func() {
		out.InvalidCount++
		if len(out.Invalid) < 20 {
			out.Invalid = append(out.Invalid, value)
		}
	}
	typ := statusPageSubscriberDestinationEmail
	destinationType, normalized, masked, err := normalizeStatusPageSubscriberDestination(&typ, &value)
	if err != nil {
		invalid()
		return
	}
	hash := hashStatusPageSubscriberValue(destinationType + ":" + normalized)
	var existing int64
	s.db.Model(&db.StatusPageSubscriber{}).Where("status_page_id = ? AND destination_type = ? AND destination_hash = ?", page.ID, destinationType, hash).Count(&existing)
	if existing > 0 {
		out.Skipped++
		return
	}
	ciphertext, err := s.encryptStatusPageSubscriberDestination(normalized)
	if err != nil {
		invalid()
		return
	}
	confirmation, manage, unsubscribe, err := generateStatusPageSubscriberTokens()
	if err != nil {
		invalid()
		return
	}
	expires := time.Now().UTC().Add(24 * time.Hour)
	sub := db.StatusPageSubscriber{ID: utils.GenerateID("status_page_subscriber"), StatusPageID: page.ID, DestinationType: destinationType, DestinationHash: hash, DestinationValueCiphertext: ciphertext, MaskedDestination: masked,
		State: statusPageSubscriberStatePending, ConfirmationTokenHash: hashStatusPageSubscriberToken(confirmation), ConfirmationTokenExpiresAt: &expires, ManageTokenHash: hashStatusPageSubscriberToken(manage), ManageTokenVersion: 1,
		UnsubscribeTokenHash: hashStatusPageSubscriberToken(unsubscribe), UnsubscribeTokenVersion: 1, Source: "import"}
	if err := s.db.Create(&sub).Error; err != nil {
		out.Skipped++
		return
	}
	out.Imported++
	if s.sendStatusPageSubscriberConfirmation(page, normalized, confirmation) == nil {
		out.ConfirmMailed++
	}
}
