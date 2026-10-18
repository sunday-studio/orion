package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Environment variables that override or replace the config file. Every
// *_FILE variant reads the value from a file so secrets stay out of the
// process environment.
const (
	EnvCoreURL    = "ORION_CORE_URL"
	EnvInterval   = "ORION_INTERVAL"
	EnvLogLevel   = "ORION_LOG_LEVEL"
	EnvLogPath    = "ORION_LOG_PATH"
	EnvEnrollment = "ORION_ENROLLMENT_TOKEN"

	// DefaultInterval is used when no interval is configured.
	DefaultInterval = "60s"
)

var getenv = os.Getenv

// Source records where the effective config came from, for `config show`.
type Source struct {
	File      string   `json:"file,omitempty"`
	FileFound bool     `json:"file_found"`
	DropIns   []string `json:"drop_ins,omitempty"`
	EnvKeys   []string `json:"env_overrides,omitempty"`
	ZeroConf  bool     `json:"zero_config"`
}

// DropInDir returns the config.d directory next to the main config file.
func DropInDir(path string) string {
	return filepath.Join(filepath.Dir(path), "config.d")
}

// Env returns an environment value, reading ORION_X_FILE when ORION_X is unset.
func Env(name string) (string, error) {
	if v := strings.TrimSpace(getenv(name)); v != "" {
		return v, nil
	}
	if p := strings.TrimSpace(getenv(name + "_FILE")); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			return "", fmt.Errorf("%s_FILE: %w", name, err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", nil
}

// Load builds the effective configuration: base file, then config.d drop-ins
// in lexical order, then environment overrides. A missing base file is fine
// when ORION_CORE_URL is set (zero-config).
func Load(path string) (*UserConfig, Source, error) { return load(path, true) }

// LoadRaw is Load without filling defaults, for tools that write the config
// back and must not pin default values (such as log paths) into the file.
func LoadRaw(path string) (*UserConfig, Source, error) { return load(path, false) }

func load(path string, defaults bool) (*UserConfig, Source, error) {
	src := Source{File: path}
	cfg := UserConfig{}
	var lines []monitorLine

	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		src.FileFound = true
		part, ml, perr := decodeStrict(data, path)
		if perr != nil {
			return nil, src, perr
		}
		cfg = *part
		lines = append(lines, ml...)
	case errors.Is(err, os.ErrNotExist):
		if v, _ := Env(EnvCoreURL); v == "" {
			return nil, src, fmt.Errorf("config file %s not found; run `orion-agent setup`, or set %s to run without a file", path, EnvCoreURL)
		}
		src.ZeroConf = true
	default:
		return nil, src, fmt.Errorf("read config %s: %w", path, err)
	}

	dropIns, _ := filepath.Glob(filepath.Join(DropInDir(path), "*.yaml"))
	more, _ := filepath.Glob(filepath.Join(DropInDir(path), "*.yml"))
	dropIns = append(dropIns, more...)
	sort.Strings(dropIns)
	for _, p := range dropIns {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, src, fmt.Errorf("read drop-in %s: %w", p, rerr)
		}
		part, ml, perr := decodeStrict(raw, p)
		if perr != nil {
			return nil, src, perr
		}
		lines = mergeInto(&cfg, part, lines, ml)
		src.DropIns = append(src.DropIns, p)
	}

	envKeys, err := applyEnv(&cfg)
	if err != nil {
		return nil, src, err
	}
	src.EnvKeys = envKeys
	checked := cfg
	if checked.Interval == "" {
		checked.Interval = DefaultInterval
	}
	checked.ApplyDefaults()
	if err := checked.Validate(); err != nil {
		return nil, src, locate(err, lines, path)
	}
	if defaults {
		return &checked, src, nil
	}
	return &cfg, src, nil
}

// mergeInto layers part over base: set scalars win, monitors merge by name.
func mergeInto(base, part *UserConfig, lines, partLines []monitorLine) []monitorLine {
	if part.CoreURL != "" {
		base.CoreURL = part.CoreURL
	}
	if part.Interval != "" {
		base.Interval = part.Interval
	}
	if part.GeoLocation {
		base.GeoLocation = true
	}
	for k, v := range part.Meta {
		if base.Meta == nil {
			base.Meta = map[string]interface{}{}
		}
		base.Meta[k] = v
	}
	base.Logging = mergeLogging(base.Logging, part.Logging)
	for i, m := range part.Monitors {
		idx := -1
		for j := range base.Monitors {
			if base.Monitors[j].Name == m.Name {
				idx = j
			}
		}
		if idx >= 0 {
			base.Monitors[idx] = m
			lines[idx] = partLines[i]
			continue
		}
		base.Monitors = append(base.Monitors, m)
		lines = append(lines, partLines[i])
	}
	return lines
}

func mergeLogging(a, b LoggingConfig) LoggingConfig {
	if b.Level != "" {
		a.Level = b.Level
	}
	if b.Path != "" {
		a.Path = b.Path
	}
	if b.Format != "" {
		a.Format = b.Format
	}
	if b.MaxSizeMB != 0 {
		a.MaxSizeMB = b.MaxSizeMB
	}
	if b.MaxBackups != 0 {
		a.MaxBackups = b.MaxBackups
	}
	if b.MaxAgeDays != 0 {
		a.MaxAgeDays = b.MaxAgeDays
	}
	if b.Compress != nil {
		a.Compress = b.Compress
	}
	return a
}

func applyEnv(cfg *UserConfig) ([]string, error) {
	var keys []string
	set := func(env string, dst *string) error {
		v, err := Env(env)
		if err != nil {
			return err
		}
		if v != "" {
			*dst = v
			keys = append(keys, env)
		}
		return nil
	}
	for _, e := range []struct {
		name string
		dst  *string
	}{
		{EnvCoreURL, &cfg.CoreURL},
		{EnvInterval, &cfg.Interval},
		{EnvLogLevel, &cfg.Logging.Level},
		{EnvLogPath, &cfg.Logging.Path},
	} {
		if err := set(e.name, e.dst); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

type monitorLine struct {
	file string
	line int
}

// decodeStrict parses YAML, rejecting unknown keys with a file:line message
// and a suggestion when a key looks like a typo.
func decodeStrict(data []byte, name string) (*UserConfig, []monitorLine, error) {
	var cfg UserConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, nil, friendlyYAMLError(err, name)
	}
	lines := make([]monitorLine, len(cfg.Monitors))
	var root yaml.Node
	if yaml.Unmarshal(data, &root) == nil && len(root.Content) > 0 {
		for i, ln := range sequenceLines(root.Content[0], "monitors") {
			if i < len(lines) {
				lines[i] = monitorLine{file: name, line: ln}
			}
		}
	}
	return &cfg, lines, nil
}

func sequenceLines(doc *yaml.Node, key string) []int {
	if doc.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(doc.Content); i += 2 {
		if doc.Content[i].Value == key && doc.Content[i+1].Kind == yaml.SequenceNode {
			var out []int
			for _, item := range doc.Content[i+1].Content {
				out = append(out, item.Line)
			}
			return out
		}
	}
	return nil
}

var (
	unknownFieldRe = regexp.MustCompile(`line (\d+): field (\S+) not found in type`)
	lineRe         = regexp.MustCompile(`line (\d+): (.*)`)
)

func friendlyYAMLError(err error, name string) error {
	msg := strings.TrimPrefix(err.Error(), "yaml: unmarshal errors:\n")
	msg = strings.TrimPrefix(msg, "yaml: ")
	var out []string
	for _, l := range strings.Split(msg, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if m := unknownFieldRe.FindStringSubmatch(l); m != nil {
			if len(out) == 0 {
				line, _ := strconv.Atoi(m[1])
				src := sourceLine(name, line)
				return &Diagnostic{File: name, Line: line, Col: keyColumn(src, m[2]), Key: m[2], Message: "unknown key", Suggest: closest(m[2]), Source: src}
			}
			out = append(out, fmt.Sprintf("%s:%s: unknown key %q", name, m[1], m[2]))
			continue
		}
		if m := lineRe.FindStringSubmatch(l); m != nil {
			out = append(out, fmt.Sprintf("%s:%s: %s", name, m[1], m[2]))
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", name, l))
	}
	return errors.New(strings.Join(out, "\n"))
}

// locate prefixes a validation error with the file and line of the monitor it
// concerns, and adds a fix hint for common mistakes.
func locate(err error, lines []monitorLine, mainFile string) error {
	msg := err.Error()
	if m := regexp.MustCompile(`monitor\[(\d+)\]`).FindStringSubmatch(msg); m != nil {
		if i, _ := strconv.Atoi(m[1]); i < len(lines) && lines[i].line > 0 {
			h := strings.TrimSpace(hint(msg))
			return &Diagnostic{File: lines[i].file, Line: lines[i].line, Message: msg, Source: sourceLine(lines[i].file, lines[i].line), Hint: h}
		}
	}
	return fmt.Errorf("%s: %s%s", mainFile, msg, hint(msg))
}

func hint(msg string) string {
	switch {
	case strings.Contains(msg, "timeout is required"):
		return "\n  fix: add `timeout: 5s` to that monitor"
	case strings.Contains(msg, "core_url"):
		return "\n  fix: set core_url to your Core address, for example https://orion.example.com (or export " + EnvCoreURL + ")"
	case strings.Contains(msg, "duplicate name"):
		return "\n  fix: monitor names must be unique across config.yaml and config.d/"
	case strings.Contains(msg, "interval"):
		return "\n  fix: use a Go duration such as 30s or 1m"
	}
	return ""
}

// knownKeys lists every YAML key the config accepts, for typo suggestions.
func knownKeys() []string {
	seen := map[string]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < t.NumField(); i++ {
			tag := strings.Split(t.Field(i).Tag.Get("yaml"), ",")[0]
			if tag != "" && tag != "-" {
				seen[tag] = true
			}
			walk(t.Field(i).Type)
		}
	}
	walk(reflect.TypeOf(UserConfig{}))
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func closest(word string) string {
	best, bestD := "", 3
	for _, k := range knownKeys() {
		if d := editDistance(word, k); d < bestD {
			best, bestD = k, d
		}
	}
	return best
}

func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}
