package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const defaultCoreManagedMonitorIntervalSeconds = 60
const defaultCoreManagedMonitorTimeoutSeconds = 10

var (
	ErrCoreManagedMonitorNotFound        = errors.New("core monitor not found")
	ErrCoreManagedMonitorUnsupportedKind = errors.New("unsupported core monitor kind")
	ErrCoreManagedMonitorValidation      = errors.New("invalid core monitor")
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
	RecoveryPeriodSeconds     *int                   `json:"recovery_period_seconds,omitempty"`
	Paused                    *bool                  `json:"paused,omitempty"`
}

type CoreManagedMonitorRecord struct {
	Monitor        db.Monitor
	Config         db.CoreMonitorConfig
	HeartbeatToken string
}

type CoreMonitorManagementService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewCoreMonitorManagementService(database *gorm.DB, logger *logging.Logger) *CoreMonitorManagementService {
	return &CoreMonitorManagementService{db: database, logger: logger}
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
	if err := validateCoreManagedMonitorConfig(kind, configJSON, secretRefJSON); err != nil {
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
		if err := validateCoreManagedMonitorConfig(nextKind, nextConfigJSON, nextSecretRefJSON); err != nil {
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
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "heartbeat":
		return "heartbeat"
	case "http", "http_status":
		return "http"
	case "http_keyword":
		return "http_keyword"
	case "expected_status":
		return "expected_status"
	case "tcp", "tcp_port":
		return "tcp"
	case "dns":
		return "dns"
	case "tls", "tls_certificate":
		return "tls"
	case "udp":
		return "udp"
	case "api_request":
		return "api_request"
	case "domain_expiration":
		return "domain_expiration"
	case "ping":
		return "ping"
	case "mail", "smtp", "imap", "pop", "pop3":
		return strings.ToLower(strings.TrimSpace(kind))
	case "synthetic", "synthetic_multi_step":
		return "synthetic"
	case "playwright", "playwright_transaction":
		return "playwright"
	default:
		return strings.ToLower(strings.TrimSpace(kind))
	}
}

func isSupportedCoreManagedMonitorKind(kind string) bool {
	switch normalizeCoreManagedMonitorKind(kind) {
	case "heartbeat", "http", "http_keyword", "expected_status", "tcp", "dns", "tls", "udp", "api_request", "domain_expiration", "ping", "mail", "smtp", "imap", "pop", "pop3", "synthetic", "playwright":
		return true
	default:
		return false
	}
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
	if strings.TrimSpace(secretRefJSON) != "" && strings.TrimSpace(secretRefJSON) != "{}" && !json.Valid([]byte(secretRefJSON)) {
		return fmt.Errorf("%w: secret refs must be valid JSON", ErrCoreManagedMonitorValidation)
	}

	switch normalizeCoreManagedMonitorKind(kind) {
	case "heartbeat":
		return validateCoreHeartbeatMonitorConfig(configJSON)
	case "http", "http_keyword", "expected_status":
		return validateCoreHTTPMonitorConfig(configJSON)
	case "api_request":
		return validateCoreAPIRequestMonitorConfig(configJSON)
	case "tcp":
		return validateCoreHostPortConfig(configJSON, true)
	case "udp":
		return validateCoreUDPMonitorConfig(configJSON)
	case "dns":
		return validateCoreDNSMonitorConfig(configJSON)
	case "tls":
		return validateCoreTLSMonitorConfig(configJSON)
	case "domain_expiration":
		return validateCoreDomainExpirationMonitorConfig(configJSON)
	case "ping":
		return validateCorePingMonitorConfig(configJSON)
	case "mail", "smtp", "imap", "pop", "pop3":
		return validateCoreMailMonitorConfig(kind, configJSON)
	case "synthetic":
		return validateCoreSyntheticMonitorConfig(configJSON)
	case "playwright":
		return validateCorePlaywrightMonitorConfig(configJSON)
	default:
		return ErrCoreManagedMonitorUnsupportedKind
	}
}

func HashHeartbeatMonitorToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func validateCoreHeartbeatMonitorConfig(configJSON string) error {
	var cfg struct {
		GraceSeconds int `json:"grace_seconds"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if cfg.GraceSeconds < 0 {
		return fmt.Errorf("%w: grace_seconds must be zero or greater", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreHTTPMonitorConfig(configJSON string) error {
	var cfg struct {
		URL              string `json:"url"`
		Method           string `json:"method"`
		ExpectedStatus   int    `json:"expected_status"`
		ExpectedStatuses []int  `json:"expected_statuses"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if err := validateCoreHTTPURL(cfg.URL, "url"); err != nil {
		return err
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead {
		return fmt.Errorf("%w: method must be GET or HEAD", ErrCoreManagedMonitorValidation)
	}
	return validateCoreExpectedStatuses(cfg.ExpectedStatus, cfg.ExpectedStatuses)
}

func validateCoreAPIRequestMonitorConfig(configJSON string) error {
	var cfg struct {
		URL              string `json:"url"`
		Method           string `json:"method"`
		ExpectedStatus   int    `json:"expected_status"`
		ExpectedStatuses []int  `json:"expected_statuses"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if err := validateCoreHTTPURL(cfg.URL, "url"); err != nil {
		return err
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
	default:
		return fmt.Errorf("%w: method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS", ErrCoreManagedMonitorValidation)
	}
	return validateCoreExpectedStatuses(cfg.ExpectedStatus, cfg.ExpectedStatuses)
}

func validateCoreHostPortConfig(configJSON string, portRequired bool) error {
	var cfg struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	if portRequired && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreUDPMonitorConfig(configJSON string) error {
	var cfg struct {
		Host             string `json:"host"`
		Port             int    `json:"port"`
		Payload          string `json:"payload"`
		ExpectedResponse string `json:"expected_response"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrCoreManagedMonitorValidation)
	}
	if cfg.Payload == "" {
		return fmt.Errorf("%w: payload is required", ErrCoreManagedMonitorValidation)
	}
	if cfg.ExpectedResponse == "" {
		return fmt.Errorf("%w: expected_response is required", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreDNSMonitorConfig(configJSON string) error {
	var cfg struct {
		Host       string `json:"host"`
		RecordType string `json:"record_type"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	recordType := strings.ToUpper(strings.TrimSpace(cfg.RecordType))
	if recordType == "" {
		recordType = "A"
	}
	switch recordType {
	case "A", "AAAA", "CNAME", "TXT", "MX", "NS":
		return nil
	default:
		return fmt.Errorf("%w: record_type must be one of A, AAAA, CNAME, TXT, MX, NS", ErrCoreManagedMonitorValidation)
	}
}

func validateCoreTLSMonitorConfig(configJSON string) error {
	var cfg struct {
		Host        string `json:"host"`
		Port        int    `json:"port"`
		WarningDays int    `json:"warning_days"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrCoreManagedMonitorValidation)
	}
	if cfg.WarningDays < 0 {
		return fmt.Errorf("%w: warning_days must be zero or greater", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreDomainExpirationMonitorConfig(configJSON string) error {
	var cfg struct {
		Domain      string `json:"domain"`
		RDAPURL     string `json:"rdap_url"`
		WarningDays int    `json:"warning_days"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		return fmt.Errorf("%w: domain is required", ErrCoreManagedMonitorValidation)
	}
	if strings.ContainsAny(cfg.Domain, "/:@") {
		return fmt.Errorf("%w: domain must be a hostname, not a URL", ErrCoreManagedMonitorValidation)
	}
	if cfg.WarningDays < 0 {
		return fmt.Errorf("%w: warning_days must be zero or greater", ErrCoreManagedMonitorValidation)
	}
	if strings.TrimSpace(cfg.RDAPURL) != "" {
		return validateCoreHTTPURL(cfg.RDAPURL, "rdap_url")
	}
	return nil
}

func validateCorePingMonitorConfig(configJSON string) error {
	var cfg struct {
		Host   string `json:"host"`
		Method string `json:"method"`
		Port   int    `json:"port"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	method := strings.ToLower(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = "tcp"
	}
	switch method {
	case "tcp":
		if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
			return fmt.Errorf("%w: port must be between 1 and 65535", ErrCoreManagedMonitorValidation)
		}
	case "icmp":
		if cfg.Port != 0 {
			return fmt.Errorf("%w: port is unsupported for icmp ping", ErrCoreManagedMonitorValidation)
		}
	default:
		return fmt.Errorf("%w: method must be one of tcp, icmp", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreMailMonitorConfig(kind string, configJSON string) error {
	var cfg struct {
		Protocol    string `json:"protocol"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		TLSMode     string `json:"tls_mode"`
		AuthEnabled bool   `json:"auth_enabled"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	protocol := normalizeCoreMailProtocol(cfg.Protocol)
	if protocol == "" {
		protocol = normalizeCoreMailProtocol(kind)
	}
	if protocol == "" || protocol == "mail" {
		return fmt.Errorf("%w: protocol must be one of smtp, imap, pop", ErrCoreManagedMonitorValidation)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrCoreManagedMonitorValidation)
	}
	tlsMode := strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if tlsMode == "" {
		tlsMode = "none"
	}
	switch tlsMode {
	case "none", "implicit", "starttls":
	default:
		return fmt.Errorf("%w: tls_mode must be one of none, implicit, starttls", ErrCoreManagedMonitorValidation)
	}
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrCoreManagedMonitorValidation)
	}
	if cfg.AuthEnabled {
		return fmt.Errorf("%w: auth_enabled is not supported for mail monitors yet", ErrCoreManagedMonitorValidation)
	}
	return nil
}

func validateCoreSyntheticMonitorConfig(configJSON string) error {
	var cfg struct {
		Steps []struct {
			Type   string `json:"type"`
			URL    string `json:"url"`
			Method string `json:"method"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	if len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: steps are required", ErrCoreManagedMonitorValidation)
	}
	for index, step := range cfg.Steps {
		stepType := strings.ToLower(strings.TrimSpace(step.Type))
		if stepType == "" || stepType == "http" {
			stepType = "api"
		}
		if stepType != "api" {
			return fmt.Errorf("%w: step %d type must be api", ErrCoreManagedMonitorValidation, index+1)
		}
		if err := validateCoreHTTPURL(step.URL, fmt.Sprintf("step %d url", index+1)); err != nil {
			return err
		}
	}
	return nil
}

func validateCorePlaywrightMonitorConfig(configJSON string) error {
	var cfg struct {
		URL      string        `json:"url"`
		StartURL string        `json:"start_url"`
		Browser  string        `json:"browser"`
		Steps    []interface{} `json:"steps"`
		Viewport struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrCoreManagedMonitorValidation, err)
	}
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(cfg.StartURL)
	}
	if targetURL == "" && len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: url or steps are required", ErrCoreManagedMonitorValidation)
	}
	if targetURL != "" {
		if err := validateCoreHTTPURL(targetURL, "url"); err != nil {
			return err
		}
	}
	browser := strings.ToLower(strings.TrimSpace(cfg.Browser))
	if browser != "" {
		switch browser {
		case "chromium", "firefox", "webkit":
		default:
			return fmt.Errorf("%w: browser must be chromium, firefox, or webkit", ErrCoreManagedMonitorValidation)
		}
	}
	if cfg.Viewport.Width != 0 || cfg.Viewport.Height != 0 {
		if cfg.Viewport.Width < 320 || cfg.Viewport.Width > 3840 || cfg.Viewport.Height < 240 || cfg.Viewport.Height > 2160 {
			return fmt.Errorf("%w: viewport must be between 320x240 and 3840x2160", ErrCoreManagedMonitorValidation)
		}
	}
	return nil
}

func validateCoreExpectedStatuses(expectedStatus int, expectedStatuses []int) error {
	if expectedStatus != 0 && (expectedStatus < 100 || expectedStatus > 599) {
		return fmt.Errorf("%w: expected_status must be between 100 and 599", ErrCoreManagedMonitorValidation)
	}
	for _, status := range expectedStatuses {
		if status < 100 || status > 599 {
			return fmt.Errorf("%w: expected_statuses must contain values between 100 and 599", ErrCoreManagedMonitorValidation)
		}
	}
	return nil
}

func validateCoreHTTPURL(rawURL string, field string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("%w: %s is required", ErrCoreManagedMonitorValidation, field)
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %s is invalid: %v", ErrCoreManagedMonitorValidation, field, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%w: %s scheme must be http or https", ErrCoreManagedMonitorValidation, field)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("%w: %s host is required", ErrCoreManagedMonitorValidation, field)
	}
	return nil
}

func normalizeCoreMailProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "smtp", "smtps":
		return "smtp"
	case "imap", "imaps":
		return "imap"
	case "pop", "pop3", "pop3s":
		return "pop"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
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
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactCoreMonitorValue(value).(map[string]interface{})
}

func RedactCoreMonitorSecretRefJSON(secretRefJSON string) map[string]interface{} {
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(secretRefJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactCoreMonitorSecretRefValue(value).(map[string]interface{})
}

func redactCoreMonitorValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if isSensitiveCoreMonitorConfigKey(key) {
				redacted[key] = "[redacted]"
				continue
			}
			redacted[key] = redactCoreMonitorValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactCoreMonitorValue(nested))
		}
		return redacted
	default:
		return typed
	}
}

func isSensitiveCoreMonitorConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, token := range []string{"secret", "token", "password", "api_key", "apikey", "authorization", "auth_header", "private_key"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func redactCoreMonitorSecretRefValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			redacted[key] = redactCoreMonitorSecretRefValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactCoreMonitorSecretRefValue(nested))
		}
		return redacted
	default:
		return "[redacted]"
	}
}
