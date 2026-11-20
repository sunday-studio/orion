package api

import (
	"errors"
	"net/http"
	"orion/core/internal/metricstore"
	"orion/core/internal/utils"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ingestOTLPMetricsImpl takes OTLP/HTTP metrics with the same ingest key, body limits and rate limiter
// as logs. Gauges, sums, histograms and summaries become samples of the app; data points over the app's series cap are
// refused and reported in partial_success, as log records are.
func (s *Server) ingestOTLPMetricsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, err := s.logs.ingester.Authenticate(c.GetHeader("Authorization"))
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer realm="orion-ingest"`)
		utils.Unauthorized(c, "Valid ingest key required")
		return
	}
	if s.metrics == nil {
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Metrics are not enabled on this Core instance", nil)
		return
	}
	body, ok := readIngestBody(c)
	if !ok {
		return
	}
	request, format, err := metricstore.DecodeOTLPMetrics(c.GetHeader("Content-Type"), body)
	switch {
	case errors.Is(err, metricstore.ErrUnsupportedMedia):
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "OTLP/HTTP needs application/x-protobuf or application/json", nil)
		return
	case errors.Is(err, metricstore.ErrInvalidBody):
		utils.BadRequest(c, "Request body is not a valid ExportMetricsServiceRequest")
		return
	case err != nil:
		utils.InternalError(c, "Failed to read the metrics request", err)
		return
	}
	result, failure := s.exportMetrics(app, request)
	switch {
	case failure == nil:
	case failure.kind == failTooMany:
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Too many data points in one request (limit 10000)", nil)
		return
	case failure.kind == failInvalid:
		utils.BadRequest(c, "Request body is not a valid ExportMetricsServiceRequest")
		return
	case failure.kind == failRateLimited:
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(failure.retryAfter)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest rate limit exceeded; retry after the indicated seconds", nil)
		return
	case failure.kind == failQueueFull:
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(failure.retryAfter)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Metric write queue is full; retry shortly", nil)
		return
	default:
		utils.InternalError(c, failure.message, failure.err)
		return
	}
	payload, contentType, err := metricstore.OTLPMetricsResponse(format, result.Rejected, result.Message)
	if err != nil {
		utils.InternalError(c, "Failed to encode the response", err)
		return
	}
	c.Data(http.StatusOK, contentType, payload)
}

// planMetricBatches drops the data points of series the app may not add, and groups the rest by minute
// (a batch has one timestamp). over counts the dropped data points.
func (s *Server) planMetricBatches(appID string, points []metricstore.TimedSample, seriesCap int) ([]metricstore.Batch, int64, error) {
	// Oldest first: the writer takes a series' readings in order and drops one that is not newer than the
	// last, so a request that lists its minutes newest first must not lose its older readings.
	points = append([]metricstore.TimedSample(nil), points...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].At.Before(points[j].At) })
	seen := map[string]bool{}
	var keys []string
	for _, p := range points {
		if !seen[p.Sample.Key] {
			seen[p.Sample.Key] = true
			keys = append(keys, p.Sample.Key)
		}
	}
	allowed, err := s.metrics.AllowSeries(metricstore.SourceOTLP, "", appID, keys, seriesCap)
	if err != nil {
		return nil, 0, err
	}
	var over int64
	byMinute := map[int64]*metricstore.Batch{}
	var order []int64
	for _, p := range points {
		if !allowed[p.Sample.Key] {
			over++
			continue
		}
		minute := p.At.Unix() / 60 * 60
		b := byMinute[minute]
		if b == nil {
			b = &metricstore.Batch{Source: metricstore.SourceOTLP, AppID: appID, At: time.Unix(minute, 0).UTC(), SeriesCap: seriesCap}
			byMinute[minute] = b
			order = append(order, minute)
		}
		b.Samples = append(b.Samples, p.Sample)
	}
	batches := make([]metricstore.Batch, 0, len(order))
	for _, m := range order {
		batches = append(batches, *byMinute[m])
	}
	return batches, over, nil
}

// enqueueMetricBatch hands a batch to the writer. A request with points spread over many minutes makes
// more batches than the queue holds, so a full queue is waited on briefly (the writer drains it in
// milliseconds) before the client is told to retry.
func (s *Server) enqueueMetricBatch(b metricstore.Batch) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := s.metrics.Enqueue(b)
		if !errors.Is(err, metricstore.ErrQueueFull) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
}
