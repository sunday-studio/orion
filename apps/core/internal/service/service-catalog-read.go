package service

import (
	"orion/core/internal/db"
	"sort"
)

// Monitors in these lifecycles are not counted or listed on a service.
const monitorLifecycleDeleted = "deleted"

// List returns every service with its link counts, health and open incident count.
func (c *ServiceCatalog) List() ([]ServiceSummary, error) {
	var rows []db.Service
	if err := c.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ServiceSummary, 0, len(rows))
	index := make(map[string]int, len(rows))
	for _, row := range rows {
		index[row.ID] = len(out)
		out = append(out, summaryOf(row))
	}
	if len(out) == 0 {
		return out, nil
	}

	var apps []struct {
		ServiceID string
		N         int
	}
	if err := c.db.Model(&db.App{}).Select("service_id, COUNT(*) AS n").Where("service_id IS NOT NULL").Group("service_id").Scan(&apps).Error; err != nil {
		return nil, err
	}
	for _, a := range apps {
		if i, ok := index[a.ServiceID]; ok {
			out[i].AppCount = a.N
		}
	}
	var resources []struct {
		ServiceID string
		N         int
	}
	if err := c.db.Model(&db.NodeAppResource{}).Select("service_id, COUNT(*) AS n").Where("service_id IS NOT NULL").Group("service_id").Scan(&resources).Error; err != nil {
		return nil, err
	}
	for _, r := range resources {
		if i, ok := index[r.ServiceID]; ok {
			out[i].ResourceCount = r.N
		}
	}
	var monitors []db.Monitor
	if err := c.db.Select("id", "service_id", "health", "computed_health", "active_incident_id", "lifecycle").
		Where("service_id IS NOT NULL AND lifecycle <> ?", monitorLifecycleDeleted).Find(&monitors).Error; err != nil {
		return nil, err
	}
	for _, m := range monitors {
		i, ok := index[*m.ServiceID]
		if !ok {
			continue
		}
		out[i].MonitorCount++
		out[i].Health = worstKnownHealth(out[i].Health, monitorHealth(m))
		if m.ActiveIncidentID != "" {
			out[i].OpenIncidentCount++
		}
	}
	sortSummaries(out)
	return out, nil
}

// monitorHealth is the cached computed health, falling back to the last reported health.
func monitorHealth(m db.Monitor) string {
	h := m.ComputedHealth
	if h == "" || h == "unknown" {
		h = m.Health
	}
	if healthRank[h] == 0 {
		return "unknown"
	}
	return h
}

func (c *ServiceCatalog) summaryWithCounts(row db.Service) (ServiceSummary, error) {
	detail, err := c.detailOf(row)
	return detail.ServiceSummary, err
}

// Get returns one service with everything linked to it.
func (c *ServiceCatalog) Get(id string) (ServiceDetail, error) {
	var row db.Service
	if err := c.db.Where("id = ?", id).First(&row).Error; err != nil {
		return ServiceDetail{}, notFoundOr(err, "Service not found")
	}
	return c.detailOf(row)
}

func (c *ServiceCatalog) detailOf(row db.Service) (ServiceDetail, error) {
	d := ServiceDetail{ServiceSummary: summaryOf(row), Apps: []ServiceLinkedApp{}, Monitors: []ServiceLinkedMonitor{}, Resources: []ServiceLinkedResource{}}

	var apps []db.App
	if err := c.db.Select("id", "name", "slug", "environment", "last_ingest_at").Where("service_id = ?", row.ID).Order("name ASC").Find(&apps).Error; err != nil {
		return d, err
	}
	for _, a := range apps {
		d.Apps = append(d.Apps, ServiceLinkedApp{ID: a.ID, Name: a.Name, Slug: a.Slug, Environment: a.Environment, LastIngestAt: a.LastIngestAt})
	}

	var monitors []db.Monitor
	if err := c.db.Select("id", "name", "type", "node_id", "health", "computed_health", "lifecycle", "active_incident_id").
		Where("service_id = ? AND lifecycle <> ?", row.ID, monitorLifecycleDeleted).Order("name ASC").Find(&monitors).Error; err != nil {
		return d, err
	}
	for _, m := range monitors {
		health := monitorHealth(m)
		d.Monitors = append(d.Monitors, ServiceLinkedMonitor{ID: m.ID, Name: m.Name, Type: m.Type, NodeID: m.NodeID, Health: health, Lifecycle: m.Lifecycle, ActiveIncidentID: m.ActiveIncidentID})
		d.Health = worstKnownHealth(d.Health, health)
		if m.ActiveIncidentID != "" {
			d.OpenIncidentCount++
		}
	}

	var resources []db.NodeAppResource
	if err := c.db.Select("node_id", "key", "kind", "name", "updated_at").Where("service_id = ?", row.ID).Find(&resources).Error; err != nil {
		return d, err
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].NodeID != resources[j].NodeID {
			return resources[i].NodeID < resources[j].NodeID
		}
		return resources[i].Key < resources[j].Key
	})
	for _, r := range resources {
		d.Resources = append(d.Resources, ServiceLinkedResource{NodeID: r.NodeID, Key: r.Key, Kind: r.Kind, Name: r.Name, UpdatedAt: r.UpdatedAt})
	}
	d.AppCount, d.MonitorCount, d.ResourceCount = len(d.Apps), len(d.Monitors), len(d.Resources)
	return d, nil
}
