package logstore

import "strings"

const redacted = "[redacted]"

// defaultRedactedKeys are matched case-insensitively against attribute key names
// (exact name, or the name contains one of the substrings).
var defaultRedactedKeys = []string{
	"password", "passwd", "secret", "token", "authorization", "api_key", "apikey", "api-key",
	"cookie", "set-cookie", "private_key", "privatekey", "credit_card", "creditcard", "card_number", "cvv", "ssn", "access_key",
}

// Redactor replaces the values of secret-looking keys, at any depth, before storage.
type Redactor struct {
	keys []string
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

func (r *Redactor) sensitive(key string) bool {
	key = strings.ToLower(key)
	for _, k := range r.keys {
		if key == k || strings.Contains(key, k) {
			return true
		}
	}
	return false
}

// Apply redacts the entry in place: attributes, resource and structured body. The
// plain-text body is not scanned, so apps should keep secrets out of message strings.
func (r *Redactor) Apply(entry *Entry) {
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
			if r.sensitive(key) {
				out[key] = redacted
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
	default:
		return v
	}
}
