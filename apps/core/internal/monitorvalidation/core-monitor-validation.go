package monitorvalidation

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
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

func ValidateUDPConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Host             string `json:"host"`
		Port             int    `json:"port"`
		Payload          string `json:"payload"`
		ExpectedResponse string `json:"expected_response"`
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
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrValidation)
	}
	if cfg.Payload == "" {
		return fmt.Errorf("%w: payload is required", ErrValidation)
	}
	if cfg.ExpectedResponse == "" {
		return fmt.Errorf("%w: expected_response is required", ErrValidation)
	}
	return nil
}

func ValidateDNSConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Host       string `json:"host"`
		RecordType string `json:"record_type"`
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
	recordType := strings.ToUpper(strings.TrimSpace(cfg.RecordType))
	if recordType == "" {
		recordType = "A"
	}
	switch recordType {
	case "A", "AAAA", "CNAME", "TXT", "MX", "NS":
		return nil
	default:
		return fmt.Errorf("%w: record_type must be one of A, AAAA, CNAME, TXT, MX, NS", ErrValidation)
	}
}

func ValidateTLSConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Host        string `json:"host"`
		Port        int    `json:"port"`
		WarningDays int    `json:"warning_days"`
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
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrValidation)
	}
	if cfg.WarningDays < 0 {
		return fmt.Errorf("%w: warning_days must be zero or greater", ErrValidation)
	}
	return nil
}

func ValidateDomainExpirationConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Domain      string `json:"domain"`
		RDAPURL     string `json:"rdap_url"`
		WHOISServer string `json:"whois_server"`
		WarningDays int    `json:"warning_days"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		return fmt.Errorf("%w: domain is required", ErrValidation)
	}
	if strings.ContainsAny(cfg.Domain, "/:@") {
		return fmt.Errorf("%w: domain must be a hostname, not a URL", ErrValidation)
	}
	if err := targetPolicy.ValidateHost(cfg.Domain, "domain"); err != nil {
		return err
	}
	if cfg.WarningDays < 0 {
		return fmt.Errorf("%w: warning_days must be zero or greater", ErrValidation)
	}
	if strings.TrimSpace(cfg.RDAPURL) != "" {
		if err := targetPolicy.ValidateURL(cfg.RDAPURL, "rdap_url"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.WHOISServer) != "" {
		return ValidateWHOISServerWithPolicy(cfg.WHOISServer, targetPolicy)
	}
	return nil
}

func ValidateWHOISServerWithPolicy(value string, targetPolicy TargetPolicy) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "/@") {
		return fmt.Errorf("%w: whois_server must be a hostname with optional port", ErrValidation)
	}
	host := value
	port := ""
	if strings.HasPrefix(value, "[") {
		var err error
		host, port, err = net.SplitHostPort(value)
		if err != nil {
			return fmt.Errorf("%w: whois_server must be a hostname with optional port", ErrValidation)
		}
	}
	if strings.Count(value, ":") == 1 {
		parts := strings.SplitN(value, ":", 2)
		host = parts[0]
		port = parts[1]
	}
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("%w: whois_server host is required", ErrValidation)
	}
	if err := targetPolicy.ValidateHost(host, "whois_server host"); err != nil {
		return err
	}
	if port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return fmt.Errorf("%w: whois_server port must be between 1 and 65535", ErrValidation)
		}
	}
	return nil
}

func ValidatePingConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Host   string `json:"host"`
		Method string `json:"method"`
		Port   int    `json:"port"`
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
	method := strings.ToLower(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = "tcp"
	}
	switch method {
	case "tcp":
		if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
			return fmt.Errorf("%w: port must be between 1 and 65535", ErrValidation)
		}
	case "icmp":
		if cfg.Port != 0 {
			return fmt.Errorf("%w: port is unsupported for icmp ping", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: method must be one of tcp, icmp", ErrValidation)
	}
	return nil
}

func ValidateMailConfigWithPolicy(kind string, configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Protocol    string `json:"protocol"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		TLSMode     string `json:"tls_mode"`
		AuthEnabled bool   `json:"auth_enabled"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	protocol := normalizeMailProtocol(cfg.Protocol)
	if protocol == "" {
		protocol = normalizeMailProtocol(kind)
	}
	if protocol == "" || protocol == "mail" {
		return fmt.Errorf("%w: protocol must be one of smtp, imap, pop", ErrValidation)
	}
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrValidation)
	}
	if err := targetPolicy.ValidateHost(cfg.Host, "host"); err != nil {
		return err
	}
	tlsMode := strings.ToLower(strings.TrimSpace(cfg.TLSMode))
	if tlsMode == "" {
		tlsMode = "none"
	}
	switch tlsMode {
	case "none", "implicit", "starttls":
	default:
		return fmt.Errorf("%w: tls_mode must be one of none, implicit, starttls", ErrValidation)
	}
	if cfg.Port != 0 && (cfg.Port < 1 || cfg.Port > 65535) {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrValidation)
	}
	if cfg.AuthEnabled {
		return fmt.Errorf("%w: auth_enabled is not supported for mail monitors yet", ErrValidation)
	}
	return nil
}

func ValidateSyntheticConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		Variables map[string]string `json:"variables"`
		Steps     []struct {
			Type             string `json:"type"`
			URL              string `json:"url"`
			Method           string `json:"method"`
			ExpectedStatus   int    `json:"expected_status"`
			ExpectedStatuses []int  `json:"expected_statuses"`
			Request          *struct {
				URL              string `json:"url"`
				Method           string `json:"method"`
				ExpectedStatus   int    `json:"expected_status"`
				ExpectedStatuses []int  `json:"expected_statuses"`
			} `json:"request"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	if len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: steps are required", ErrValidation)
	}
	if len(cfg.Steps) > maxSyntheticSteps {
		return fmt.Errorf("%w: steps must contain at most %d items", ErrValidation, maxSyntheticSteps)
	}
	if len(cfg.Variables) > maxSyntheticVariables {
		return fmt.Errorf("%w: variables must contain at most %d entries", ErrValidation, maxSyntheticVariables)
	}
	for key, value := range cfg.Variables {
		if !variableNameValid(key) {
			return fmt.Errorf("%w: variable %q has invalid name", ErrValidation, key)
		}
		if len(value) > maxSyntheticVariableLength {
			return fmt.Errorf("%w: variable %q exceeds %d bytes", ErrValidation, key, maxSyntheticVariableLength)
		}
	}
	for index, step := range cfg.Steps {
		stepType := strings.ToLower(strings.TrimSpace(step.Type))
		if stepType == "" || stepType == "http" {
			stepType = "api"
		}
		switch stepType {
		case "api":
			targetURL := strings.TrimSpace(step.URL)
			method := strings.ToUpper(strings.TrimSpace(step.Method))
			expectedStatus := step.ExpectedStatus
			expectedStatuses := step.ExpectedStatuses
			if step.Request != nil {
				if strings.TrimSpace(step.Request.URL) != "" {
					targetURL = strings.TrimSpace(step.Request.URL)
				}
				if strings.TrimSpace(step.Request.Method) != "" {
					method = strings.ToUpper(strings.TrimSpace(step.Request.Method))
				}
				if step.Request.ExpectedStatus != 0 {
					expectedStatus = step.Request.ExpectedStatus
				}
				if len(step.Request.ExpectedStatuses) > 0 {
					expectedStatuses = step.Request.ExpectedStatuses
				}
			}
			if strings.TrimSpace(targetURL) == "" {
				return fmt.Errorf("%w: step %d url is required", ErrValidation, index+1)
			}
			if !strings.Contains(targetURL, "{{") {
				if err := targetPolicy.ValidateURL(targetURL, fmt.Sprintf("step %d url", index+1)); err != nil {
					return err
				}
			}
			if method == "" {
				method = http.MethodGet
			}
			if !apiRequestMethodAllowed(method) {
				return fmt.Errorf("%w: step %d method must be GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS", ErrValidation, index+1)
			}
			if err := validateExpectedStatuses(expectedStatus, expectedStatuses); err != nil {
				return err
			}
		case "browser":
			continue
		default:
			return fmt.Errorf("%w: step %d type must be api, http, or browser", ErrValidation, index+1)
		}
	}
	return nil
}

func ValidatePlaywrightConfigWithPolicy(configJSON string, targetPolicy TargetPolicy) error {
	var cfg struct {
		URL      string `json:"url"`
		StartURL string `json:"start_url"`
		Browser  string `json:"browser"`
		Steps    []struct {
			Name      string `json:"name"`
			Action    string `json:"action"`
			URL       string `json:"url"`
			Selector  string `json:"selector"`
			Value     string `json:"value"`
			Text      string `json:"text"`
			Contains  string `json:"contains"`
			TimeoutMS int    `json:"timeout_ms"`
		} `json:"steps"`
		ArtifactLimitBytes int `json:"artifact_limit_bytes"`
		Viewport           struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return fmt.Errorf("%w: parse config json: %v", ErrValidation, err)
	}
	targetURL := strings.TrimSpace(cfg.URL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(cfg.StartURL)
	}
	if targetURL == "" && len(cfg.Steps) == 0 {
		return fmt.Errorf("%w: url or steps are required", ErrValidation)
	}
	if targetURL != "" {
		if err := targetPolicy.ValidateURL(targetURL, "url"); err != nil {
			return err
		}
	}
	browser := strings.ToLower(strings.TrimSpace(cfg.Browser))
	if browser != "" {
		switch browser {
		case "chromium", "firefox", "webkit":
		default:
			return fmt.Errorf("%w: browser must be chromium, firefox, or webkit", ErrValidation)
		}
	}
	if cfg.Viewport.Width != 0 || cfg.Viewport.Height != 0 {
		if cfg.Viewport.Width < 320 || cfg.Viewport.Width > 3840 || cfg.Viewport.Height < 240 || cfg.Viewport.Height > 2160 {
			return fmt.Errorf("%w: viewport must be between 320x240 and 3840x2160", ErrValidation)
		}
	}
	if cfg.ArtifactLimitBytes < 0 || cfg.ArtifactLimitBytes > maxPlaywrightArtifactBytes {
		return fmt.Errorf("%w: artifact_limit_bytes must be between 0 and %d", ErrValidation, maxPlaywrightArtifactBytes)
	}
	if len(cfg.Steps) > maxPlaywrightSteps {
		return fmt.Errorf("%w: steps must contain at most %d items", ErrValidation, maxPlaywrightSteps)
	}
	for index, step := range cfg.Steps {
		action := strings.ToLower(strings.TrimSpace(step.Action))
		if action == "" {
			action = "goto"
		}
		switch action {
		case "goto", "click", "fill", "select", "check", "wait_for_selector", "text_contains", "assert_text", "assert_url", "screenshot":
		default:
			return fmt.Errorf("%w: step %d action is unsupported", ErrValidation, index+1)
		}
		if step.TimeoutMS < 0 || step.TimeoutMS > 60000 {
			return fmt.Errorf("%w: step %d timeout_ms must be between 0 and 60000", ErrValidation, index+1)
		}
		if action == "goto" {
			if strings.TrimSpace(step.URL) == "" {
				return fmt.Errorf("%w: step %d url is required", ErrValidation, index+1)
			}
			if !strings.Contains(step.URL, "{{") {
				if err := targetPolicy.ValidateURL(step.URL, fmt.Sprintf("step %d url", index+1)); err != nil {
					return err
				}
			}
		}
		if playwrightActionRequiresSelector(action) && strings.TrimSpace(step.Selector) == "" {
			return fmt.Errorf("%w: step %d selector is required", ErrValidation, index+1)
		}
		if len(step.Selector) > maxPlaywrightSelectorLen {
			return fmt.Errorf("%w: step %d selector exceeds %d bytes", ErrValidation, index+1, maxPlaywrightSelectorLen)
		}
		if len(step.Value) > maxPlaywrightValueLen || len(step.Text) > maxPlaywrightValueLen || len(step.Contains) > maxPlaywrightValueLen {
			return fmt.Errorf("%w: step %d value exceeds %d bytes", ErrValidation, index+1, maxPlaywrightValueLen)
		}
	}
	return nil
}

func RedactConfigJSON(configJSON string) map[string]interface{} {
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactValue(value).(map[string]interface{})
}

func RedactSecretRefJSON(secretRefJSON string) map[string]interface{} {
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(secretRefJSON), &value); err != nil {
		return map[string]interface{}{}
	}
	return redactSecretRefValue(value).(map[string]interface{})
}

func SanitizeURL(rawURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return strings.TrimSpace(rawURL)
	}
	parsedURL.User = nil
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String()
}

func apiRequestMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func hasNonEmptyString(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func playwrightActionRequiresSelector(action string) bool {
	switch action {
	case "click", "fill", "select", "check", "wait_for_selector", "text_contains", "assert_text":
		return true
	default:
		return false
	}
}

func variableNameValid(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if index == 0 {
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || char == '_' {
				continue
			}
			return false
		}
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

func validateExpectedStatuses(expectedStatus int, expectedStatuses []int) error {
	if expectedStatus != 0 && (expectedStatus < 100 || expectedStatus > 599) {
		return fmt.Errorf("%w: expected_status must be between 100 and 599", ErrValidation)
	}
	for _, status := range expectedStatuses {
		if status < 100 || status > 599 {
			return fmt.Errorf("%w: expected_statuses must contain values between 100 and 599", ErrValidation)
		}
	}
	return nil
}

func normalizeMailProtocol(protocol string) string {
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

func redactValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			if isSensitiveConfigKey(key) {
				redacted[key] = "[redacted]"
				continue
			}
			if isURLConfigKey(key) {
				if rawURL, ok := nested.(string); ok {
					redacted[key] = SanitizeURL(rawURL)
					continue
				}
			}
			redacted[key] = redactValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactValue(nested))
		}
		return redacted
	default:
		return typed
	}
}

func isSensitiveConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, token := range []string{"secret", "token", "password", "api_key", "apikey", "authorization", "auth_header", "private_key"} {
		if strings.Contains(key, token) {
			return true
		}
	}
	return false
}

func isURLConfigKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	switch key {
	case "url", "start_url", "rdap_url", "target_url":
		return true
	default:
		return false
	}
}

func redactSecretRefValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			redacted[key] = redactSecretRefValue(nested)
		}
		return redacted
	case []interface{}:
		redacted := make([]interface{}, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactSecretRefValue(nested))
		}
		return redacted
	default:
		return "[redacted]"
	}
}
