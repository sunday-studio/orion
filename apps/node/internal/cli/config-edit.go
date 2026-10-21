package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// editConfigYAML edits the MAIN config file in place through mutate and
// nothing else: comments, key order and everything the person wrote stay as
// they were, and drop-ins and environment values are never merged into the
// file. mutate reports whether it changed anything.
func editConfigYAML(configPath string, mutate func(root *yaml.Node) bool) (bool, error) {
	path := configPath
	if resolved, err := filepath.EvalSymlinks(configPath); err == nil {
		path = resolved // edit the real file, keep a managed symlink a symlink
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	var doc yaml.Node
	if len(bytes.TrimSpace(data)) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return false, fmt.Errorf("%s is not valid YAML: %w", path, err)
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return false, fmt.Errorf("%s: the top level must be a mapping", path)
	}
	if !mutate(root) {
		return false, nil
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(detectIndent(data))
	if err := enc.Encode(&doc); err != nil {
		return false, err
	}
	enc.Close()
	return true, writeFilePreserving(path, out.Bytes())
}

// editDiscoveryIgnore adds or removes one id in discovery.ignore. It reports
// whether anything changed.
func editDiscoveryIgnore(configPath, id string, add bool) (bool, error) {
	return editConfigYAML(configPath, func(root *yaml.Node) bool {
		disc := mapChild(root, "discovery", yaml.MappingNode, add)
		if disc == nil {
			return false
		}
		ignore := mapChild(disc, "ignore", yaml.SequenceNode, add)
		if ignore == nil {
			return false
		}
		idx := -1
		for i, n := range ignore.Content {
			if n.Value == id {
				idx = i
			}
		}
		switch {
		case add && idx >= 0, !add && idx < 0:
			return false
		case add:
			ignore.Content = append(ignore.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id})
		default:
			ignore.Content = append(ignore.Content[:idx], ignore.Content[idx+1:]...)
		}
		return true
	})
}

// editDiscoveryEnabled sets discovery.enabled.
func editDiscoveryEnabled(configPath string, on bool) (bool, error) {
	return editConfigYAML(configPath, func(root *yaml.Node) bool {
		disc := mapChild(root, "discovery", yaml.MappingNode, true)
		val := mapScalar(disc, "enabled")
		want := fmt.Sprint(on)
		if val.Value == want {
			return false
		}
		val.Kind, val.Tag, val.Value = yaml.ScalarNode, "!!bool", want
		return true
	})
}

// editDiscoveryKind turns one scanned kind on or off in discovery.kinds. An
// empty list means "all", so the first change writes the full list.
func editDiscoveryKind(configPath, kind string, on bool, all []string) (bool, error) {
	return editConfigYAML(configPath, func(root *yaml.Node) bool {
		disc := mapChild(root, "discovery", yaml.MappingNode, true)
		kinds := mapChild(disc, "kinds", yaml.SequenceNode, true)
		if len(kinds.Content) == 0 {
			for _, k := range all {
				kinds.Content = append(kinds.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k})
			}
		}
		idx := -1
		for i, n := range kinds.Content {
			if n.Value == kind {
				idx = i
			}
		}
		switch {
		case on && idx >= 0, !on && idx < 0:
			return false
		case on:
			kinds.Content = append(kinds.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: kind})
		default:
			kinds.Content = append(kinds.Content[:idx], kinds.Content[idx+1:]...)
		}
		return true
	})
}

// mapScalar returns the scalar value node of key in a mapping, adding it.
func mapScalar(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	val := &yaml.Node{Kind: yaml.ScalarNode}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, val)
	return val
}

// mapChild returns the child of a mapping by key, creating it when create is set.
func mapChild(m *yaml.Node, key string, kind yaml.Kind, create bool) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			if m.Content[i+1].Kind != kind { // e.g. "ignore: null"
				m.Content[i+1] = &yaml.Node{Kind: kind}
			}
			return m.Content[i+1]
		}
	}
	if !create {
		return nil
	}
	child := &yaml.Node{Kind: kind}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
	return child
}

// detectIndent follows the file's own indentation (2 or 4), defaulting to 2.
func detectIndent(data []byte) int {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == line {
			continue
		}
		if n := len(line) - len(trimmed); n == 4 {
			return 4
		} else if n >= 1 {
			return 2
		}
	}
	return 2
}

// writeFilePreserving replaces path atomically, keeping the original mode and
// owner (a new file gets 0640 and is handed to the service user).
func writeFilePreserving(path string, data []byte) error {
	mode := os.FileMode(0o640)
	uid, gid, hadOwner := -1, -1, false
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
		if sys, ok := st.Sys().(*syscall.Stat_t); ok {
			uid, gid, hadOwner = int(sys.Uid), int(sys.Gid), true
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if hadOwner && os.Geteuid() == 0 {
		_ = tmp.Chown(uid, gid)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if !hadOwner {
		handOverFileToServiceUser(path)
	}
	return nil
}
