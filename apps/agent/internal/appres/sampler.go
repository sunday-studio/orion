package appres

import (
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

// Sampler turns cumulative counters into rates. Call Sample once per report;
// the first call returns instantaneous values only (no rates).
type Sampler struct {
	opts       Options
	prevAt     time.Time
	prevCgroup map[string]cgroupSample
	prevProc   map[int32]ProcSample
	prevNet    map[string]NetCounters
}

// NewSampler returns a sampler with caps applied.
func NewSampler(opts Options) *Sampler {
	if opts.CgroupRoot == "" && runtime.GOOS == "linux" {
		opts.CgroupRoot = "/sys/fs/cgroup"
	}
	if opts.PathBudget <= 0 {
		opts.PathBudget = 2 * time.Second
	}
	if opts.ProcessList == nil {
		opts.ProcessList = hostProcesses
	}
	if opts.NetSource == nil {
		opts.NetSource = hostNetwork
	}
	return &Sampler{opts: opts}
}

// Sample reads all sources. A failing source adds a note and nothing else.
func (s *Sampler) Sample() Report {
	now := s.opts.now()
	elapsed := now.Sub(s.prevAt).Seconds()
	haveRates := !s.prevAt.IsZero() && elapsed > 0
	rep := Report{IntervalSeconds: elapsed}
	if !haveRates {
		rep.IntervalSeconds = 0
	}

	// cgroups: units and containers (Linux)
	var cg []cgroupSample
	if s.opts.CgroupRoot != "" { // empty on non-Linux: no cgroups exist, so no note either
		var notes []string
		cg, notes = readCgroups(s.opts.CgroupRoot)
		rep.Notes = append(rep.Notes, notes...)
	}
	curCg := map[string]cgroupSample{}
	for _, c := range cg {
		curCg[c.Key] = c
		ser := Series{Kind: c.Kind, Key: c.Key, Name: c.Name}
		if c.HasMem {
			ser.MemoryBytes = uptr(c.Memory)
		}
		if c.HasPids {
			ser.Pids = iptr(c.Pids)
		}
		if prev, ok := s.prevCgroup[c.Key]; ok && haveRates {
			if c.HasCPU && prev.HasCPU && c.CPUUsec >= prev.CPUUsec {
				ser.CPUPercent = fptr(float64(c.CPUUsec-prev.CPUUsec) / 1e6 / elapsed * 100)
			}
			if c.HasIO && prev.HasIO && c.Read >= prev.Read && c.Write >= prev.Write {
				ser.IOReadBps = fptr(float64(c.Read-prev.Read) / elapsed)
				ser.IOWriteBps = fptr(float64(c.Write-prev.Write) / elapsed)
			}
		}
		rep.Series = append(rep.Series, ser)
	}

	// processes, grouped by executable name
	procs, err := s.opts.ProcessList()
	if err != nil {
		rep.Notes = append(rep.Notes, "processes: "+err.Error())
	}
	nets, nerr := s.opts.NetSource()
	if nerr != nil {
		rep.Notes = append(rep.Notes, "process network: "+nerr.Error())
	}
	curProc := map[int32]ProcSample{}
	groups := map[string]*procGroup{}
	for _, p := range procs {
		curProc[p.Pid] = p
		g := groups[p.Name]
		if g == nil {
			g = &procGroup{name: p.Name}
			groups[p.Name] = g
		}
		g.pids++
		g.rss += p.RSS
		if prev, ok := s.prevProc[p.Pid]; ok && haveRates && prev.StartMs == p.StartMs && p.CPUSeconds >= prev.CPUSeconds {
			g.cpu += (p.CPUSeconds - prev.CPUSeconds) / elapsed * 100
			g.hasCPU = true
			if p.ReadBytes != nil && prev.ReadBytes != nil && p.WriteBytes != nil && prev.WriteBytes != nil &&
				*p.ReadBytes >= *prev.ReadBytes && *p.WriteBytes >= *prev.WriteBytes {
				g.read += float64(*p.ReadBytes-*prev.ReadBytes) / elapsed
				g.write += float64(*p.WriteBytes-*prev.WriteBytes) / elapsed
				g.hasIO = true
			}
		}
	}
	rep.Series = append(rep.Series, topProcessSeries(groups, nets, s.prevNet, haveRates, elapsed, s.opts.topN())...)

	// storage paths
	for _, p := range s.opts.Paths {
		size, capped, err := dirSize(p, s.opts.PathBudget)
		if err != nil {
			rep.Notes = append(rep.Notes, "path "+p+": "+err.Error())
			continue
		}
		rep.Series = append(rep.Series, Series{Kind: KindPath, Key: "path:" + p, Name: p, StorageBytes: uptr(size), StorageCapped: capped})
	}

	// Hard cap: keep the busiest series. Paths always survive.
	if max := s.opts.maxSeries(); len(rep.Series) > max {
		sort.SliceStable(rep.Series, func(i, j int) bool { return weight(rep.Series[i]) > weight(rep.Series[j]) })
		rep.Series = rep.Series[:max]
		rep.Notes = append(rep.Notes, "series capped at "+strconv.Itoa(max))
	}

	s.prevAt, s.prevCgroup, s.prevProc, s.prevNet = now, curCg, curProc, nets
	return rep
}

type procGroup struct {
	name        string
	pids        int
	rss         uint64
	cpu         float64
	hasCPU      bool
	read, write float64
	hasIO       bool
}

func topProcessSeries(groups map[string]*procGroup, nets, prevNets map[string]NetCounters, haveRates bool, elapsed float64, n int) []Series {
	all := make([]*procGroup, 0, len(groups))
	for _, g := range groups {
		all = append(all, g)
	}
	pick := map[string]*procGroup{}
	sort.Slice(all, func(i, j int) bool { return all[i].cpu > all[j].cpu })
	for i := 0; i < len(all) && i < n && all[i].hasCPU && all[i].cpu > 0; i++ {
		pick[all[i].name] = all[i]
	}
	sort.Slice(all, func(i, j int) bool { return all[i].rss > all[j].rss })
	for i := 0; i < len(all) && i < n; i++ {
		pick[all[i].name] = all[i]
	}
	var out []Series
	for name, g := range pick {
		ser := Series{Kind: KindProcess, Key: "process:" + name, Name: name, MemoryBytes: uptr(g.rss), Pids: iptr(g.pids)}
		if g.hasCPU {
			ser.CPUPercent = fptr(g.cpu)
		}
		if g.hasIO {
			ser.IOReadBps, ser.IOWriteBps = fptr(g.read), fptr(g.write)
		}
		if cur, ok := nets[name]; ok && haveRates {
			if prev, ok := prevNets[name]; ok && cur.Rx >= prev.Rx && cur.Tx >= prev.Tx {
				ser.NetRxBps = fptr(float64(cur.Rx-prev.Rx) / elapsed)
				ser.NetTxBps = fptr(float64(cur.Tx-prev.Tx) / elapsed)
			}
		}
		out = append(out, ser)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func weight(s Series) float64 {
	if s.Kind == KindPath {
		return 1e18
	}
	w := 0.0
	if s.CPUPercent != nil {
		w += *s.CPUPercent * 1e6
	}
	if s.MemoryBytes != nil {
		w += float64(*s.MemoryBytes)
	}
	return w
}

// hostProcesses lists processes via gopsutil. I/O is best effort: processes
// owned by other users return permission errors and simply have no I/O.
func hostProcesses() ([]ProcSample, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}
	out := make([]ProcSample, 0, len(ps))
	for _, p := range ps {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		mi, err := p.MemoryInfo()
		if err != nil || mi == nil {
			continue
		}
		t, err := p.Times()
		if err != nil {
			continue
		}
		start, _ := p.CreateTime()
		sample := ProcSample{Pid: p.Pid, StartMs: start, Name: name, CPUSeconds: t.User + t.System, RSS: mi.RSS}
		if io, err := p.IOCounters(); err == nil && io != nil {
			// On Linux ReadBytes/WriteBytes are rchar/wchar (every read(2) incl.
			// pipes and page cache); the Disk* fields are actual storage I/O.
			if runtime.GOOS == "linux" {
				sample.ReadBytes, sample.WriteBytes = uptr(io.DiskReadBytes), uptr(io.DiskWriteBytes)
			} else {
				sample.ReadBytes, sample.WriteBytes = uptr(io.ReadBytes), uptr(io.WriteBytes)
			}
		}
		out = append(out, sample)
	}
	return out, nil
}
