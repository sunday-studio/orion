// Package appres measures what applications cost (CPU, memory, disk I/O,
// network, storage) from the outside, with no app integration and no extra
// privilege. See docs/architecture/passive-metrics-support-matrix.md.
package appres

import "time"

// Kind of measured thing.
type Kind string

const (
	KindProcess   Kind = "process"   // processes grouped by executable name
	KindUnit      Kind = "unit"      // systemd unit (cgroup)
	KindContainer Kind = "container" // container (cgroup or Docker API)
	KindPath      Kind = "path"      // configured storage path
)

// Series is one measured thing in one report. Rates are per second over the
// sampling window; fields that are not measurable are omitted, never zero.
type Series struct {
	Kind          Kind     `json:"kind"`
	Key           string   `json:"key"` // stable id, e.g. unit:nginx.service
	Name          string   `json:"name"`
	CPUPercent    *float64 `json:"cpu_percent,omitempty"` // percent of one core
	MemoryBytes   *uint64  `json:"memory_bytes,omitempty"`
	IOReadBps     *float64 `json:"io_read_bytes_per_sec,omitempty"`
	IOWriteBps    *float64 `json:"io_write_bytes_per_sec,omitempty"`
	NetRxBps      *float64 `json:"net_rx_bytes_per_sec,omitempty"`
	NetTxBps      *float64 `json:"net_tx_bytes_per_sec,omitempty"`
	Pids          *int     `json:"pids,omitempty"`
	StorageBytes  *uint64  `json:"storage_bytes,omitempty"`
	StorageCapped bool     `json:"storage_capped,omitempty"` // the walk hit its budget; value is a lower bound
}

// Report is the `app_resources` section of a system report.
type Report struct {
	IntervalSeconds float64  `json:"interval_seconds"`
	Series          []Series `json:"series"`
	// Notes record sources that failed or were skipped. A failing source never
	// fails the report.
	Notes []string `json:"notes,omitempty"`
}

// Options controls collection and bounds cardinality.
type Options struct {
	TopN        int           // processes reported per ranking (CPU, memory); default 10
	MaxSeries   int           // hard cap on series per report; default 200
	Paths       []string      // storage paths to size
	PathBudget  time.Duration // time allowed per path walk; default 2s
	CgroupRoot  string        // default /sys/fs/cgroup
	DockerSock  string        // optional Docker socket for container names and network
	Now         func() time.Time
	ProcessList func() ([]ProcSample, error)           // replaced in tests
	NetSource   func() (map[string]NetCounters, error) // per-process network (macOS), replaced in tests
}

func (o Options) topN() int {
	if o.TopN <= 0 {
		return 10
	}
	return o.TopN
}

func (o Options) maxSeries() int {
	if o.MaxSeries <= 0 {
		return 200
	}
	return o.MaxSeries
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// ProcSample is one process at one instant.
type ProcSample struct {
	Pid        int32
	StartMs    int64 // process start time; guards against pid reuse between samples
	Name       string
	CPUSeconds float64 // cumulative user+system
	RSS        uint64
	ReadBytes  *uint64 // nil when unreadable (other user's process)
	WriteBytes *uint64
}

// NetCounters are cumulative bytes for one name (macOS nettop).
type NetCounters struct{ Rx, Tx uint64 }

func fptr(v float64) *float64 { return &v }
func uptr(v uint64) *uint64   { return &v }
func iptr(v int) *int         { return &v }
