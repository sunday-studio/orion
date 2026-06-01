package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/monitorvalidation"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const defaultCoreManagedMonitorIntervalSeconds = 60
const defaultCoreManagedMonitorTimeoutSeconds = 10

var (
	ErrCoreManagedMonitorNotFound        = errors.New("core monitor not found")
	ErrCoreManagedMonitorUnsupportedKind = monitorvalidation.ErrUnsupportedKind
	ErrCoreManagedMonitorValidation      = monitorvalidation.ErrValidation
)

type CoreManagedMonitorCreateRequest struct {
	Name                      string                 `json:"name" binding:"required"`
	Description               *string                `json:"description"`
	Type                      string                 `json:"type,omitempty"`
	Kind                      string                 `json:"kind" binding:"required"`
	Config                    map[string]interface{} `json:"config" binding:"required"`
	SecretRefs                map[string]interface{} `json:"secret_refs,omitempty"`
	IntervalSeconds           int                    `json:"interval_seconds,omitempty"`
	TimeoutSeconds            int                    `json:"timeout_seconds,omitempty"`
	ConfirmationPeriodSeconds int                    `json:"confirmation_period_seconds,omitempty"`
	ConfirmationCheckCount    int                    `json:"confirmation_check_count,omitempty"`
	RecoveryPeriodSeconds     int                    `json:"recovery_period_seconds,omitempty"`
	Paused                    bool                   `json:"paused,omitempty"`
}

type CoreManagedMonitorUpdateRequest struct {
	Name                      *string                `json:"name,omitempty"`
	Description               *string                `json:"description,omitempty"`
	Type                      *string                `json:"type,omitempty"`
	Kind                      *string                `json:"kind,omitempty"`
	Config                    map[string]interface{} `json:"config,omitempty"`
	SecretRefs                map[string]interface{} `json:"secret_refs,omitempty"`
	IntervalSeconds           *int                   `json:"interval_seconds,omitempty"`
	TimeoutSeconds            *int                   `json:"timeout_seconds,omitempty"`
	ConfirmationPeriodSeconds *int                   `json:"confirmation_period_seconds,omitempty"`
	ConfirmationCheckCount    *int                   `json:"confirmation_check_count,omitempty"`
	RecoveryPeriodSeconds     *int                   `json:"recovery_period_seconds,omitempty"`
	Paused                    *bool                  `json:"paused,omitempty"`
}

type CoreManagedMonitorRecord struct {
	Monitor        db.Monitor
	Config         db.CoreMonitorConfig
	HeartbeatToken string
}

type CoreMonitorManagementService struct {
	db           *gorm.DB
	logger       *logging.Logger
	targetPolicy CoreMonitorTargetPolicy
}

func NewCoreMonitorManagementService(database *gorm.DB, logger *logging.Logger, cfg ...*config.Config) *CoreMonitorManagementService {
	var runtimeConfig *config.Config
	if len(cfg) > 0 {
		runtimeConfig = cfg[0]
	}
	return &CoreMonitorManagementService{db: database, logger: logger, targetPolicy: NewCoreMonitorTargetPolicy(runtimeConfig)}
}

func (s *CoreMonitorManagementService) CreateCoreMonitor(req CoreManagedMonitorCreateRequest) (*CoreManagedMonitorRecord, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrCoreManagedMonitorValidation)
	}
	kind := normalizeCoreManagedMonitorKind(req.Kind)
	if !isSupportedCoreManagedMonitorKind(kind) {
		return nil, ErrCoreManagedMonitorUnsupportedKind
	}
	monitorType := normalizeCoreManagedMonitorKind(req.Type)
	if monitorType == "" {
		monitorType = kind
	}
	if !isSupportedCoreManagedMonitorKind(monitorType) {
		return nil, ErrCoreManagedMonitorUnsupportedKind
	}

	configJSON, err := marshalJSONObject(req.Config)
	if err != nil {
		return nil, err
	}
	secretRefJSON, err := marshalJSONObject(req.SecretRefs)
	if err != nil {
		return nil, err
	}
	if err := validateCoreManagedMonitorConfigWithPolicy(kind, configJSON, secretRefJSON, s.targetPolicy); err != nil {
		return nil, err
	}

	interval := boundedPositive(req.IntervalSeconds, defaultCoreManagedMonitorIntervalSeconds)
	timeout := boundedPositive(req.TimeoutSeconds, defaultCoreManagedMonitorTimeoutSeconds)
	now := time.Now().UTC()
	record := CoreManagedMonitorRecord{}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		owner, err := NewIncidentService(tx, s.logger, nil).ensureCoreOwnerAgent("agent-core-worker")
		if err != nil {
			return err
		}

		monitor := db.Monitor{
			ID:                       utils.GenerateID("monitor"),
			AgentID:                  owner.ID,
			Description:              req.Description,
			Type:                     monitorType,
			Name:                     name,
			Lifecycle:                "active",
			Health:                   "unknown",
			ComputedHealth:           "unknown",
			ReportingIntervalSeconds: interval,
			Meta:                     `{"owner":"core","source":"core"}`,
			CreatedAt:                now,
			UpdatedAt:                now,
		}
		if err := tx.Create(&monitor).Error; err != nil {
			return err
		}

		heartbeatToken := ""
		heartbeatTokenHash := ""
		if kind == "heartbeat" {
			heartbeatToken, err = utils.GenerateToken()
			if err != nil {
				return err
			}
			heartbeatTokenHash = HashHeartbeatMonitorToken(heartbeatToken)
		}

		nextRunAt := now
		if req.Paused {
			nextRunAt = now.Add(time.Duration(interval) * time.Second)
		}
		config := db.CoreMonitorConfig{
			MonitorID:                 monitor.ID,
			Kind:                      kind,
			ConfigJSON:                configJSON,
			SecretRefJSON:             secretRefJSON,
			HeartbeatTokenHash:        heartbeatTokenHash,
			IntervalSeconds:           interval,
			TimeoutSeconds:            timeout,
			ConfirmationPeriodSeconds: nonNegative(req.ConfirmationPeriodSeconds),
			ConfirmationCheckCount:    nonNegative(req.ConfirmationCheckCount),
			RecoveryPeriodSeconds:     nonNegative(req.RecoveryPeriodSeconds),
			Paused:                    req.Paused,
			NextRunAt:                 nextRunAt,
			CreatedAt:                 now,
			UpdatedAt:                 now,
		}
		if err := tx.Create(&config).Error; err != nil {
			return err
		}

		record = CoreManagedMonitorRecord{Monitor: monitor, Config: config, HeartbeatToken: heartbeatToken}
		return nil
	})
	if err != nil {
		s.logger.Error("Failed to create core monitor", "error", err)
		return nil, err
	}
	return &record, nil
}

func (s *CoreMonitorManagementService) UpdateCoreMonitor(monitorID string, req CoreManagedMonitorUpdateRequest) (*CoreManagedMonitorRecord, error) {
	monitorID = strings.TrimSpace(monitorID)
	if monitorID == "" {
		return nil, ErrCoreManagedMonitorNotFound
	}

	record := CoreManagedMonitorRecord{}
	now := time.Now().UTC()
	heartbeatToken := ""
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var monitor db.Monitor
		if err := tx.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}
		var config db.CoreMonitorConfig
		if err := tx.Where("monitor_id = ?", monitorID).First(&config).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}

		nextKind := config.Kind
		if req.Kind != nil {
			nextKind = normalizeCoreManagedMonitorKind(*req.Kind)
		}
		nextConfigJSON := config.ConfigJSON
		if req.Config != nil {
			configJSON, err := marshalJSONObject(req.Config)
			if err != nil {
				return err
			}
			nextConfigJSON = configJSON
		}
		nextSecretRefJSON := config.SecretRefJSON
		if req.SecretRefs != nil {
			secretRefJSON, err := marshalJSONObject(req.SecretRefs)
			if err != nil {
				return err
			}
			nextSecretRefJSON = secretRefJSON
		}
		if err := validateCoreManagedMonitorConfigWithPolicy(nextKind, nextConfigJSON, nextSecretRefJSON, s.targetPolicy); err != nil {
			return err
		}
		nextHeartbeatTokenHash := config.HeartbeatTokenHash
		if nextKind == "heartbeat" && nextHeartbeatTokenHash == "" {
			var err error
			heartbeatToken, err = utils.GenerateToken()
			if err != nil {
				return err
			}
			nextHeartbeatTokenHash = HashHeartbeatMonitorToken(heartbeatToken)
		}
		if nextKind != "heartbeat" {
			nextHeartbeatTokenHash = ""
		}

		monitorUpdates := map[string]interface{}{"updated_at": now}
		if req.Name != nil {
			name := strings.TrimSpace(*req.Name)
			if name == "" {
				return fmt.Errorf("%w: name is required", ErrCoreManagedMonitorValidation)
			}
			monitorUpdates["name"] = name
		}
		if req.Description != nil {
			monitorUpdates["description"] = req.Description
		}
		if req.Type != nil {
			monitorType := normalizeCoreManagedMonitorKind(*req.Type)
			if !isSupportedCoreManagedMonitorKind(monitorType) {
				return ErrCoreManagedMonitorUnsupportedKind
			}
			monitorUpdates["type"] = monitorType
		}
		if req.IntervalSeconds != nil {
			monitorUpdates["reporting_interval_seconds"] = boundedPositive(*req.IntervalSeconds, defaultCoreManagedMonitorIntervalSeconds)
		}
		if err := tx.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(monitorUpdates).Error; err != nil {
			return err
		}

		configUpdates := map[string]interface{}{"updated_at": now}
		if req.Kind != nil {
			kind := normalizeCoreManagedMonitorKind(*req.Kind)
			if !isSupportedCoreManagedMonitorKind(kind) {
				return ErrCoreManagedMonitorUnsupportedKind
			}
			configUpdates["kind"] = kind
		}
		if nextHeartbeatTokenHash != config.HeartbeatTokenHash {
			configUpdates["heartbeat_token_hash"] = nextHeartbeatTokenHash
		}
		if req.Config != nil {
			configUpdates["config_json"] = nextConfigJSON
		}
		if req.SecretRefs != nil {
			configUpdates["secret_ref_json"] = nextSecretRefJSON
		}
		if req.IntervalSeconds != nil {
			configUpdates["interval_seconds"] = boundedPositive(*req.IntervalSeconds, defaultCoreManagedMonitorIntervalSeconds)
		}
		if req.TimeoutSeconds != nil {
			configUpdates["timeout_seconds"] = boundedPositive(*req.TimeoutSeconds, defaultCoreManagedMonitorTimeoutSeconds)
		}
		if req.ConfirmationPeriodSeconds != nil {
			configUpdates["confirmation_period_seconds"] = nonNegative(*req.ConfirmationPeriodSeconds)
		}
		if req.ConfirmationCheckCount != nil {
			configUpdates["confirmation_check_count"] = nonNegative(*req.ConfirmationCheckCount)
		}
		if req.RecoveryPeriodSeconds != nil {
			configUpdates["recovery_period_seconds"] = nonNegative(*req.RecoveryPeriodSeconds)
		}
		if req.Paused != nil {
			configUpdates["paused"] = *req.Paused
			configUpdates["lease_owner"] = ""
			configUpdates["lease_expires_at"] = nil
			if !*req.Paused {
				configUpdates["next_run_at"] = now
			}
		}
		if err := tx.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorID).Updates(configUpdates).Error; err != nil {
			return err
		}

		if err := tx.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", monitorID).First(&config).Error; err != nil {
			return err
		}
		record = CoreManagedMonitorRecord{Monitor: monitor, Config: config, HeartbeatToken: heartbeatToken}
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrCoreManagedMonitorNotFound) && !errors.Is(err, ErrCoreManagedMonitorUnsupportedKind) {
			s.logger.Error("Failed to update core monitor", "monitor_id", monitorID, "error", err)
		}
		return nil, err
	}
	return &record, nil
}

func (s *CoreMonitorManagementService) DeleteCoreMonitor(monitorID string) error {
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureCoreMonitorExists(tx, monitorID); err != nil {
			return err
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

func (s *CoreMonitorManagementService) PauseCoreMonitor(monitorID string) (*CoreManagedMonitorRecord, error) {
	return s.setCoreMonitorPaused(monitorID, true)
}

func (s *CoreMonitorManagementService) ResumeCoreMonitor(monitorID string) (*CoreManagedMonitorRecord, error) {
	return s.setCoreMonitorPaused(monitorID, false)
}

func (s *CoreMonitorManagementService) GetCoreMonitorConfig(monitorID string) (*CoreManagedMonitorRecord, error) {
	var record CoreManagedMonitorRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", monitorID).First(&record.Monitor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}
		if err := tx.Where("monitor_id = ?", monitorID).First(&record.Config).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *CoreMonitorManagementService) GetHeartbeatMonitorByToken(token string) (*CoreManagedMonitorRecord, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrCoreManagedMonitorNotFound
	}

	var record CoreManagedMonitorRecord
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("kind = ? AND heartbeat_token_hash = ?", "heartbeat", HashHeartbeatMonitorToken(token)).
			First(&record.Config).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}
		if err := tx.Where("id = ?", record.Config.MonitorID).First(&record.Monitor).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCoreManagedMonitorNotFound
			}
			return err
		}
		if record.Monitor.Lifecycle == "deleted" {
			return ErrCoreManagedMonitorNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *CoreMonitorManagementService) RecordHeartbeatSignal(monitorID string, health string, receivedAt time.Time) error {
	updates := map[string]interface{}{
		"last_signal_at": receivedAt,
		"updated_at":     receivedAt,
	}
	switch health {
	case "up":
		updates["last_success_at"] = receivedAt
	case "down":
		updates["last_failure_at"] = receivedAt
	}
	return s.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorID).Updates(updates).Error
}

func (s *CoreMonitorManagementService) setCoreMonitorPaused(monitorID string, paused bool) (*CoreManagedMonitorRecord, error) {
	now := time.Now().UTC()
	record := CoreManagedMonitorRecord{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureCoreMonitorExists(tx, monitorID); err != nil {
			return err
		}
		updates := map[string]interface{}{
			"paused":           paused,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"updated_at":       now,
		}
		if !paused {
			updates["next_run_at"] = now
		}
		if err := tx.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("id = ?", monitorID).First(&record.Monitor).Error; err != nil {
			return err
		}
		if err := tx.Where("monitor_id = ?", monitorID).First(&record.Config).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func ensureCoreMonitorExists(tx *gorm.DB, monitorID string) error {
	monitorID = strings.TrimSpace(monitorID)
	if monitorID == "" {
		return ErrCoreManagedMonitorNotFound
	}
	var count int64
	if err := tx.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrCoreManagedMonitorNotFound
	}
	return nil
}

func normalizeCoreManagedMonitorKind(kind string) string {
	return monitorvalidation.NormalizeKind(kind)
}

func isSupportedCoreManagedMonitorKind(kind string) bool {
	return monitorvalidation.IsSupportedKind(kind)
}

func marshalJSONObject(value map[string]interface{}) (string, error) {
	if value == nil {
		value = map[string]interface{}{}
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func validateCoreManagedMonitorConfig(kind string, configJSON string, secretRefJSON string) error {
	return validateCoreManagedMonitorConfigWithPolicy(kind, configJSON, secretRefJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreManagedMonitorConfigWithPolicy(kind string, configJSON string, secretRefJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateConfigWithPolicy(kind, configJSON, secretRefJSON, targetPolicy)
}

func ValidateCoreManagedMonitorConfig(kind string, configJSON string, secretRefJSON string) error {
	return validateCoreManagedMonitorConfig(kind, configJSON, secretRefJSON)
}

func (s *CoreMonitorManagementService) ValidateCoreMonitorConfig(kind string, configJSON string, secretRefJSON string) error {
	return validateCoreManagedMonitorConfigWithPolicy(kind, configJSON, secretRefJSON, s.targetPolicy)
}

func HashHeartbeatMonitorToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func validateCoreHeartbeatMonitorConfig(configJSON string) error {
	return monitorvalidation.ValidateHeartbeatConfig(configJSON)
}

func validateCoreHTTPMonitorConfig(kind string, configJSON string) error {
	return validateCoreHTTPMonitorConfigWithPolicy(kind, configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreHTTPMonitorConfigWithPolicy(kind string, configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateHTTPConfigWithPolicy(kind, configJSON, targetPolicy)
}

func validateCoreAPIRequestMonitorConfig(configJSON string) error {
	return validateCoreAPIRequestMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreAPIRequestMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateAPIRequestConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreHostPortConfig(configJSON string, portRequired bool) error {
	return validateCoreHostPortConfigWithPolicy(configJSON, portRequired, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreHostPortConfigWithPolicy(configJSON string, portRequired bool, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateHostPortConfigWithPolicy(configJSON, portRequired, targetPolicy)
}

func validateCoreUDPMonitorConfig(configJSON string) error {
	return validateCoreUDPMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreUDPMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateUDPConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreDNSMonitorConfig(configJSON string) error {
	return validateCoreDNSMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreDNSMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateDNSConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreTLSMonitorConfig(configJSON string) error {
	return validateCoreTLSMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreTLSMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateTLSConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreDomainExpirationMonitorConfig(configJSON string) error {
	return validateCoreDomainExpirationMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreDomainExpirationMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateDomainExpirationConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreWHOISServer(value string) error {
	return validateCoreWHOISServerWithPolicy(value, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreWHOISServerWithPolicy(value string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateWHOISServerWithPolicy(value, targetPolicy)
}

func validateCorePingMonitorConfig(configJSON string) error {
	return validateCorePingMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCorePingMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidatePingConfigWithPolicy(configJSON, targetPolicy)
}

func validateCoreMailMonitorConfig(kind string, configJSON string) error {
	return validateCoreMailMonitorConfigWithPolicy(kind, configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreMailMonitorConfigWithPolicy(kind string, configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateMailConfigWithPolicy(kind, configJSON, targetPolicy)
}

func validateCoreSyntheticMonitorConfig(configJSON string) error {
	return validateCoreSyntheticMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreSyntheticMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateSyntheticConfigWithPolicy(configJSON, targetPolicy)
}

func validateCorePlaywrightMonitorConfig(configJSON string) error {
	return validateCorePlaywrightMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCorePlaywrightMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidatePlaywrightConfigWithPolicy(configJSON, targetPolicy)
}

func boundedPositive(value int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func RedactCoreMonitorConfigJSON(configJSON string) map[string]interface{} {
	return monitorvalidation.RedactConfigJSON(configJSON)
}

func RedactCoreMonitorSecretRefJSON(secretRefJSON string) map[string]interface{} {
	return monitorvalidation.RedactSecretRefJSON(secretRefJSON)
}
