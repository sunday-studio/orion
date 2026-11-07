package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/metricstore"

	"gorm.io/gorm"
)

// ServiceMetricTargets is what a service's metric history is read from: the measured resources linked
// to it, and the apps linked to it (whose app-reported series are looked up by app).
type ServiceMetricTargets struct {
	Resources []metricstore.SeriesRef
	AppIDs    []string
}

// MetricTargets resolves a service's links at read time, so linking or unlinking never rewrites stored
// history: the same samples simply belong to the service while the link exists.
func (c *ServiceCatalog) MetricTargets(serviceID string) (ServiceMetricTargets, error) {
	var svc db.Service
	if err := c.db.Select("id").Where("id = ?", serviceID).First(&svc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ServiceMetricTargets{}, serviceNotFound("Service not found")
		}
		return ServiceMetricTargets{}, err
	}
	var out ServiceMetricTargets
	var resources []db.NodeAppResource
	if err := c.db.Select("node_id", "key").Where("service_id = ?", serviceID).Order("node_id, key").Find(&resources).Error; err != nil {
		return out, err
	}
	for _, r := range resources {
		out.Resources = append(out.Resources, metricstore.SeriesRef{Source: metricstore.SourceOS, NodeID: r.NodeID, Key: r.Key})
	}
	if err := c.db.Model(&db.App{}).Where("service_id = ?", serviceID).Order("name ASC").Pluck("id", &out.AppIDs).Error; err != nil {
		return out, err
	}
	return out, nil
}
