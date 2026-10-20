package config

import "testing"

func TestExcerptsNeverShowCredentialValues(t *testing.T) {
	for in, want := range map[string]string{
		"enrollment_token: orn_enr_abc":      "enrollment_token: <hidden>",
		"  ingest_key_file: /run/key":        "  ingest_key_file: /run/key",
		"password: hunter2":                  "password: <hidden>",
		"interval: 30s":                      "interval: 30s",
		"  intervall: 30s":                   "  intervall: 30s",
		"core_url: https://core.example.com": "core_url: https://core.example.com",
	} {
		if got := redactLine(in); got != want {
			t.Errorf("redactLine(%q) = %q, want %q", in, got, want)
		}
	}
}
