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
