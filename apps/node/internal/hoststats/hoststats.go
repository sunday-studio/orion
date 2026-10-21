// Package hoststats keeps a fresh snapshot of host load (CPU, memory, disk,
// load average, network rates) for `status`. It samples on its own short
// interval so a status call is instant and shows live values, independent of
// the slower reporting interval.
package hoststats

import (
	"context"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// Snapshot is the latest host reading. Rates are zero until two samples exist.
type Snapshot struct {
	At            time.Time
	CPUPercent    float64
	MemoryPercent float64
	DiskPath      string
	DiskPercent   float64
	Load1         float64
	NetRxBps      float64
	NetTxBps      float64
	Ready         bool // false until the first full sample
}

// Reading is one raw sample; the source is replaced in tests.
type Reading struct {
	CPUPercent, MemoryPercent, DiskPercent, Load1 float64
	NetRx, NetTx                                  uint64 // cumulative bytes
}

// Source reads the host once.
type Source func() (Reading, error)

// HostSource samples the real machine.
func HostSource(diskPath string) Source {
	return func() (Reading, error) {
		var r Reading
		if p, err := cpu.Percent(0, false); err == nil && len(p) > 0 {
			r.CPUPercent = p[0]
		}
		if m, err := mem.VirtualMemory(); err == nil {
			r.MemoryPercent = m.UsedPercent
		}
		if d, err := disk.Usage(diskPath); err == nil {
			r.DiskPercent = d.UsedPercent
		}
		if l, err := load.Avg(); err == nil {
			r.Load1 = l.Load1
		}
		if n, err := net.IOCounters(false); err == nil && len(n) > 0 {
			r.NetRx, r.NetTx = n[0].BytesRecv, n[0].BytesSent
		}
		return r, nil
	}
}

// Sampler holds the latest snapshot.
type Sampler struct {
	src      Source
	diskPath string
	now      func() time.Time

	mu   sync.Mutex
	snap Snapshot
	prev *Reading
	at   time.Time
}

// New returns a sampler for diskPath ("/" if empty).
func New(src Source, diskPath string) *Sampler {
	if diskPath == "" {
		diskPath = "/"
	}
	return &Sampler{src: src, diskPath: diskPath, now: time.Now}
}

// Sample reads once and updates the snapshot.
func (s *Sampler) Sample() {
	r, err := s.src()
	if err != nil {
		return
	}
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := Snapshot{At: now, CPUPercent: r.CPUPercent, MemoryPercent: r.MemoryPercent, DiskPath: s.diskPath, DiskPercent: r.DiskPercent, Load1: r.Load1, Ready: s.prev != nil}
	if s.prev != nil {
		if dt := now.Sub(s.at).Seconds(); dt > 0 {
			if r.NetRx >= s.prev.NetRx {
				snap.NetRxBps = float64(r.NetRx-s.prev.NetRx) / dt
			}
			if r.NetTx >= s.prev.NetTx {
				snap.NetTxBps = float64(r.NetTx-s.prev.NetTx) / dt
			}
		}
	}
	s.prev, s.at, s.snap = &r, now, snap
}

// Snapshot returns the latest reading.
func (s *Sampler) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap
}

// Run samples every interval until ctx ends.
func (s *Sampler) Run(ctx context.Context, every time.Duration) {
	s.Sample()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Sample()
		}
	}
}
