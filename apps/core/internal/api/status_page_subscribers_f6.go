package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type StatusPageSubscriberCounts struct {
	Confirmed    int `json:"confirmed"`
	Pending      int `json:"pending"`
	Disabled     int `json:"disabled"`
	Unsubscribed int `json:"unsubscribed"`
	Total        int `json:"total"`
}

func (s *Server) statusPageSubscriberCounts(pageID string) StatusPageSubscriberCounts {
	var rows []struct {
		State string
		N     int
	}
	s.db.Model(&db.StatusPageSubscriber{}).Select("state, COUNT(*) AS n").Where("status_page_id = ?", pageID).Group("state").Scan(&rows)
	var out StatusPageSubscriberCounts
	for _, r := range rows {
		switch r.State {
		case "confirmed":
			out.Confirmed = r.N
		case "pending":
			out.Pending = r.N
		case "disabled":
			out.Disabled = r.N
		case "unsubscribed":
			out.Unsubscribed = r.N
		}
		out.Total += r.N
	}
	return out
}

func subscriberScope(components []StatusPageSubscriberAdminComponentResponse) string {
	switch len(components) {
	case 0:
		return "All components"
	case 1:
		return components[0].Name + " only"
	default:
		return fmt.Sprintf("%d components", len(components))
	}
}

func firstTime(at *time.Time, fallback time.Time) time.Time {
	if at != nil {
		return *at
	}
	return fallback
}

func filterSubscriberResponses(rows []StatusPageSubscriberAdminResponse, q string) []StatusPageSubscriberAdminResponse {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]StatusPageSubscriberAdminResponse, 0, len(rows))
	for _, r := range rows {
		if strings.Contains(strings.ToLower(r.Email), q) || strings.Contains(strings.ToLower(r.MaskedDestination), q) {
			out = append(out, r)
		}
	}
	return out
}

// exportStatusPageSubscribers downloads the subscribers as CSV.
// @Summary      Export status page subscribers
// @Description  CSV with email, scope, status and since. Honors the state filter. Emails need the subscriber secret to be configured.
// @Tags         status-pages
// @Produce      text/csv
// @Security     BearerAuth
// @ID           exportStatusPageSubscribers
// @Param        id     path   string  true   "Status page ID"
// @Param        state  query  string  false  "Subscriber state filter"
// @Success      200    {string}  string  "CSV"
// @Failure      404    {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers/export.csv [get]
func (s *Server) exportStatusPageSubscribers(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	if !s.statusPageExists(c, pageID) {
		return
	}
	query := s.db.Where("status_page_id = ?", pageID).Order("created_at DESC")
	if state := strings.TrimSpace(c.Query("state")); state != "" {
		query = query.Where("state = ?", state)
	}
	var subscribers []db.StatusPageSubscriber
	if err := query.Find(&subscribers).Error; err != nil {
		utils.InternalError(c, "Failed to export subscribers", err)
		return
	}
	rows, err := s.statusPageSubscriberAdminResponses(pageID, subscribers)
	if err != nil {
		utils.InternalError(c, "Failed to export subscribers", err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="subscribers.csv"`)
	c.Header("Cache-Control", "no-store")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"email", "scope", "status", "since"})
	for _, r := range rows {
		_ = w.Write([]string{csvSafe(firstNonEmpty(r.Email, r.MaskedDestination)), csvSafe(r.Scope), r.State, r.Since.UTC().Format("2006-01-02")})
	}
	w.Flush()
	c.Status(http.StatusOK)
}
