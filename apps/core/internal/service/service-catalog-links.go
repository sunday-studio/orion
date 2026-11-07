package service

import (
	"errors"
	"orion/core/internal/db"
	"strings"

	"gorm.io/gorm"
)

// linkTarget is the row a ServiceLinkRef points at, as a scoped query plus what to call it in errors.
type linkTarget struct {
	label string
	scope func(tx *gorm.DB) *gorm.DB
	table string
}

func resolveLinkRef(ref ServiceLinkRef) (linkTarget, ServiceLinkRef, error) {
	ref.Kind = strings.TrimSpace(ref.Kind)
	ref.ID, ref.NodeID, ref.Key = strings.TrimSpace(ref.ID), strings.TrimSpace(ref.NodeID), strings.TrimSpace(ref.Key)
	switch ref.Kind {
	case ServiceLinkApp, ServiceLinkMonitor:
		if ref.ID == "" {
			return linkTarget{}, ref, serviceInvalid("id is required for an " + ref.Kind + " link")
		}
		if ref.Kind == ServiceLinkApp {
			return linkTarget{label: "App", table: "apps", scope: func(tx *gorm.DB) *gorm.DB { return tx.Where("id = ?", ref.ID) }}, ref, nil
		}
		return linkTarget{label: "Monitor", table: "monitors", scope: func(tx *gorm.DB) *gorm.DB {
			return tx.Where("id = ? AND lifecycle <> ?", ref.ID, monitorLifecycleDeleted)
		}}, ref, nil
	case ServiceLinkResource:
		if ref.NodeID == "" || ref.Key == "" {
			return linkTarget{}, ref, serviceInvalid("node_id and key are required for a resource link")
		}
		return linkTarget{label: "Resource", table: "node_app_resources", scope: func(tx *gorm.DB) *gorm.DB {
			return tx.Where("node_id = ? AND key = ?", ref.NodeID, ref.Key)
		}}, ref, nil
	}
	return linkTarget{}, ref, serviceInvalid("kind must be app, monitor or resource")
}

// currentLink reads the target's service_id; ok is false when the target does not exist.
func (t linkTarget) currentLink(tx *gorm.DB) (current string, ok bool, err error) {
	var rows []struct{ ServiceID *string }
	if err := t.scope(tx.Table(t.table).Select("service_id")).Scan(&rows).Error; err != nil {
		return "", false, err
	}
	if len(rows) == 0 {
		return "", false, nil
	}
	if rows[0].ServiceID != nil {
		current = *rows[0].ServiceID
	}
	return current, true, nil
}

// Link attaches an app, monitor or resource to the service. Linking something that already
// belongs to another service moves it (previous_service_id says from where); linking it to the
// same service again changes nothing.
func (c *ServiceCatalog) Link(serviceID string, ref ServiceLinkRef) (ServiceLinkResult, error) {
	target, ref, err := resolveLinkRef(ref)
	if err != nil {
		return ServiceLinkResult{}, err
	}
	result := ServiceLinkResult{ServiceID: serviceID, ServiceLinkRef: ref}
	err = c.db.Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&db.Service{}).Where("id = ?", serviceID).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return serviceNotFound("Service not found")
		}
		current, ok, err := target.currentLink(tx)
		if err != nil {
			return err
		}
		if !ok {
			return serviceNotFound(target.label + " not found")
		}
		if ref.Kind == ServiceLinkMonitor {
			var system bool
			if err := tx.Model(&db.Monitor{}).Select("system").Where("id = ?", ref.ID).Scan(&system).Error; err != nil {
				return err
			}
			if system {
				return serviceInvalid("System monitors cannot be linked to a service")
			}
		}
		if current == serviceID {
			return nil
		}
		result.PreviousServiceID = current
		// UpdateColumn leaves updated_at alone: a link is not an edit of the monitor or app itself.
		return target.scope(tx.Table(target.table)).UpdateColumn("service_id", serviceID).Error
	})
	return result, err
}

// Unlink detaches the thing from the service. It only clears service_id: logs, reports, incidents
// and samples stay exactly where they are. A thing that is not linked to this service is a 404.
func (c *ServiceCatalog) Unlink(serviceID string, ref ServiceLinkRef) error {
	target, _, err := resolveLinkRef(ref)
	if err != nil {
		return err
	}
	return c.db.Transaction(func(tx *gorm.DB) error {
		current, ok, err := target.currentLink(tx)
		if err != nil {
			return err
		}
		if !ok {
			return serviceNotFound(target.label + " not found")
		}
		if current != serviceID {
			return serviceNotFound(target.label + " is not linked to this service")
		}
		return target.scope(tx.Table(target.table)).UpdateColumn("service_id", nil).Error
	})
}

// IsServiceError reports whether err is a ServiceError, returning it.
func IsServiceError(err error) (*ServiceError, bool) {
	var se *ServiceError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}
