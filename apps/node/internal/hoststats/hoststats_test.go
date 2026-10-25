package hoststats

import (
	"testing"
	"time"
)

func TestNetworkRatesFromDeltasAndReadyOnSecondSample(t *testing.T) {
	now := time.Unix(1000, 0)
	rx, tx := uint64(1000), uint64(500)
	s := New(func() (Reading, error) {
		return Reading{CPUPercent: 23, MemoryPercent: 61, DiskPercent: 44, Load1: 0.42, NetRx: rx, NetTx: tx}, nil
	}, "/")
	s.now = func() time.Time { return now }
	s.Sample()
	if first := s.Snapshot(); first.Ready || first.NetRxBps != 0 || first.CPUPercent != 23 {
		t.Fatalf("first sample has instant values but no rates: %+v", first)
	}
	now, rx, tx = now.Add(10*time.Second), 11000, 3500
	s.Sample()
	got := s.Snapshot()
	if !got.Ready || got.NetRxBps != 1000 || got.NetTxBps != 300 || got.DiskPath != "/" || got.Load1 != 0.42 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestCounterResetDoesNotProduceNegativeOrHugeRates(t *testing.T) {
	now := time.Unix(1000, 0)
	rx := uint64(5000)
	s := New(func() (Reading, error) { return Reading{NetRx: rx}, nil }, "")
	s.now = func() time.Time { return now }
	s.Sample()
	now, rx = now.Add(time.Second), 10 // interface counters reset
	s.Sample()
	if g := s.Snapshot(); g.NetRxBps != 0 {
		t.Fatalf("rate after a counter reset = %v", g.NetRxBps)
	}
}

func TestHostSourceReadsTheRealMachine(t *testing.T) {
	r, err := HostSource("/")()
	if err != nil || r.MemoryPercent <= 0 {
		t.Fatalf("%+v %v", r, err)
	}
}
