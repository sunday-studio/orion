package discover

import (
	"fmt"
	"strings"

	"orion/agent/internal/config"

	"github.com/shirou/gopsutil/v4/disk"
)

// Mount is one real filesystem.
type Mount struct {
	Path        string
	Device      string
	FSType      string
	UsedPercent float64
	TotalBytes  uint64
}

// realFS are filesystems that hold data worth watching; pseudo and
// in-memory filesystems are not.
var realFS = map[string]bool{"ext4": true, "ext3": true, "xfs": true, "btrfs": true, "zfs": true, "apfs": true, "hfs": true, "f2fs": true, "ntfs": true, "vfat": false}

func hostMounts() ([]Mount, error) {
	parts, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var out []Mount
	for _, p := range parts {
		if !realFS[strings.ToLower(p.Fstype)] {
			continue
		}
		u, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}
		out = append(out, Mount{Path: p.Mountpoint, Device: p.Device, FSType: p.Fstype, UsedPercent: u.UsedPercent, TotalBytes: u.Total})
	}
	return out, nil
}

// skipMount hides mounts that are never a place to alert on (boot partitions,
// snaps, macOS system volumes, container overlays).
func skipMount(m Mount) bool {
	for _, p := range []string{"/boot", "/snap", "/System/Volumes", "/var/lib/docker", "/var/lib/containers", "/run", "/Library/Developer"} {
		if m.Path == p || strings.HasPrefix(m.Path, p+"/") {
			return true
		}
	}
	return false
}

// diskMounts proposes "alert above 90% full" for each real filesystem.
func diskMounts(e Env) ([]Candidate, error) {
	if e.Mounts == nil {
		return nil, nil
	}
	mounts, err := e.Mounts()
	if err != nil {
		return nil, fmt.Errorf("listing filesystems: %w", err)
	}
	seen := map[string]bool{}
	var out []Candidate
	for _, m := range mounts {
		if skipMount(m) || seen[m.Device] {
			continue
		}
		seen[m.Device] = true
		conf := High
		if m.TotalBytes < 1<<30 {
			conf = Medium // a tiny filesystem is rarely what anyone wants alerts on
		}
		path := m.Path
		out = append(out, Candidate{
			ID: "disk:" + path, Kind: KindDisk, Name: path, Confidence: conf,
			Detail: fmt.Sprintf("%.0f%% used", m.UsedPercent),
			Monitor: newMonitor(e, "disk-"+diskSlug(path), config.UserMonitorTypeDiskUsage, func(mon *config.UserMonitor) {
				mon.Description = "disk usage of " + path
				mon.DiskUsage = &config.DiskUsageConfig{Path: path, MaxPercent: 90}
			}),
		})
	}
	return out, nil
}

func diskSlug(p string) string {
	if p == "/" {
		return "root"
	}
	return strings.ReplaceAll(strings.Trim(p, "/"), "/", "-")
}
