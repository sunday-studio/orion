package logstore

import (
	"fmt"
	"regexp"
	"strings"
)

const redacted = "[redacted]"

// defaultRedactedKeys are matched case-insensitively against attribute key names
// (exact name, or the name contains one of the substrings).
var defaultRedactedKeys = []string{
	"password", "passwd", "secret", "token", "authorization", "api_key", "apikey", "api-key",
	"cookie", "set-cookie", "private_key", "privatekey", "credit_card", "creditcard", "card_number", "cvv", "ssn", "access_key",
}

// Redactor replaces the values of secret-looking keys, at any depth, before storage.
type Redactor struct {
	keys     []string
	keyRules []keyRule
	regexes  []regexRule
}

// Rule is a custom redaction rule: a key name or a regular expression over text values.
type Rule struct {
	Type        string `json:"type"` // key | regex
	Pattern     string `json:"pattern"`
	ReplaceWith string `json:"replace_with"`
}

type keyRule struct{ key, replace string }
type regexRule struct {
	re      *regexp.Regexp
	replace string
}

const (
	maxRules       = 20
	maxPatternSize = 200
	maxReplaceSize = 40
)

// DefaultKeys lists the key names redacted unless defaults are disabled.
func DefaultKeys() []string { return append([]string(nil), defaultRedactedKeys...) }

// ValidateRules checks custom rules before they are saved.
func ValidateRules(rules []Rule) error {
	if len(rules) > maxRules {
		return fmt.Errorf("at most %d custom rules", maxRules)
	}
	for i, r := range rules {
		if r.Type != "key" && r.Type != "regex" {
			return fmt.Errorf("rule %d: type must be key or regex", i+1)
		}
		if strings.TrimSpace(r.Pattern) == "" || len(r.Pattern) > maxPatternSize {
			return fmt.Errorf("rule %d: pattern is required (at most %d characters)", i+1, maxPatternSize)
		}
		if len(r.ReplaceWith) > maxReplaceSize {
			return fmt.Errorf("rule %d: replace_with is at most %d characters", i+1, maxReplaceSize)
		}
		if r.Type == "regex" {
			if _, err := regexp.Compile(r.Pattern); err != nil {
				return fmt.Errorf("rule %d: pattern is not a valid regular expression", i+1)
			}
		}
	}
	return nil
}

// NewRedactorRules is NewRedactor plus custom rules. Invalid rules are skipped; save-time validation rejects them.
func NewRedactorRules(extraKeys []string, disableDefaults bool, rules []Rule) *Redactor {
	r := NewRedactor(extraKeys, disableDefaults)
	for _, rule := range rules {
		replace := rule.ReplaceWith
		if replace == "" {
			replace = redacted
		}
		switch rule.Type {
		case "key":
			if key := strings.ToLower(strings.TrimSpace(rule.Pattern)); key != "" {
				r.keyRules = append(r.keyRules, keyRule{key: key, replace: replace})
			}
		case "regex":
			if re, err := regexp.Compile(rule.Pattern); err == nil {
				r.regexes = append(r.regexes, regexRule{re: re, replace: replace})
			}
		}
	}
	return r
}

// Redact applies the rules to a sample (decoded JSON or a string) for the "test your rules" preview.
func (r *Redactor) Redact(sample any) any {
	if text, ok := sample.(string); ok {
		return r.text(text)
	}
	return r.value(sample)
}

func (r *Redactor) text(s string) string {
	for _, rx := range r.regexes {
		s = rx.re.ReplaceAllString(s, rx.replace)
	}
	return s
}

// NewRedactor builds a redactor from the defaults (unless disabled) plus extra key names.
func NewRedactor(extraKeys []string, disableDefaults bool) *Redactor {
	r := &Redactor{}
	if !disableDefaults {
		r.keys = append(r.keys, defaultRedactedKeys...)
	}
	for _, key := range extraKeys {
		if key = strings.ToLower(strings.TrimSpace(key)); key != "" {
			r.keys = append(r.keys, key)
		}
	}
	return r
}

func (r *Redactor) sensitive(key string) (string, bool) {
	key = strings.ToLower(key)
	for _, k := range r.keyRules {
		if key == k.key || strings.Contains(key, k.key) {
			return k.replace, true
		}
	}
	for _, k := range r.keys {
		if key == k || strings.Contains(key, k) {
			return redacted, true
		}
	}
	return "", false
}

// Apply redacts the entry in place: attributes, resource and structured body. The
// plain-text body is not scanned, so apps should keep secrets out of message strings.
func (r *Redactor) Apply(entry *Entry) {
	if entry.BodyJSON == "" {
		entry.Body = r.text(entry.Body)
	}
	entry.Attributes = r.value(entry.Attributes).(map[string]any)
	entry.Resource = r.value(entry.Resource).(map[string]any)
	if entry.BodyJSON != "" {
		if structured, ok := decodeJSON(entry.BodyJSON); ok {
			entry.BodyJSON = encodeJSON(r.value(structured))
			entry.Body = entry.BodyJSON
		}
	}
}

func (r *Redactor) value(v any) any {
	switch value := v.(type) {
	case map[string]any:
		if value == nil {
			return map[string]any(nil)
		}
		out := make(map[string]any, len(value))
		for key, child := range value {
			if replacement, hit := r.sensitive(key); hit {
				out[key] = replacement
			} else {
				out[key] = r.value(child)
			}
		}
		return out
	case []any:
		out := make([]any, len(value))
		for i, child := range value {
			out[i] = r.value(child)
		}
		return out
	case string:
		return r.text(value)
	default:
		return v
	}
}
