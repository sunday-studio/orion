package api

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/publicpage"
	"orion/core/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const publicHistoryMonthsPerPage = 3

func publicComponentView(component StatusPagePublicComponentResponse, withBars bool) publicpage.Component {
	view := publicpage.Component{ID: component.ID, Name: component.Name, Reason: component.StatusReason, Status: component.Status, Display: component.StatusDisplay, Class: publicStatusClass(component.Status), Uptime: statusPagePublicNoDataDisplay}
	if component.Uptime != nil && component.Uptime.UptimeDisplay != "" {
		view.Uptime = component.Uptime.UptimeDisplay
	}
	if withBars {
		for _, bucket := range component.UptimeHistory {
			view.Bars = append(view.Bars, publicpage.Bar{Class: publicBarClass(bucket), Tip: publicBarTip(bucket)})
		}
	}
	return view
}

// publicUptime30 averages the last 30 days of every visible component's history.
func publicUptime30(sections []StatusPagePublicSectionResponse) string {
	var sum float64
	var n int
	for _, section := range sections {
		for _, component := range section.Components {
			history := component.UptimeHistory
			if len(history) > 30 {
				history = history[len(history)-30:]
			}
			for _, bucket := range history {
				if bucket.UptimeRatio != nil {
					sum += *bucket.UptimeRatio
					n++
				}
			}
		}
	}
	if n == 0 {
		return statusPagePublicNoDataDisplay
	}
	ratio := sum / float64(n)
	return publicUptimeDisplay(&ratio)
}

func (s *Server) publicIncidentView(base string, incident StatusPagePublicIncidentResponse, withLatest bool) publicpage.Incident {
	display, class := publicIncidentPill(incident.PublicStatus)
	started := publicIncidentStart(incident)
	view := publicpage.Incident{ID: incident.ID, Title: incident.Title, URL: base + "/incidents/" + url.PathEscape(incident.ID), Status: incident.PublicStatus, Display: display, Class: class, Started: started, Duration: publicDuration(started, incident.ResolvedAt), Kind: "incident"}
	if withLatest {
		if updates, err := s.publicStatusPageIncidentUpdates(incident.ID); err == nil && len(updates) > 0 {
			last := updates[len(updates)-1]
			if last.PublishedAt != nil {
				d, cl := publicIncidentPill(last.Status)
				view.Latest = &publicpage.Update{At: *last.PublishedAt, Status: last.Status, Display: d, Class: cl, Message: last.Message}
			}
		}
	}
	return view
}

func (s *Server) writePublicStatusPageHTML(c *gin.Context, preview StatusPagePreviewResponse) {
	shell, theme := publicPageShell(c, preview, "", true)
	word, tone := publicTickerFor(preview.OverallStatus)
	data := publicpage.StatusData{Shell: shell, ShowStats: theme.showUptime, Updated: preview.LastUpdated}
	data.Hero = publicpage.Hero{Ticker: ticker(word, tone), Eyebrow: "Current status", Title: publicStatusPageHTMLSummary(preview.OverallStatus), Text: preview.Page.Description}
	for _, section := range preview.Sections {
		view := publicpage.Section{Name: section.Name}
		for _, component := range section.Components {
			item := publicComponentView(component, theme.showUptime)
			if !theme.showUptime {
				item.Uptime = ""
			}
			if component.Status != "operational" {
				data.Affected++
			}
			data.Total++
			view.Components = append(view.Components, item)
		}
		if len(view.Components) > 0 {
			data.Sections = append(data.Sections, view)
		}
	}
	data.Uptime30 = publicUptime30(preview.Sections)
	base := publicPageBase(preview.Page.Slug)
	for _, incident := range preview.Incidents {
		if incident.PublicStatus != "resolved" {
			data.Active = append(data.Active, s.publicIncidentView(base, incident, true))
		}
	}
	data.OpenCount = len(data.Active)
	for _, window := range preview.Maintenance {
		display, started := "Scheduled", window.StartsAt
		if window.Status == "in_progress" {
			display = "In progress"
		}
		end := window.EndsAt
		data.Maintenance = append(data.Maintenance, publicpage.Incident{Title: window.Title, Display: display, Class: "mnt", Started: started, Duration: publicDuration(started, &end), Kind: "maintenance"})
	}
	s.writePublicPage(c, http.StatusOK, "status.html", data, false)
}

// publicHistoryEnd parses ?before=YYYY-MM into the first instant after the page's last month.
func publicHistoryEnd(raw string, now time.Time) time.Time {
	if t, err := time.Parse("2006-01", strings.TrimSpace(raw)); err == nil && !t.After(now) {
		return t.AddDate(0, 1, 0)
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
}

// writePublicStatusPageHistoryHTML is the archive: published incidents and past maintenance, three months a page.
func (s *Server) writePublicStatusPageHistoryHTML(c *gin.Context, page db.StatusPage) {
	preview, err := s.publicPreviewFor(page)
	if err != nil {
		utils.InternalError(c, "Failed to load status page", err)
		return
	}
	shell, theme := publicPageShell(c, preview, preview.Page.Title+" incident history", false)
	if !theme.showHistory {
		utils.NotFound(c, "Status page history not found")
		return
	}
	now := time.Now().UTC()
	end := publicHistoryEnd(c.Query("before"), now)
	start := end.AddDate(0, -publicHistoryMonthsPerPage, 0)
	base := publicPageBase(page.Slug)
	var incidents []db.StatusPageIncident
	if err := s.db.Where("status_page_id = ? AND visibility = ? AND published_at IS NOT NULL AND published_at >= ? AND published_at < ?", page.ID, statusPageIncidentVisibilityPublished, start, end).Order("published_at DESC").Find(&incidents).Error; err != nil {
		utils.InternalError(c, "Failed to load status page history", err)
		return
	}
	var windows []db.StatusPageMaintenance
	if err := s.db.Where("status_page_id = ? AND ends_at <= ? AND starts_at >= ? AND starts_at < ?", page.ID, now, start, end).Order("starts_at DESC").Find(&windows).Error; err != nil {
		utils.InternalError(c, "Failed to load status page history", err)
		return
	}
	type entry struct {
		at   time.Time
		item publicpage.Incident
	}
	var entries []entry
	for _, incident := range incidents {
		view := s.publicIncidentView(base, StatusPagePublicIncidentResponse{ID: incident.ID, Title: incident.Title, PublicStatus: incident.PublicStatus, PublishedAt: incident.PublishedAt, ResolvedAt: incident.ResolvedAt, ScheduledStartAt: incident.ScheduledStartAt}, false)
		entries = append(entries, entry{view.Started, view})
	}
	for _, w := range windows {
		ends := w.EndsAt
		entries = append(entries, entry{w.StartsAt, publicpage.Incident{Title: w.Title, Display: "Maintenance", Class: "mnt", Started: w.StartsAt, Duration: publicDuration(w.StartsAt, &ends), Kind: "maintenance"}})
	}
	for i := 1; i < len(entries); i++ { // insertion sort, newest first; entry counts are small
		for j := i; j > 0 && entries[j].at.After(entries[j-1].at); j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	data := publicpage.HistoryData{Shell: shell, Empty: len(entries) == 0}
	data.Hero = publicpage.Hero{Ticker: ticker("History", publicpage.ToneGood), Eyebrow: "Archive · 90 days", Title: "Incident history", Text: preview.Page.Description}
	for _, e := range entries {
		label := e.at.Format("January 2006")
		if n := len(data.Months); n == 0 || data.Months[n-1].Label != label {
			data.Months = append(data.Months, publicpage.Month{Label: label})
		}
		month := &data.Months[len(data.Months)-1]
		month.Items = append(month.Items, e.item)
	}
	for i := range data.Months {
		n := len(data.Months[i].Items)
		data.Months[i].Count = pluralCount(n, "event")
	}
	var older int64
	s.db.Model(&db.StatusPageIncident{}).Where("status_page_id = ? AND visibility = ? AND published_at IS NOT NULL AND published_at < ?", page.ID, statusPageIncidentVisibilityPublished, start).Count(&older)
	if older > 0 {
		data.PrevURL = base + "/history?before=" + start.AddDate(0, -1, 0).Format("2006-01")
	}
	s.writePublicPage(c, http.StatusOK, "history.html", data, false)
}

func pluralCount(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

func (s *Server) writePublicStatusPageIncidentHTML(c *gin.Context, preview StatusPagePreviewResponse, incident StatusPagePublicIncidentResponse) {
	shell, _ := publicPageShell(c, preview, incident.Title+" - "+preview.Page.Title, false)
	updates, err := s.publicStatusPageIncidentUpdates(incident.ID)
	if err != nil {
		utils.InternalError(c, "Failed to load incident updates", err)
		return
	}
	display, _ := publicIncidentPill(incident.PublicStatus)
	started := publicIncidentStart(incident)
	data := publicpage.IncidentData{Shell: shell, Subscribe: shell.SubscribeURL}
	data.Hero = publicpage.Hero{Ticker: ticker(display, publicIncidentTone(incident.PublicStatus)), Eyebrow: "Incident · " + started.UTC().Format("Jan 2, 2006"), Title: incident.Title, Text: incident.ImpactSummary}
	for i := len(updates) - 1; i >= 0; i-- {
		u := updates[i]
		if u.PublishedAt == nil {
			continue
		}
		d, cl := publicIncidentPill(u.Status)
		data.Updates = append(data.Updates, publicpage.Update{At: *u.PublishedAt, Status: u.Status, Display: d, Class: cl, Message: u.Message})
	}
	s.writePublicPage(c, http.StatusOK, "incident.html", data, false)
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}
type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}
type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate,omitempty"`
	Description string `xml:"description"`
}

// getStatusPageRSSFeed is the RSS twin of the Atom feed: the latest published incidents.
// @Summary      Get public status page RSS feed
// @Tags         public-status
// @Produce      application/rss+xml
// @ID           getPublicStatusPageRSSFeed
// @Param        slug  path      string  true  "Status page slug"
// @Success      200   {string}  string  "RSS 2.0 document"
// @Failure      404   {object}  utils.APIResponse
// @Router       /status/{slug}/feed.rss [get]
func (s *Server) getStatusPageRSSFeed(c *gin.Context) {
	page, ok := s.loadStatusPageFeedPage(c, strings.TrimSpace(c.Param("slug")))
	if !ok {
		return
	}
	var incidents []db.StatusPageIncident
	if err := s.db.Where("status_page_id = ? AND visibility = ? AND published_at IS NOT NULL", page.ID, statusPageIncidentVisibilityPublished).Order("published_at DESC").Limit(50).Find(&incidents).Error; err != nil {
		utils.InternalError(c, "Failed to load status page feed", err)
		return
	}
	root := statusPagePublicURL(c, page)
	feed := rssFeed{Version: "2.0", Channel: rssChannel{Title: page.Title, Link: root, Description: firstNonEmpty(page.Description, page.Title+" incidents")}}
	for _, incident := range incidents {
		link := strings.TrimRight(root, "/") + "/incidents/" + url.PathEscape(incident.ID)
		item := rssItem{Title: incident.Title, Link: link, GUID: link, Description: incident.ImpactSummary}
		if incident.PublishedAt != nil {
			item.PubDate = incident.PublishedAt.UTC().Format(time.RFC1123Z)
		}
		feed.Channel.Items = append(feed.Channel.Items, item)
	}
	payload, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		utils.InternalError(c, "Failed to render status page feed", err)
		return
	}
	writePublicStatusPagePayload(c, http.StatusOK, "application/rss+xml; charset=utf-8", append([]byte(xml.Header), payload...))
}

// publicBarTip is the hover text of one day: "Oct 3 · Operational · 100%".
func publicBarTip(bucket StatusPagePublicUptimeBucketResponse) string {
	label := publicStatusDisplay(bucket.Status)
	if bucket.Status == "no_data" || bucket.Status == "" {
		label = statusPagePublicNoDataDisplay
	}
	day := bucket.Date
	if t, err := time.Parse("2006-01-02", bucket.Date); err == nil {
		day = t.Format("Jan 2")
	}
	tip := day + " · " + label
	if bucket.UptimeDisplay != "" && bucket.Status != "no_data" {
		tip += " · " + bucket.UptimeDisplay
	}
	return tip
}
