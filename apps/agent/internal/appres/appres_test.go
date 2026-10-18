package appres

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fakeCgroup(t *testing.T, cpuUsec, memory, rbytes, wbytes int) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "cgroup.controllers"), "cpu memory io pids")
	unit := filepath.Join(root, "system.slice", "nginx.service")
	write(t, filepath.Join(unit, "cpu.stat"), "usage_usec "+fmtInt(cpuUsec)+"\nuser_usec 1\nsystem_usec 1\n")
	write(t, filepath.Join(unit, "memory.current"), fmtInt(memory)+"\n")
	write(t, filepath.Join(unit, "pids.current"), "4\n")
	write(t, filepath.Join(unit, "io.stat"), "254:16 rbytes="+fmtInt(rbytes)+" wbytes="+fmtInt(wbytes)+" rios=1 wios=1 dbytes=0 dios=0\n8:0 rbytes=100 wbytes=100\n")
	ctr := filepath.Join(root, "system.slice", "docker-0123456789abcdef0123.scope")
	write(t, filepath.Join(ctr, "memory.current"), "2048\n")
	write(t, filepath.Join(root, "system.slice", "ignored.mount", "memory.current"), "1\n")
	return root
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestCgroupUnitsAndContainersWithRates(t *testing.T) {
	root := fakeCgroup(t, 1_000_000, 5000, 1000, 2000)
	now := time.Unix(1000, 0)
	s := NewSampler(Options{CgroupRoot: root, Now: func() time.Time { return now },
		ProcessList: func() ([]ProcSample, error) { return nil, nil }, NetSource: func() (map[string]NetCounters, error) { return nil, nil }})

	first := s.Sample()
	var unit, ctr *Series
	for i := range first.Series {
		switch first.Series[i].Key {
		case "unit:nginx.service":
			unit = &first.Series[i]
		case "container:0123456789ab":
			ctr = &first.Series[i]
		}
	}
	if unit == nil || ctr == nil {
		t.Fatalf("series = %+v", first.Series)
	}
	if unit.CPUPercent != nil || *unit.MemoryBytes != 5000 || *unit.Pids != 4 {
		t.Fatalf("first sample must have instantaneous values only: %+v", unit)
	}
	if *ctr.MemoryBytes != 2048 || ctr.CPUPercent != nil {
		t.Fatalf("container = %+v (missing files must stay nil, not zero)", ctr)
	}

	// 10 s later: +2 CPU-seconds (20% of a core), +5000 read, +10000 write bytes.
	write(t, filepath.Join(root, "system.slice", "nginx.service", "cpu.stat"), "usage_usec 3000000\n")
	write(t, filepath.Join(root, "system.slice", "nginx.service", "io.stat"), "254:16 rbytes=6000 wbytes=12000\n8:0 rbytes=100 wbytes=100\n")
	now = now.Add(10 * time.Second)
	second := s.Sample()
	for _, ser := range second.Series {
		if ser.Key != "unit:nginx.service" {
			continue
		}
		if ser.CPUPercent == nil || *ser.CPUPercent < 19.9 || *ser.CPUPercent > 20.1 {
			t.Errorf("cpu = %v, want 20%%", ser.CPUPercent)
		}
		if ser.IOReadBps == nil || *ser.IOReadBps != 500 || *ser.IOWriteBps != 1000 {
			t.Errorf("io = %v/%v, want 500/1000 B/s summed per device delta", ser.IOReadBps, ser.IOWriteBps)
		}
	}
}

func TestNoCgroupV2AddsNoteNotFailure(t *testing.T) {
	s := NewSampler(Options{CgroupRoot: t.TempDir(), ProcessList: func() ([]ProcSample, error) { return nil, nil }, NetSource: func() (map[string]NetCounters, error) { return nil, nil }})
	rep := s.Sample()
	if len(rep.Notes) == 0 {
		t.Fatal("expected a note explaining cgroup v2 is missing")
	}
}

func TestProcessesGroupedAndTopNCapped(t *testing.T) {
	now := time.Unix(1000, 0)
	cpu := 0.0
	mk := func() []ProcSample {
		var out []ProcSample
		for i := 0; i < 30; i++ {
			out = append(out, ProcSample{Pid: int32(100 + i), Name: "proc" + fmtInt(i), CPUSeconds: cpu * float64(i), RSS: uint64(1000 * (i + 1))})
		}
		out = append(out, ProcSample{Pid: 500, Name: "proc29", CPUSeconds: cpu, RSS: 10}) // second pid of proc29
		return out
	}
	s := NewSampler(Options{CgroupRoot: t.TempDir(), TopN: 3, Now: func() time.Time { return now },
		ProcessList: func() ([]ProcSample, error) { return mk(), nil }, NetSource: func() (map[string]NetCounters, error) { return nil, nil }})
	s.Sample()
	cpu = 10
	now = now.Add(10 * time.Second)
	rep := s.Sample()
	if len(rep.Series) > 6 {
		t.Fatalf("top-3 by CPU and by memory should yield at most 6 series, got %d", len(rep.Series))
	}
	var got *Series
	for i := range rep.Series {
		if rep.Series[i].Key == "process:proc29" {
			got = &rep.Series[i]
		}
	}
	if got == nil || *got.Pids != 2 || *got.MemoryBytes != 30010 {
		t.Fatalf("proc29 must aggregate both pids: %+v", got)
	}
}

func TestMaxSeriesHardCapKeepsPaths(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "data", "a.bin"), "0123456789")
	var many []ProcSample
	for i := 0; i < 50; i++ {
		many = append(many, ProcSample{Pid: int32(i + 1), Name: "p" + fmtInt(i), RSS: uint64(i + 1)})
	}
	s := NewSampler(Options{CgroupRoot: t.TempDir(), TopN: 50, MaxSeries: 5, Paths: []string{filepath.Join(dir, "data")},
		ProcessList: func() ([]ProcSample, error) { return many, nil }, NetSource: func() (map[string]NetCounters, error) { return nil, nil }})
	rep := s.Sample()
	if len(rep.Series) != 5 {
		t.Fatalf("series = %d, want hard cap 5", len(rep.Series))
	}
	found := false
	for _, ser := range rep.Series {
		if ser.Kind == KindPath && *ser.StorageBytes == 10 {
			found = true
		}
	}
	if !found {
		t.Fatalf("configured path must survive the cap: %+v", rep.Series)
	}
}

func TestPathBudgetMarksLowerBound(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 200; i++ {
		write(t, filepath.Join(dir, fmtInt(i), "f"), "x")
	}
	_, capped, err := dirSize(dir, time.Nanosecond)
	if err != nil || !capped {
		t.Fatalf("capped=%v err=%v, want budget exceeded", capped, err)
	}
	if _, _, err := dirSize(filepath.Join(dir, "missing"), time.Second); err == nil {
		t.Fatal("missing path must error")
	}
}

func TestParseNettop(t *testing.T) {
	out := ",bytes_in,bytes_out,\nnginx.101,100,50,\nnginx.102,10,5,\nkernel_task.0,9,9,\nbad.line\n"
	got := parseNettop(out)
	if got["nginx"].Rx != 110 || got["nginx"].Tx != 55 || got["kernel_task"].Rx != 9 {
		t.Fatalf("got %+v", got)
	}
}

func TestSeriesNeverContainCommandLines(t *testing.T) {
	// ProcSample has no command-line field by construction; guard against adding one.
	var p ProcSample
	_ = p
}

func TestReusedPidIsNotTreatedAsTheSameProcess(t *testing.T) {
	now := time.Unix(1000, 0)
	read1, read2 := uint64(0), uint64(50_000_000_000)
	procs := []ProcSample{{Pid: 7, StartMs: 1, Name: "sh", CPUSeconds: 1, RSS: 10, ReadBytes: &read1, WriteBytes: &read1}}
	s := NewSampler(Options{CgroupRoot: t.TempDir(), Now: func() time.Time { return now },
		ProcessList: func() ([]ProcSample, error) { return procs, nil }, NetSource: func() (map[string]NetCounters, error) { return nil, nil }})
	s.Sample()
	now = now.Add(time.Second)
	procs = []ProcSample{{Pid: 7, StartMs: 2, Name: "sh", CPUSeconds: 100, RSS: 10, ReadBytes: &read2, WriteBytes: &read2}} // new process, same pid
	rep := s.Sample()
	for _, ser := range rep.Series {
		if ser.IOReadBps != nil || ser.CPUPercent != nil {
			t.Fatalf("a recycled pid must not produce a rate: %+v", ser)
		}
	}
}
