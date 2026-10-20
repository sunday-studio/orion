package appres

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

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
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return func() ([]dockerContainer, error) {
		if _, err := os.Stat(sock); err != nil {
			return nil, fmt.Errorf("docker socket %s not found", sock)
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
		if len(list) > maxDockerContainers {
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
			}(i, c.ID)
		}
		wg.Wait()
		return out, nil
	}
}
