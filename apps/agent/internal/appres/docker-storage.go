package appres

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// dockerStorage is how much disk Docker holds for this host: the writable
// layer of each running container, each volume, and the images and build
// cache. Docker computes these sizes by walking the disk, so they are read at
// most every dockerStorageEvery and reused in between.
type dockerStorage struct {
	Containers map[string]uint64 // short container id -> writable layer bytes
	Volumes    []dockerVolume
	ImageBytes uint64
	CacheBytes uint64
	HasImages  bool
}

type dockerVolume struct {
	Name  string
	Bytes uint64
}

const (
	dockerStorageEvery = 5 * time.Minute
	maxDockerVolumes   = 50
)

type dockerStorageSource func() (dockerStorage, error)

func newDockerStorageSource(sock string, now func() time.Time) dockerStorageSource {
	api := newDockerAPI(sock)
	var (
		mu      sync.Mutex
		at      time.Time // when sizes were last asked for, whether or not it worked
		last    dockerStorage
		lastErr error
	)
	return func() (dockerStorage, error) {
		mu.Lock()
		defer mu.Unlock()
		if !at.IsZero() && now().Sub(at) < dockerStorageEvery {
			return last, lastErr // a struggling Docker is not asked again before the window ends
		}
		if _, err := os.Stat(api.sock); err != nil {
			return dockerStorage{}, errNoDocker
		}
		fail := func(err error) (dockerStorage, error) {
			at, lastErr = now(), err // keep the last good sizes in the meantime
			return last, err
		}
		var list []struct {
			ID     string `json:"Id"`
			SizeRw int64  `json:"SizeRw"`
		}
		if err := api.get("/containers/json?size=true", &list); err != nil {
			if errors.Is(err, fs.ErrPermission) || strings.Contains(err.Error(), "permission denied") {
				return fail(fmt.Errorf("docker socket: permission denied (the agent user is not in the docker group)"))
			}
			return fail(err)
		}
		out := dockerStorage{Containers: map[string]uint64{}}
		for _, c := range list {
			if c.SizeRw >= 0 {
				out.Containers[c.ID[:min(12, len(c.ID))]] = uint64(c.SizeRw)
			}
		}
		var df struct {
			LayersSize int64 `json:"LayersSize"`
			Volumes    []struct {
				Name      string `json:"Name"`
				UsageData *struct {
					Size int64 `json:"Size"`
				} `json:"UsageData"`
			} `json:"Volumes"`
			BuildCache []struct {
				Size int64 `json:"Size"`
			} `json:"BuildCache"`
		}
		if err := api.get("/system/df", &df); err == nil { // volumes are optional: containers alone are still useful
			out.ImageBytes, out.HasImages = uint64(max(df.LayersSize, 0)), true
			for _, b := range df.BuildCache {
				out.CacheBytes += uint64(max(b.Size, 0))
			}
			for _, v := range df.Volumes {
				if v.UsageData != nil && v.UsageData.Size >= 0 {
					out.Volumes = append(out.Volumes, dockerVolume{v.Name, uint64(v.UsageData.Size)})
				}
			}
			sort.Slice(out.Volumes, func(i, j int) bool { return out.Volumes[i].Bytes > out.Volumes[j].Bytes })
			if len(out.Volumes) > maxDockerVolumes {
				out.Volumes = out.Volumes[:maxDockerVolumes]
			}
		}
		at, last, lastErr = now(), out, nil
		return out, nil
	}
}

// applyDockerStorage adds the writable-layer size to container series and the
// volumes, images and build cache as series of their own.
func (s *Sampler) applyDockerStorage(rep *Report) {
	if s.opts.DockerStorage == nil {
		return
	}
	st, err := s.opts.DockerStorage()
	if err != nil {
		if err != errNoDocker && !s.dockerStorageNote {
			rep.Notes = append(rep.Notes, "docker storage: "+err.Error())
			s.dockerStorageNote = true
		}
		return
	}
	s.dockerStorageNote = false
	for i := range rep.Series {
		if rep.Series[i].Kind != KindContainer {
			continue
		}
		short := strings.TrimPrefix(rep.Series[i].Key, "container:")
		if n, ok := st.Containers[short]; ok {
			rep.Series[i].StorageBytes = uptr(n)
		}
	}
	for _, v := range st.Volumes {
		rep.Series = append(rep.Series, Series{Kind: KindVolume, Key: "volume:" + v.Name, Name: v.Name, StorageBytes: uptr(v.Bytes)})
	}
	if st.HasImages {
		rep.Series = append(rep.Series,
			Series{Kind: KindDocker, Key: "docker:images", Name: "docker images", StorageBytes: uptr(st.ImageBytes)},
			Series{Kind: KindDocker, Key: "docker:build-cache", Name: "docker build cache", StorageBytes: uptr(st.CacheBytes)})
	}
}
