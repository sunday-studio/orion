package service

import (
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"orion/core/internal/config"
	"strconv"
	"strings"
	"time"
)

const alertSMTPTimeout = 20 * time.Second

// alertSMTPTLSConfig is overridden by tests that run an SMTP server with a self-signed certificate.
var alertSMTPTLSConfig = func(host string) *tls.Config { return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12} }

// AlertEmailMessage is one outbound alert mail.
type AlertEmailMessage struct {
	To      string
	Subject string
	Text    string
	Headers map[string]string
}

// AlertEmailConfigured reports whether Core can send alert mail.
func AlertEmailConfigured(cfg *config.Config) error {
	if cfg == nil || strings.TrimSpace(cfg.AlertSMTPHost) == "" || cfg.AlertSMTPPort <= 0 || strings.TrimSpace(cfg.AlertSMTPFrom) == "" {
		return errors.New("email alerts are not configured: set ORION_ALERT_SMTP_HOST, ORION_ALERT_SMTP_PORT and ORION_ALERT_SMTP_FROM")
	}
	if _, err := mail.ParseAddress(cfg.AlertSMTPFrom); err != nil {
		return errors.New("ORION_ALERT_SMTP_FROM is not a valid address")
	}
	switch cfg.AlertSMTPTLS {
	case "starttls", "tls", "none", "":
	default:
		return errors.New("ORION_ALERT_SMTP_TLS must be starttls, tls or none")
	}
	if alertEmailSecret(cfg) == "" || strings.TrimSpace(cfg.PublicURL) == "" {
		return errors.New("email alerts need ORION_PUBLIC_URL and ORION_ALERT_EMAIL_SECRET (or ORION_JWT_SECRET) for confirmation links")
	}
	return nil
}

// SendAlertEmail delivers one message over SMTP using the configured TLS mode.
func SendAlertEmail(cfg *config.Config, message AlertEmailMessage) error {
	if err := AlertEmailConfigured(cfg); err != nil {
		return err
	}
	from, _ := mail.ParseAddress(cfg.AlertSMTPFrom)
	to, err := mail.ParseAddress(message.To)
	if err != nil {
		return fmt.Errorf("invalid recipient address")
	}
	address := net.JoinHostPort(cfg.AlertSMTPHost, strconv.Itoa(cfg.AlertSMTPPort))
	dialer := &net.Dialer{Timeout: alertSMTPTimeout}
	var conn net.Conn
	if cfg.AlertSMTPTLS == "tls" {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, alertSMTPTLSConfig(cfg.AlertSMTPHost))
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return fmt.Errorf("smtp connect: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * alertSMTPTimeout))
	client, err := smtp.NewClient(conn, cfg.AlertSMTPHost)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer client.Close()
	if cfg.AlertSMTPTLS == "starttls" || cfg.AlertSMTPTLS == "" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return errors.New("smtp server does not offer STARTTLS; set ORION_ALERT_SMTP_TLS=none to allow plaintext")
		}
		if err := client.StartTLS(alertSMTPTLSConfig(cfg.AlertSMTPHost)); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if cfg.AlertSMTPUsername != "" {
		if err := client.Auth(smtp.PlainAuth("", cfg.AlertSMTPUsername, cfg.AlertSMTPPassword, cfg.AlertSMTPHost)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(alertEmailBytes(from, to, message)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return client.Quit()
}

func alertEmailBytes(from *mail.Address, to *mail.Address, message AlertEmailMessage) []byte {
	headers := []string{
		"From: " + from.String(),
		"To: " + to.String(),
		"Subject: " + mime.QEncoding.Encode("utf-8", sanitizeAlertMailHeader(message.Subject)),
		"Date: " + time.Now().UTC().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}
	for key, value := range message.Headers {
		headers = append(headers, sanitizeAlertMailHeader(key)+": "+sanitizeAlertMailHeader(value))
	}
	body := strings.ReplaceAll(strings.ReplaceAll(message.Text, "\r\n", "\n"), "\n", "\r\n")
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")
}

func sanitizeAlertMailHeader(value string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}
