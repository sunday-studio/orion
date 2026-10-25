package worker

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"orion/core/internal/service"
	"strconv"
	"strings"
	"time"
)

func (a *App) runMailProtocol(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, conn net.Conn, cfg mailConfig, result *mailResult) error {
	switch cfg.Protocol {
	case "smtp":
		return a.runSMTPProtocol(ctx, reader, writer, conn, cfg, result)
	case "imap":
		return a.runIMAPProtocol(ctx, reader, writer, conn, cfg, result)
	case "pop":
		return a.runPOPProtocol(ctx, reader, writer, conn, cfg, result)
	default:
		result.FailureStage = "config"
		return fmt.Errorf("unsupported mail protocol %s", cfg.Protocol)
	}
}

func (a *App) runSMTPProtocol(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, conn net.Conn, cfg mailConfig, result *mailResult) error {
	code, lines, err := readSMTPReply(reader)
	if err != nil {
		result.FailureStage = mailFailureStage(err, "banner")
		return err
	}
	result.Banner = strings.Join(lines, "\n")
	if code != 220 {
		result.FailureStage = "banner"
		return fmt.Errorf("SMTP banner code %d, want 220", code)
	}
	if cfg.ExpectedBanner != "" && !strings.Contains(result.Banner, cfg.ExpectedBanner) {
		result.FailureStage = "banner"
		return fmt.Errorf("SMTP banner did not contain expected text")
	}

	if len(cfg.ExpectedCapabilities) > 0 || cfg.TLSMode == "starttls" {
		if err := writeMailLine(writer, "EHLO orion.local"); err != nil {
			result.FailureStage = "write"
			return err
		}
		code, ehloLines, err := readSMTPReply(reader)
		if err != nil {
			result.FailureStage = mailFailureStage(err, "capability")
			return err
		}
		if code != 250 {
			result.FailureStage = "capability"
			return fmt.Errorf("SMTP EHLO code %d, want 250", code)
		}
		result.Capabilities = normalizeMailCapabilities(stripSMTPReplyCodes(ehloLines))
	}
	return a.finishMailStartTLS(ctx, reader, writer, conn, cfg, result)
}

func (a *App) runIMAPProtocol(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, conn net.Conn, cfg mailConfig, result *mailResult) error {
	line, err := readMailLine(reader)
	if err != nil {
		result.FailureStage = mailFailureStage(err, "banner")
		return err
	}
	result.Banner = line
	if !strings.HasPrefix(strings.ToUpper(line), "* OK") {
		result.FailureStage = "banner"
		return fmt.Errorf("IMAP banner did not start with * OK")
	}
	if cfg.ExpectedBanner != "" && !strings.Contains(result.Banner, cfg.ExpectedBanner) {
		result.FailureStage = "banner"
		return fmt.Errorf("IMAP banner did not contain expected text")
	}

	if len(cfg.ExpectedCapabilities) > 0 || cfg.TLSMode == "starttls" {
		if err := writeMailLine(writer, "A001 CAPABILITY"); err != nil {
			result.FailureStage = "write"
			return err
		}
		capabilities, err := readIMAPCapabilities(reader, "A001")
		if err != nil {
			result.FailureStage = mailFailureStage(err, "capability")
			return err
		}
		result.Capabilities = normalizeMailCapabilities(capabilities)
	}
	return a.finishMailStartTLS(ctx, reader, writer, conn, cfg, result)
}

func (a *App) runPOPProtocol(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, conn net.Conn, cfg mailConfig, result *mailResult) error {
	line, err := readMailLine(reader)
	if err != nil {
		result.FailureStage = mailFailureStage(err, "banner")
		return err
	}
	result.Banner = line
	if !strings.HasPrefix(strings.ToUpper(line), "+OK") {
		result.FailureStage = "banner"
		return fmt.Errorf("POP banner did not start with +OK")
	}
	if cfg.ExpectedBanner != "" && !strings.Contains(result.Banner, cfg.ExpectedBanner) {
		result.FailureStage = "banner"
		return fmt.Errorf("POP banner did not contain expected text")
	}

	if len(cfg.ExpectedCapabilities) > 0 || cfg.TLSMode == "starttls" {
		if err := writeMailLine(writer, "CAPA"); err != nil {
			result.FailureStage = "write"
			return err
		}
		capabilities, err := readPOPCapabilities(reader)
		if err != nil {
			result.FailureStage = mailFailureStage(err, "capability")
			return err
		}
		result.Capabilities = normalizeMailCapabilities(capabilities)
	}
	return a.finishMailStartTLS(ctx, reader, writer, conn, cfg, result)
}

func (a *App) finishMailStartTLS(ctx context.Context, reader *bufio.Reader, writer *bufio.Writer, conn net.Conn, cfg mailConfig, result *mailResult) error {
	result.MissingCapabilities = missingMailCapabilities(result.Capabilities, result.ExpectedCapabilities)
	if len(result.MissingCapabilities) > 0 {
		result.FailureStage = "capability"
		return fmt.Errorf("expected mail capabilities missing: %s", strings.Join(result.MissingCapabilities, ", "))
	}
	if cfg.TLSMode != "starttls" {
		return nil
	}
	requiredCapability := map[string]string{"smtp": "STARTTLS", "imap": "STARTTLS", "pop": "STLS"}[cfg.Protocol]
	if !mailCapabilitiesContain(result.Capabilities, requiredCapability) {
		result.FailureStage = "starttls"
		return fmt.Errorf("%s capability is missing", requiredCapability)
	}

	switch cfg.Protocol {
	case "smtp":
		if err := writeMailLine(writer, "STARTTLS"); err != nil {
			result.FailureStage = "write"
			return err
		}
		code, _, err := readSMTPReply(reader)
		if err != nil {
			result.FailureStage = mailFailureStage(err, "starttls")
			return err
		}
		if code != 220 {
			result.FailureStage = "starttls"
			return fmt.Errorf("SMTP STARTTLS code %d, want 220", code)
		}
	case "imap":
		if err := writeMailLine(writer, "A002 STARTTLS"); err != nil {
			result.FailureStage = "write"
			return err
		}
		line, err := readMailLine(reader)
		if err != nil {
			result.FailureStage = mailFailureStage(err, "starttls")
			return err
		}
		if !strings.HasPrefix(strings.ToUpper(line), "A002 OK") {
			result.FailureStage = "starttls"
			return fmt.Errorf("IMAP STARTTLS response was not OK")
		}
	case "pop":
		if err := writeMailLine(writer, "STLS"); err != nil {
			result.FailureStage = "write"
			return err
		}
		line, err := readMailLine(reader)
		if err != nil {
			result.FailureStage = mailFailureStage(err, "starttls")
			return err
		}
		if !strings.HasPrefix(strings.ToUpper(line), "+OK") {
			result.FailureStage = "starttls"
			return fmt.Errorf("POP STLS response was not OK")
		}
	}

	tlsConn := tls.Client(conn, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.ServerName})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		result.FailureStage = mailFailureStage(err, "tls")
		return err
	}
	result.TLSNegotiated = true
	return nil
}

func readSMTPReply(reader *bufio.Reader) (int, []string, error) {
	firstLine, err := readMailLine(reader)
	if err != nil {
		return 0, nil, err
	}
	if len(firstLine) < 3 {
		return 0, nil, fmt.Errorf("short SMTP reply")
	}
	code, err := strconv.Atoi(firstLine[:3])
	if err != nil {
		return 0, nil, fmt.Errorf("parse SMTP reply code: %w", err)
	}
	lines := []string{firstLine}
	if len(firstLine) < 4 || firstLine[3] != '-' {
		return code, lines, nil
	}
	prefix := firstLine[:3] + " "
	for {
		line, err := readMailLine(reader)
		if err != nil {
			return code, lines, err
		}
		lines = append(lines, line)
		if strings.HasPrefix(line, prefix) {
			return code, lines, nil
		}
	}
}

func stripSMTPReplyCodes(lines []string) []string {
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= 4 && line[0] >= '0' && line[0] <= '9' {
			values = append(values, line[4:])
			continue
		}
		values = append(values, line)
	}
	return values
}

func readIMAPCapabilities(reader *bufio.Reader, tag string) ([]string, error) {
	capabilities := []string{}
	for {
		line, err := readMailLine(reader)
		if err != nil {
			return nil, err
		}
		upperLine := strings.ToUpper(line)
		if strings.HasPrefix(upperLine, "* CAPABILITY ") {
			capabilities = append(capabilities, strings.Fields(line[len("* CAPABILITY "):])...)
			continue
		}
		if strings.HasPrefix(upperLine, strings.ToUpper(tag)+" OK") {
			return capabilities, nil
		}
		if strings.HasPrefix(upperLine, strings.ToUpper(tag)+" ") {
			return nil, fmt.Errorf("IMAP capability command failed: %s", line)
		}
	}
}

func readPOPCapabilities(reader *bufio.Reader) ([]string, error) {
	line, err := readMailLine(reader)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.ToUpper(line), "+OK") {
		return nil, fmt.Errorf("POP CAPA response was not OK")
	}
	capabilities := []string{}
	for {
		line, err := readMailLine(reader)
		if err != nil {
			return nil, err
		}
		if line == "." {
			return capabilities, nil
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			capabilities = append(capabilities, fields[0])
		}
	}
}

func writeMailLine(writer *bufio.Writer, line string) error {
	if _, err := writer.WriteString(line + "\r\n"); err != nil {
		return err
	}
	return writer.Flush()
}

func readMailLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func normalizeMailCapabilities(values []string) []string {
	normalized := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if idx := strings.IndexAny(value, " ="); idx > 0 {
			value = value[:idx]
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func missingMailCapabilities(actual []string, expected []string) []string {
	missing := []string{}
	for _, value := range expected {
		if !mailCapabilitiesContain(actual, value) {
			missing = append(missing, value)
		}
	}
	return missing
}

func mailCapabilitiesContain(values []string, expected string) bool {
	expected = strings.ToUpper(strings.TrimSpace(expected))
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func mailFailureStage(err error, fallback string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return fallback
}

func (a *App) storeMailReport(monitorID string, result mailResult) error {
	payload := service.MonitorReportPayload{
		Timestamp: result.FinishedAt.Format(time.RFC3339Nano),
		Health:    result.Health,
		Metrics:   mailPayload(result, nil),
	}
	if result.Error != nil {
		payload.Error = mailPayload(result, result.Error)
	}
	_, err := a.reports.StoreMonitorReport(monitorID, payload)
	return err
}

func mailPayload(result mailResult, resultErr error) map[string]any {
	payload := map[string]any{
		"runner":                "core",
		"type":                  "mail",
		"protocol":              result.Protocol,
		"host":                  result.Host,
		"port":                  result.Port,
		"address":               result.Address,
		"tls_mode":              result.TLSMode,
		"server_name":           result.ServerName,
		"tls_negotiated":        result.TLSNegotiated,
		"banner":                result.Banner,
		"capabilities":          result.Capabilities,
		"expected_banner":       result.ExpectedBanner,
		"expected_capabilities": result.ExpectedCapabilities,
		"missing_capabilities":  result.MissingCapabilities,
		"auth_enabled":          result.AuthEnabled,
		"auth_attempted":        result.AuthAttempted,
		"duration_ms":           result.Duration.Milliseconds(),
		"ok":                    result.Health == "up",
		"collected_at":          result.FinishedAt.Format(time.RFC3339Nano),
		"failure_stage":         result.FailureStage,
	}
	if resultErr != nil {
		payload["error"] = resultErr.Error()
	}
	return payload
}
