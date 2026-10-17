package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"orion/core/internal/config"
	"strconv"
	"strings"
	"time"
)

const (
	AlertEmailPurposeConfirm     = "confirm"
	AlertEmailPurposeUnsubscribe = "unsubscribe"
	alertEmailConfirmTTL         = 72 * time.Hour
)

var ErrAlertEmailToken = errors.New("invalid or expired link")

func alertEmailSecret(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.AlertEmailSecret != "" {
		return cfg.AlertEmailSecret
	}
	return cfg.JWTSecret
}

func alertEmailSignature(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte("orion-alert-email|"+secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SignAlertEmailToken makes an HMAC-signed link token. ttl <= 0 means no expiry
// (unsubscribe links must keep working in old mail).
func SignAlertEmailToken(cfg *config.Config, purpose, recipientID string, ttl time.Duration, now time.Time) string {
	var exp int64
	if ttl > 0 {
		exp = now.Add(ttl).Unix()
	}
	payload := purpose + "." + recipientID + "." + strconv.FormatInt(exp, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + alertEmailSignature(alertEmailSecret(cfg), payload)
}

// VerifyAlertEmailToken returns the recipient id when the token is authentic,
// matches the purpose and has not expired.
func VerifyAlertEmailToken(cfg *config.Config, purpose, token string, now time.Time) (string, error) {
	encoded, signature, ok := strings.Cut(token, ".")
	if !ok || alertEmailSecret(cfg) == "" {
		return "", ErrAlertEmailToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrAlertEmailToken
	}
	payload := string(raw)
	if !hmac.Equal([]byte(alertEmailSignature(alertEmailSecret(cfg), payload)), []byte(signature)) {
		return "", ErrAlertEmailToken
	}
	parts := strings.Split(payload, ".")
	if len(parts) != 3 || parts[0] != purpose {
		return "", ErrAlertEmailToken
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || (exp != 0 && now.Unix() > exp) {
		return "", ErrAlertEmailToken
	}
	return parts[1], nil
}
