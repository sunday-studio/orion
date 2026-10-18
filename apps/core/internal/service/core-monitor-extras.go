package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

// deletedMonitorHistoryDays is how long check history survives a delete made with keep_history.
const deletedMonitorHistoryDays = 90

func validateRunbookURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(raw) > 500 || u.User != nil {
		return "", fmt.Errorf("%w: runbook link must be an http or https URL", ErrCoreManagedMonitorValidation)
	}
	return raw, nil
}

// purgeMonitorsHistory removes the monitors and everything recorded for them. Incidents stay.
func purgeMonitorsHistory(tx *gorm.DB, monitorIDs []string) error {
	if len(monitorIDs) == 0 {
		return nil
	}
	steps := []*gorm.DB{
		tx.Where("monitor_id IN ?", monitorIDs).Delete(&db.MonitorReport{}),
		tx.Where("monitor_id IN ?", monitorIDs).Delete(&db.MonitorUptimeRollup{}),
		tx.Where("monitor_id IN ?", monitorIDs).Delete(&db.IncidentCandidateDismissal{}),
		tx.Where("resource_type = ? AND resource_id IN ?", "monitor", monitorIDs).Delete(&db.StatusPageComponentMapping{}),
		tx.Where("monitor_id IN ?", monitorIDs).Delete(&db.CoreMonitorConfig{}),
		tx.Model(&db.Incident{}).Where("monitor_id IN ?", monitorIDs).Update("monitor_id", ""),
		tx.Where("id IN ?", monitorIDs).Delete(&db.Monitor{}),
	}
	for _, step := range steps {
		if step.Error != nil {
			return step.Error
		}
	}
	return nil
}

// PurgeDeletedMonitors removes deleted monitors whose kept history has aged out.
func PurgeDeletedMonitors(database *gorm.DB, logger *utils.Logger, now time.Time) (int64, error) {
	var ids []string
	cutoff := now.AddDate(0, 0, -deletedMonitorHistoryDays)
	if err := database.Model(&db.Monitor{}).Where("lifecycle = ? AND deleted_at <> ? AND deleted_at < ?", "deleted", time.Time{}, cutoff).Limit(200).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := database.Transaction(func(tx *gorm.DB) error { return purgeMonitorsHistory(tx, ids) }); err != nil {
		return 0, err
	}
	logger.Info("Purged deleted monitors", "count", len(ids))
	return int64(len(ids)), nil
}

// CloneCoreMonitor makes a copy with " copy" appended to the name (numbered if taken).
func (s *CoreMonitorManagementService) CloneCoreMonitor(monitorID string) (*CoreManagedMonitorRecord, error) {
	source, err := s.GetCoreMonitorConfig(monitorID)
	if err != nil {
		return nil, err
	}
	config, secrets, err := decodeJSONObjects(source.Config.ConfigJSON, source.Config.SecretRefJSON)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(source.Monitor.Name) + " copy"
	for i := 2; i < 100; i++ {
		var taken int64
		s.db.Model(&db.Monitor{}).Where("name = ? AND lifecycle <> ?", name, "deleted").Count(&taken)
		if taken == 0 {
			break
		}
		name = fmt.Sprintf("%s copy %d", strings.TrimSpace(source.Monitor.Name), i)
	}
	for attempt := 0; ; attempt++ {
		record, err := s.cloneAs(source, config, secrets, name)
		if err != nil && attempt < 5 && strings.Contains(err.Error(), "UNIQUE") {
			name = fmt.Sprintf("%s copy %d", strings.TrimSpace(source.Monitor.Name), attempt+3)
			continue
		}
		return record, err
	}
}

func (s *CoreMonitorManagementService) cloneAs(source *CoreManagedMonitorRecord, config, secrets map[string]interface{}, name string) (*CoreManagedMonitorRecord, error) {
	return s.CreateCoreMonitor(CoreManagedMonitorCreateRequest{
		Name: name, Description: source.Monitor.Description, Type: source.Monitor.Type, Kind: source.Config.Kind, Config: config, SecretRefs: secrets,
		IntervalSeconds: source.Config.IntervalSeconds, TimeoutSeconds: source.Config.TimeoutSeconds, ConfirmationPeriodSeconds: source.Config.ConfirmationPeriodSeconds,
		ConfirmationCheckCount: source.Config.ConfirmationCheckCount, RecoveryPeriodSeconds: source.Config.RecoveryPeriodSeconds, RunbookURL: source.Monitor.RunbookURL,
	})
}

var errBadStoredConfig = errors.New("stored monitor config is not valid JSON")

func decodeJSONObjects(configJSON, secretsJSON string) (map[string]interface{}, map[string]interface{}, error) {
	config, secrets := map[string]interface{}{}, map[string]interface{}{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, nil, errBadStoredConfig
	}
	if err := json.Unmarshal([]byte(secretsJSON), &secrets); err != nil {
		return nil, nil, errBadStoredConfig
	}
	return config, secrets, nil
}

func (s *CoreMonitorManagementService) DeleteCoreMonitor(monitorID string) error {
	return s.DeleteCoreMonitorWith(monitorID, true)
}

// DeleteCoreMonitorWith deletes a monitor. With keepHistory the reports stay for
// deletedMonitorHistoryDays and retention purges them; without it everything goes now.
// Incidents are never deleted.
func (s *CoreMonitorManagementService) DeleteCoreMonitorWith(monitorID string, keepHistory bool) error {
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureCoreMonitorExists(tx, monitorID); err != nil {
			return err
		}
		if err := resolveIncidentsForRemovedMonitor(tx, s.logger, monitorID); err != nil {
			return err
		}
		if !keepHistory {
			return purgeMonitorsHistory(tx, []string{monitorID})
		}
		if err := tx.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(map[string]interface{}{
			"lifecycle":  "deleted",
			"health":     "unknown",
			"updated_at": now,
			"deleted_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorID).Updates(map[string]interface{}{
			"paused":           true,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"updated_at":       now,
		}).Error
	})
}

// resolveIncidentsForRemovedMonitor closes the monitor's active incident (resolution kind
// monitor_removed) and stops its undelivered alerts, so nothing pages for a deleted monitor.
func resolveIncidentsForRemovedMonitor(tx *gorm.DB, logger *utils.Logger, monitorID string) error {
	incidents := NewIncidentService(tx, logger, nil)
	incidents.inTransaction = true
	incident, found, err := incidents.findActiveIncident(monitorID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := incidents.resolveIncidentRecordTx(incident, "Monitor removed; active incident resolved", "", "unknown", "monitor_removed", false, incidentSystemEventMetadata()); err != nil {
		return err
	}
	return tx.Model(&db.AlertDelivery{}).Where("incident_id = ? AND status IN ?", incident.ID, []string{"pending", "failed"}).
		Updates(map[string]any{"status": "suppressed", "error": "monitor removed", "locked_until": nil}).Error
}
