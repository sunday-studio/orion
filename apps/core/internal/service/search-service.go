package service

import (
	"orion/core/internal/db"
	"strings"

	"gorm.io/gorm"
)

const searchPerGroup = 5

// SearchHit is one command-palette result.
type SearchHit struct {
	Type   string // monitor | incident | server | app
	ID     string
	Name   string
	Status string // health for monitors/servers, incident status for incidents
	Number int    // incident number, 0 for other types
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Search finds monitors, incidents, servers and apps whose name contains q (case-insensitive).
func Search(database *gorm.DB, q string) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []SearchHit{}, nil
	}
	pattern := "%" + likeEscaper.Replace(strings.ToLower(q)) + "%"
	var hits []SearchHit

	var monitors []db.Monitor
	if err := database.Where("lifecycle = ? AND system = ? AND LOWER(name) LIKE ? ESCAPE '\\'", "active", false, pattern).Order("name ASC").Limit(searchPerGroup).Find(&monitors).Error; err != nil {
		return nil, err
	}
	for _, m := range monitors {
		hits = append(hits, SearchHit{Type: "monitor", ID: m.ID, Name: m.Name, Status: m.ComputedHealth})
	}
	var incidents []db.Incident
	if err := database.Where("LOWER(title) LIKE ? ESCAPE '\\'", pattern).Order("opened_at DESC").Limit(searchPerGroup).Find(&incidents).Error; err != nil {
		return nil, err
	}
	for _, i := range incidents {
		hits = append(hits, SearchHit{Type: "incident", ID: i.ID, Name: i.Title, Status: i.Status, Number: i.Number})
	}
	var agents []db.Agent
	if err := database.Where("system = ? AND LOWER(name) LIKE ? ESCAPE '\\'", false, pattern).Order("name ASC").Limit(searchPerGroup).Find(&agents).Error; err != nil {
		return nil, err
	}
	for _, a := range agents {
		hits = append(hits, SearchHit{Type: "server", ID: a.ID, Name: a.Name})
	}
	var apps []db.App
	if err := database.Where("LOWER(name) LIKE ? ESCAPE '\\'", pattern).Order("name ASC").Limit(searchPerGroup).Find(&apps).Error; err != nil {
		return nil, err
	}
	for _, a := range apps {
		hits = append(hits, SearchHit{Type: "app", ID: a.ID, Name: a.Name})
	}
	return hits, nil
}
