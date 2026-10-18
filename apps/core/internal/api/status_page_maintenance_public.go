package api

import (
	"orion/core/internal/db"
	"time"
)

// StatusPagePublicMaintenanceResponse is a maintenance window as visitors see it.
type StatusPagePublicMaintenanceResponse struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	StartsAt     time.Time `json:"starts_at"`
	EndsAt       time.Time `json:"ends_at"`
	Status       string    `json:"status" enums:"scheduled,in_progress"`
	ComponentIDs []string  `json:"component_ids"`
}

const statusPageMaintenanceLookahead = 14 * 24 * time.Hour

// publicMaintenance returns in-progress and upcoming windows plus the ids of components under maintenance now.
// Only visible components are listed; hidden ones never appear.
func (s *Server) publicMaintenance(detail StatusPageDetailResponse, now time.Time) ([]StatusPagePublicMaintenanceResponse, map[string]bool) {
	visible := map[string]bool{}
	for _, c := range detail.Components {
		if c.Visible {
			visible[c.ID] = true
		}
	}
	var rows []db.StatusPageMaintenance
	s.db.Where("status_page_id = ? AND ends_at > ? AND starts_at < ?", detail.Page.ID, now, now.Add(statusPageMaintenanceLookahead)).Order("starts_at ASC").Find(&rows)
	active := map[string]bool{}
	out := make([]StatusPagePublicMaintenanceResponse, 0, len(rows))
	for _, w := range rows {
		ids := []string{}
		for _, id := range decodeComponentIDs(w.ComponentIDs) {
			if visible[id] {
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			continue
		}
		status := maintenanceStatus(w, now)
		if status == "in_progress" {
			for _, id := range ids {
				active[id] = true
			}
		}
		out = append(out, StatusPagePublicMaintenanceResponse{ID: w.ID, Title: w.Title, Description: w.Description, StartsAt: publicMinute(w.StartsAt), EndsAt: publicMinute(w.EndsAt), Status: status, ComponentIDs: ids})
	}
	return out, active
}

func statusPageHTMLMaintenanceList(windows []StatusPagePublicMaintenanceResponse, sections []statusPageHTMLSection) []statusPageHTMLMaintenance {
	names := map[string]string{}
	for _, section := range sections {
		for _, component := range section.Components {
			names[component.ID] = component.Name
		}
	}
	out := make([]statusPageHTMLMaintenance, 0, len(windows))
	for _, w := range windows {
		item := statusPageHTMLMaintenance{Title: w.Title, Description: w.Description, InProgress: w.Status == "in_progress", Window: publicStatusPageHTMLTime(w.StartsAt) + " to " + publicStatusPageHTMLTime(w.EndsAt)}
		for _, id := range w.ComponentIDs {
			if n := names[id]; n != "" {
				item.Components = append(item.Components, n)
			}
		}
		out = append(out, item)
	}
	return out
}
