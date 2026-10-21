package cli

import (
	"errors"
	"net/url"
	"os"
	"strings"

	"orion/node/internal/config"
)

func stageEnrollmentToken(token string) (string, error) {
	f, err := os.CreateTemp("", "orion-enroll-*")
	if err != nil {
		return "", NewCommandError("could not stage the token", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", NewCommandError("could not protect the staged token", err)
	}
	if _, err := f.WriteString(strings.TrimSpace(token)); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", NewCommandError("could not stage the token", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", NewCommandError("could not stage the token", err)
	}
	return f.Name(), nil
}

// pickableCandidates drops items that cannot become monitors and low-confidence noise.
func validateCoreURL(s string) error {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("enter a full URL such as https://orion.example.com")
	}
	return nil
}

func envOrEmpty(name string) string { v, _ := config.Env(name); return v }
