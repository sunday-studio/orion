package monitorvalidation

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

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
