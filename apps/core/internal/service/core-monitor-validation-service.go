package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"orion/core/internal/monitorvalidation"
	"strings"
)

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

func validateCoreManagedMonitorConfigWithPolicy(kind string, configJSON string, secretRefJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateConfigWithPolicy(kind, configJSON, secretRefJSON, targetPolicy)
}

func (s *CoreMonitorManagementService) ValidateCoreMonitorConfig(kind string, configJSON string, secretRefJSON string) error {
	return validateCoreManagedMonitorConfigWithPolicy(kind, configJSON, secretRefJSON, s.targetPolicy)
}

func HashHeartbeatMonitorToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func validateCoreDomainExpirationMonitorConfig(configJSON string) error {
	return validateCoreDomainExpirationMonitorConfigWithPolicy(configJSON, NewCoreMonitorTargetPolicy(nil))
}

func validateCoreDomainExpirationMonitorConfigWithPolicy(configJSON string, targetPolicy CoreMonitorTargetPolicy) error {
	return monitorvalidation.ValidateDomainExpirationConfigWithPolicy(configJSON, targetPolicy)
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
