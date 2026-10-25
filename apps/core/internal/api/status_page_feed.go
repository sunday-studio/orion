package api

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Author  atomAuthor  `xml:"author"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Title     string     `xml:"title"`
	ID        string     `xml:"id"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published,omitempty"`
	Links     []atomLink `xml:"link"`
	Summary   atomText   `xml:"summary"`
	Content   atomText   `xml:"content"`
}

type atomText struct {
	Type string `xml:"type,attr,omitempty"`
	Text string `xml:",chardata"`
}

func (s *Server) getStatusPageAtomFeed(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	page, ok := s.loadStatusPageFeedPage(c, slug)
	if !ok {
		return
	}
	s.writeStatusPageAtomFeed(c, page)
}

func (s *Server) getCustomDomainStatusPageAtomFeed(c *gin.Context) {
	if !s.requestHostHasCustomStatusPage(c) {
		s.serveConsole(c)
		return
	}
	page, ok := s.loadStatusPageFeedPage(c, "")
	if !ok {
		return
	}
	s.writeStatusPageAtomFeed(c, page)
}

func (s *Server) loadStatusPageFeedPage(c *gin.Context, slug string) (db.StatusPage, bool) {
	page, err := s.loadPublicStatusPageForRequest(c, slug)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Status page not found")
			return db.StatusPage{}, false
		}
		s.logger.Error("Failed to load status page feed", "slug", slug, "error", err)
		utils.InternalError(c, "Failed to load status page feed", err)
		return db.StatusPage{}, false
	}
	return page, true
}

func (s *Server) writeStatusPageAtomFeed(c *gin.Context, page db.StatusPage) {
	var incidents []db.StatusPageIncident
	if err := s.db.
		Where("status_page_id = ? AND visibility = ? AND published_at IS NOT NULL", page.ID, "published").
		Order("published_at DESC").
		Limit(50).
		Find(&incidents).Error; err != nil {
		s.logger.Error("Failed to load status page feed incidents", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to load status page feed", err)
		return
	}

	updatesByIncident, err := s.publishedStatusPageIncidentUpdates(incidents)
	if err != nil {
		s.logger.Error("Failed to load status page feed incident updates", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to load status page feed", err)
		return
	}

	alternateURL := statusPagePublicURL(c, page)
	selfURL := strings.TrimRight(alternateURL, "/") + "/feed.atom"
	feed := atomFeed{
		XMLNS:   "http://www.w3.org/2005/Atom",
		Title:   page.Title,
		ID:      alternateURL,
		Updated: atomTime(statusPageFeedUpdatedAt(page, incidents, updatesByIncident)),
		Author:  atomAuthor{Name: statusPageFeedAuthorName(page)},
		Links: []atomLink{
			{Rel: "self", Type: "application/atom+xml", Href: selfURL},
			{Rel: "alternate", Type: "text/html", Href: alternateURL},
		},
		Entries: make([]atomEntry, 0, len(incidents)),
	}

	for _, incident := range incidents {
		entryURL := strings.TrimRight(alternateURL, "/") + "/incidents/" + url.PathEscape(incident.ID)
		updates := updatesByIncident[incident.ID]
		entryUpdated := statusPageIncidentFeedUpdatedAt(incident, updates)
		entry := atomEntry{
			Title:     incident.Title,
			ID:        entryURL,
			Updated:   atomTime(entryUpdated),
			Published: atomOptionalTime(incident.PublishedAt),
			Links: []atomLink{
				{Rel: "alternate", Type: "text/html", Href: entryURL},
			},
			Summary: atomText{Type: "text", Text: incident.ImpactSummary},
			Content: atomText{Type: "text", Text: publicIncidentFeedContent(incident, updates)},
		}
		feed.Entries = append(feed.Entries, entry)
	}

	payload, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		s.logger.Error("Failed to render status page feed", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to render status page feed", err)
		return
	}

	writePublicStatusPagePayload(c, http.StatusOK, "application/atom+xml; charset=utf-8", append([]byte(xml.Header), payload...))
}

func (s *Server) publishedStatusPageIncidentUpdates(incidents []db.StatusPageIncident) (map[string][]db.StatusPageIncidentUpdate, error) {
	updatesByIncident := make(map[string][]db.StatusPageIncidentUpdate, len(incidents))
	if len(incidents) == 0 {
		return updatesByIncident, nil
	}

	incidentIDs := make([]string, 0, len(incidents))
	for _, incident := range incidents {
		incidentIDs = append(incidentIDs, incident.ID)
	}

	var updates []db.StatusPageIncidentUpdate
	err := s.db.
		Where("incident_id IN ? AND published_at IS NOT NULL", incidentIDs).
		Order("published_at ASC").
		Find(&updates).Error
	if err != nil {
		return nil, err
	}

	for _, update := range updates {
		updatesByIncident[update.IncidentID] = append(updatesByIncident[update.IncidentID], update)
	}
	return updatesByIncident, nil
}

func statusPagePublicURL(c *gin.Context, page db.StatusPage) string {
	if strings.TrimSpace(page.CanonicalURL) != "" {
		return strings.TrimRight(strings.TrimSpace(page.CanonicalURL), "/")
	}

	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}

	host := c.Request.Host
	if requestHost, ok := publicStatusPageRequestHost(c); ok && requestHost == page.CustomDomain {
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	return fmt.Sprintf("%s://%s/status/%s", scheme, host, url.PathEscape(page.Slug))
}

func statusPageFeedUpdatedAt(page db.StatusPage, incidents []db.StatusPageIncident, updatesByIncident map[string][]db.StatusPageIncidentUpdate) time.Time {
	updatedAt := page.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = page.CreatedAt
	}
	if page.PublishedAt != nil && page.PublishedAt.After(updatedAt) {
		updatedAt = *page.PublishedAt
	}

	for _, incident := range incidents {
		incidentUpdatedAt := statusPageIncidentFeedUpdatedAt(incident, updatesByIncident[incident.ID])
		if incidentUpdatedAt.After(updatedAt) {
			updatedAt = incidentUpdatedAt
		}
	}

	if updatedAt.IsZero() {
		return time.Now().UTC()
	}
	return updatedAt
}

func statusPageIncidentFeedUpdatedAt(incident db.StatusPageIncident, updates []db.StatusPageIncidentUpdate) time.Time {
	updatedAt := incident.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = incident.CreatedAt
	}
	if incident.PublishedAt != nil && incident.PublishedAt.After(updatedAt) {
		updatedAt = *incident.PublishedAt
	}
	for _, update := range updates {
		if update.PublishedAt != nil && update.PublishedAt.After(updatedAt) {
			updatedAt = *update.PublishedAt
		}
	}
	if updatedAt.IsZero() {
		return time.Now().UTC()
	}
	return updatedAt
}

func publicIncidentFeedContent(incident db.StatusPageIncident, updates []db.StatusPageIncidentUpdate) string {
	lines := []string{
		"Status: " + incident.PublicStatus,
		"Severity: " + incident.Severity,
	}
	if strings.TrimSpace(incident.ImpactSummary) != "" {
		lines = append(lines, "", incident.ImpactSummary)
	}

	if len(updates) > 0 {
		latest := updates[len(updates)-1]
		lines = append(lines, "", "Latest update: "+latest.Message)
	}

	return strings.Join(lines, "\n")
}

func statusPageFeedAuthorName(page db.StatusPage) string {
	if title := strings.TrimSpace(page.Title); title != "" {
		return title
	}
	return "Orion Status"
}

func atomTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func atomOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return atomTime(*value)
}
