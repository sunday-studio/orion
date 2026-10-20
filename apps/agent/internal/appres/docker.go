package appres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// errNoDocker means there is no Docker socket on this host; that is normal and silent.
var errNoDocker = errors.New("no docker socket")

// DefaultDockerSocket is where the Docker Engine API listens on Linux.
const DefaultDockerSocket = "/var/run/docker.sock"

// maxDockerContainers bounds API calls per sample.
const maxDockerContainers = 50

// dockerContainer is what the Docker Engine API tells us about one running
// container: its name (cgroups only carry the id) and cumulative counters.
type dockerContainer struct {
	ID     string
	Name   string
	NetRx  uint64
	NetTx  uint64
	Memory uint64 // bytes in use, from the API (used where there are no cgroups)
	HasNet bool
}

// dockerSource reads container names and network counters from the Docker
// Engine API. It is opt-in by permission: the agent only gets data when its
// user can open the socket (group docker, which is root-equivalent).
type dockerSource func() ([]dockerContainer, error)

func newDockerSource(sock string) dockerSource {
	if sock == "" {
		sock = DefaultDockerSocket
		if _, err := os.Stat(sock); err != nil { // Docker Desktop on macOS keeps it in the home directory
			if home, herr := os.UserHomeDir(); herr == nil {
				if alt := home + "/.docker/run/docker.sock"; fileExists(alt) {
					sock = alt
				}
			}
		}
	}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		}},
	}
	get := func(path string, out any) error {
		resp, err := client.Get("http://docker" + path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("docker API %s: HTTP %d", path, resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
	}
	return func() ([]dockerContainer, error) {
		if _, err := os.Stat(sock); err != nil {
			return nil, errNoDocker
		}
		var list []struct {
			ID    string   `json:"Id"`
			Names []string `json:"Names"`
		}
		if err := get("/containers/json", &list); err != nil {
			if os.IsPermission(err) || strings.Contains(err.Error(), "permission denied") {
				return nil, fmt.Errorf("docker socket: permission denied (the agent user is not in the docker group)")
			}
			return nil, err
		}
		truncated := len(list) > maxDockerContainers
		if truncated { // a stable subset: sort by id so which 50 are kept does not shuffle
			sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
			list = list[:maxDockerContainers]
		}
		out := make([]dockerContainer, len(list))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, c := range list {
			name := c.ID[:min(12, len(c.ID))]
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			out[i] = dockerContainer{ID: c.ID, Name: name}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, id string) {
				defer wg.Done()
				defer func() { <-sem }()
				var st struct {
					Networks map[string]struct {
						RxBytes uint64 `json:"rx_bytes"`
						TxBytes uint64 `json:"tx_bytes"`
					} `json:"networks"`
					Memory struct {
						Usage uint64 `json:"usage"`
						Stats struct {
							InactiveFile      uint64 `json:"inactive_file"`
							TotalInactiveFile uint64 `json:"total_inactive_file"`
						} `json:"stats"`
					} `json:"memory_stats"`
				}
				if err := get("/containers/"+id+"/stats?stream=false&one-shot=true", &st); err != nil {
					return
				}
				for _, n := range st.Networks {
					out[i].NetRx += n.RxBytes
					out[i].NetTx += n.TxBytes
					out[i].HasNet = true
				}
				out[i].Memory = st.Memory.Usage
				if cache := max(st.Memory.Stats.InactiveFile, st.Memory.Stats.TotalInactiveFile); cache < out[i].Memory {
					out[i].Memory -= cache // page cache is not the app's memory
				}
			}(i, c.ID)
		}
		wg.Wait()
		if truncated {
			return out, fmt.Errorf("%w: more than %d containers; showing the first %d by id", errPartial, maxDockerContainers, maxDockerContainers)
		}
		return out, nil
	}
}

// errPartial marks a usable but incomplete container list; it becomes a note.
var errPartial = errors.New("partial container list")

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
