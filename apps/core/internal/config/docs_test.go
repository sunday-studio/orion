package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryEnvironmentVariableCoreReadsIsDocumented keeps docs/deployment/core-environment.md complete:
// a variable added to the code without a row in that page fails here.
func TestEveryEnvironmentVariableCoreReadsIsDocumented(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "docs", "deployment", "core-environment.md"))
	if err != nil {
		t.Fatalf("read the environment reference: %v", err)
	}
	pattern := regexp.MustCompile(`"(ORION_[A-Z0-9_]+[A-Z0-9])"`)
	found := map[string]string{}
	for _, root := range []string{"..", filepath.Join("..", "..", "cmd")} {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, m := range pattern.FindAllStringSubmatch(string(src), -1) {
				found[m[1]] = path
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(found) < 40 {
		t.Fatalf("found only %d ORION_ variables in the source; the scan is broken", len(found))
	}
	for name, path := range found {
		if !strings.Contains(string(doc), "`"+name+"`") {
			t.Errorf("%s (read in %s) is not documented in docs/deployment/core-environment.md", name, path)
		}
	}
}
