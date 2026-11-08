package api

import (
	"orion/core/internal/metricstore"
	"orion/core/internal/service"
	"time"
)

// EnableMetrics turns on resource history using an opened metric store. Until then reports are
// stored as before and the history routes answer 503.
func (s *Server) EnableMetrics(store metricstore.MetricStore) {
	s.metrics = store
}

// recordResourceSamples hands a report's measurements to the metric writer. It never waits and never
// fails the report: a full queue just drops this minute, and the next report fills the gap.
func (s *Server) recordResourceSamples(nodeID string, series []service.AppResourceSeries) {
	if s.metrics == nil || len(series) == 0 {
		return
	}
	if err := s.metrics.Enqueue(resourceBatch(nodeID, series, time.Now().UTC())); err != nil {
		s.logger.Debug("Dropped resource samples", "error", err, "node_id", nodeID)
	}
}

// resourceBatch converts a report's series into a metric batch. The store applies the series cap.
func resourceBatch(nodeID string, series []service.AppResourceSeries, at time.Time) metricstore.Batch {
	b := metricstore.Batch{Source: metricstore.SourceOS, NodeID: nodeID, At: at, Samples: make([]metricstore.Sample, 0, len(series))}
	for _, r := range series {
		v := metricstore.Values{CPUPercent: r.CPUPercent, IOReadBps: r.IOReadBps, IOWriteBps: r.IOWriteBps, NetRxBps: r.NetRxBps, NetTxBps: r.NetTxBps}
		if r.MemoryBytes != nil {
			n := int64(*r.MemoryBytes)
			v.MemoryBytes = &n
		}
		if r.StorageBytes != nil {
			n := int64(*r.StorageBytes)
			v.StorageBytes = &n
		}
		if r.Pids != nil {
			n := int64(*r.Pids)
			v.Pids = &n
		}
		b.Samples = append(b.Samples, metricstore.Sample{Key: r.Key, Kind: r.Kind, Name: r.Name, Values: v})
	}
	return b
}
