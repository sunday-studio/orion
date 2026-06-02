package monitorvalidation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	maxSyntheticSteps          = 10
	maxSyntheticVariables      = 10
	maxSyntheticVariableLength = 1024
	maxPlaywrightSteps         = 30
	maxPlaywrightArtifactBytes = 256 * 1024
	maxPlaywrightSelectorLen   = 2048
	maxPlaywrightValueLen      = 4096
)

var (
	ErrUnsupportedKind = errors.New("unsupported core monitor kind")
	ErrValidation      = errors.New("invalid core monitor")
)

type TargetPolicy interface {
	ValidateURL(rawURL string, field string) error
	ValidateHost(host string, field string) error
}

func NormalizeKind(kind string) string {
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

func IsSupportedKind(kind string) bool {
	switch NormalizeKind(kind) {
	case "heartbeat", "http", "http_keyword", "expected_status", "tcp", "dns", "tls", "udp", "api_request", "domain_expiration", "ping", "mail", "smtp", "imap", "pop", "pop3", "synthetic", "playwright":
		return true
	default:
		return false
	}
}

func ValidateConfigWithPolicy(kind string, configJSON string, secretRefJSON string, targetPolicy TargetPolicy) error {
	if strings.TrimSpace(secretRefJSON) != "" && strings.TrimSpace(secretRefJSON) != "{}" && !json.Valid([]byte(secretRefJSON)) {
		return fmt.Errorf("%w: secret refs must be valid JSON", ErrValidation)
	}

	switch NormalizeKind(kind) {
	case "heartbeat":
		return ValidateHeartbeatConfig(configJSON)
	case "http", "http_keyword", "expected_status":
		return ValidateHTTPConfigWithPolicy(kind, configJSON, targetPolicy)
	case "api_request":
		return ValidateAPIRequestConfigWithPolicy(configJSON, targetPolicy)
	case "tcp":
		return ValidateHostPortConfigWithPolicy(configJSON, true, targetPolicy)
	case "udp":
		return ValidateUDPConfigWithPolicy(configJSON, targetPolicy)
	case "dns":
		return ValidateDNSConfigWithPolicy(configJSON, targetPolicy)
	case "tls":
		return ValidateTLSConfigWithPolicy(configJSON, targetPolicy)
	case "domain_expiration":
		return ValidateDomainExpirationConfigWithPolicy(configJSON, targetPolicy)
	case "ping":
		return ValidatePingConfigWithPolicy(configJSON, targetPolicy)
	case "mail", "smtp", "imap", "pop", "pop3":
		return ValidateMailConfigWithPolicy(kind, configJSON, targetPolicy)
	case "synthetic":
		return ValidateSyntheticConfigWithPolicy(configJSON, targetPolicy)
	case "playwright":
		return ValidatePlaywrightConfigWithPolicy(configJSON, targetPolicy)
	default:
		return ErrUnsupportedKind
	}
}

func ValidateHeartbeatConfig(configJSON string) error {
	var cfg struct {
		GraceSeconds int `json:"grace_seconds"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if cfg.GraceSeconds < 0 {
		return fmt.Errorf("%w: grace_seconds must be zero or greater", ErrValidation)
	}
	return nil
}

func ValidateHTTPConfigWithPolicy(kind string, configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		URL               string   `json:"url"`
		Method            string   `json:"method"`
		ExpectedStatus    int      `json:"expected_status"`
		ExpectedStatuses  []int    `json:"expected_statuses"`
		RequiredContains  []string `json:"required_contains"`
		ForbiddenContains []string `json:"forbidden_contains"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if err := targetPolicy.ValidateURL(cfg.URL, "url"); err != nil {
		return err
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead {
		return fmt.Errorf("%w: method must be GET or HEAD", ErrValidation)
	}
	normalizedKind := NormalizeKind(kind)
	if normalizedKind == "http_keyword" && !hasNonEmptyString(cfg.RequiredContains) && !hasNonEmptyString(cfg.ForbiddenContains) {
		return fmt.Errorf("%w: required_contains or forbidden_contains is required", ErrValidation)
	}
	if normalizedKind == "expected_status" && cfg.ExpectedStatus == 0 && len(cfg.ExpectedStatuses) == 0 {
		return fmt.Errorf("%w: expected_status or expected_statuses is required", ErrValidation)
	}
	return validateExpectedStatuses(cfg.ExpectedStatus, cfg.ExpectedStatuses)
}

func ValidateAPIRequestConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		URL              string `json:"url"`
		Method           string `json:"method"`
		ExpectedStatus   int    `json:"expected_status"`
		ExpectedStatuses []int  `json:"expected_statuses"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if err := targetPolicy.ValidateURL(cfg.URL, "url"); err != nil {
		return err
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	if !apiRequestMethodAllowed(method) {
		return fmt.Errorf("%w: method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS", ErrValidation)
	}
	return validateExpectedStatuses(cfg.ExpectedStatus, cfg.ExpectedStatuses)
}

func ValidateHostPortConfigWithPolicy(configJSON string, portRequired bool, targetPolicy TargetPolicy) error {
	var cfg struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrValidation)
	}
	if err := targetPolicy.ValidateHost(cfg.Host, "host"); err != nil {
		return err
	}
	if portRequired && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrValidation)
	}
	return nil
}
