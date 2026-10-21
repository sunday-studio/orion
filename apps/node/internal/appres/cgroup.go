package appres

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// cgroupSample is the cumulative state of one cgroup.
type cgroupSample struct {
	Kind    Kind
	Key     string
	Name    string
	CPUUsec uint64
	Memory  uint64
	Read    uint64
	Write   uint64
	Pids    int
	HasIO   bool
	HasCPU  bool
	HasMem  bool
	HasPids bool
}

// readCgroups lists systemd units and container scopes under a cgroup v2
// root. Nothing here needs privilege: these files are world-readable.
func readCgroups(root string) ([]cgroupSample, []string) {
	var out []cgroupSample
	var notes []string
	if _, err := os.Stat(filepath.Join(root, "cgroup.controllers")); err != nil {
		return nil, []string{"cgroup v2 not found at " + root + " (unified hierarchy required for unit and container metrics)"}
	}
	for _, slice := range []string{"system.slice", "docker", "machine.slice"} {
		entries, err := os.ReadDir(filepath.Join(root, slice))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			kind, key, display := classify(slice, name)
			if kind == "" {
				continue
			}
			s := readOneCgroup(filepath.Join(root, slice, name))
			s.Kind, s.Key, s.Name = kind, key, display
			out = append(out, s)
		}
	}
	return out, notes
}

// classify maps a cgroup directory to a series identity, or "" to skip it.
func classify(slice, dir string) (Kind, string, string) {
	switch {
	case slice == "system.slice" && strings.HasSuffix(dir, ".service"):
		return KindUnit, "unit:" + dir, dir
	case slice == "system.slice" && strings.HasPrefix(dir, "docker-") && strings.HasSuffix(dir, ".scope"):
		id := strings.TrimSuffix(strings.TrimPrefix(dir, "docker-"), ".scope")
		return KindContainer, "container:" + short(id), short(id)
	case slice == "docker" && len(dir) >= 12:
		return KindContainer, "container:" + short(dir), short(dir)
	}
	return "", "", ""
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func readOneCgroup(dir string) cgroupSample {
	var s cgroupSample
	if b, err := os.ReadFile(filepath.Join(dir, "cpu.stat")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if f := strings.Fields(line); len(f) == 2 && f[0] == "usage_usec" {
				s.CPUUsec, _ = strconv.ParseUint(f[1], 10, 64)
				s.HasCPU = true
			}
		}
	}
	if v, ok := readUint(filepath.Join(dir, "memory.current")); ok {
		s.Memory, s.HasMem = v, true
	}
	if v, ok := readUint(filepath.Join(dir, "pids.current")); ok {
		s.Pids, s.HasPids = int(v), true
	}
	if b, err := os.ReadFile(filepath.Join(dir, "io.stat")); err == nil {
		for _, line := range strings.Split(string(b), "\n") { // one line per device: "254:16 rbytes=.. wbytes=.."
			for _, field := range strings.Fields(line) {
				k, v, ok := strings.Cut(field, "=")
				if !ok {
					continue
				}
				n, _ := strconv.ParseUint(v, 10, 64)
				switch k {
				case "rbytes":
					s.Read += n
					s.HasIO = true
				case "wbytes":
					s.Write += n
					s.HasIO = true
				}
			}
		}
	}
	return s
}

func readUint(path string) (uint64, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return n, err == nil
}
