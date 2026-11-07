package api

import (
	"errors"
	"net/http"
	"orion/core/internal/logstore"
	"orion/core/internal/metricstore"
	"orion/core/internal/utils"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ingestOTLPMetricsImpl takes OTLP/HTTP metrics with the same ingest key, body limits and rate limiter
// as logs. Gauges and sums become samples of the app; data points over the app's series cap are
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
	now := time.Now().UTC()
	parsed, format, err := metricstore.ParseOTLPMetrics(c.GetHeader("Content-Type"), body, now)
	switch {
	case errors.Is(err, metricstore.ErrUnsupportedMedia):
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "OTLP/HTTP needs application/x-protobuf or application/json", nil)
		return
	case errors.Is(err, metricstore.ErrTooManyPoints):
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Too many data points in one request (limit 10000)", nil)
		return
	case errors.Is(err, metricstore.ErrInvalidBody):
		utils.BadRequest(c, "Request body is not a valid ExportMetricsServiceRequest")
		return
	case err != nil:
		utils.InternalError(c, "Failed to read the metrics request", err)
		return
	}

	rejected, messages := parsed.Rejected, parsed.Errors
	if len(parsed.Points) > 0 {
		var limited *logstore.RateLimitedError
		if err := s.logs.ingester.Allow(app, len(parsed.Points)); errors.As(err, &limited) {
			seconds := int((limited.RetryAfter + 999_999_999) / 1_000_000_000)
			c.Header("Retry-After", strconv.Itoa(max(seconds, 1)))
			utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest rate limit exceeded; retry after the indicated seconds", nil)
			return
		}
		seriesCap := app.App.MetricSeriesCap
		if seriesCap <= 0 {
			seriesCap = metricstore.DefaultSeriesCapPerApp
		}
		batches, over, err := s.planMetricBatches(app.App.ID, parsed.Points, seriesCap)
		if err != nil {
			utils.InternalError(c, "Failed to check the metric series limit", err)
			return
		}
		if over > 0 {
			rejected += over
			messages = append(messages, "app reached its metric series limit of "+strconv.Itoa(seriesCap)+"; raise metric_series_cap on the app to accept more")
		}
		for _, b := range batches {
			if err := s.metrics.Enqueue(b); err != nil {
				c.Header("Retry-After", "1")
				utils.ErrorResponse(c, http.StatusTooManyRequests, "Metric write queue is full; retry shortly", nil)
				return
			}
		}
	}
	message := ""
	for i, m := range messages {
		if i > 0 {
			message += "; "
		}
		message += m
	}
	payload, contentType, err := metricstore.OTLPMetricsResponse(format, rejected, message)
	if err != nil {
		utils.InternalError(c, "Failed to encode the response", err)
		return
	}
	c.Data(http.StatusOK, contentType, payload)
}

// planMetricBatches drops the data points of series the app may not add, and groups the rest by minute
// (a batch has one timestamp). over counts the dropped data points.
func (s *Server) planMetricBatches(appID string, points []metricstore.TimedSample, seriesCap int) ([]metricstore.Batch, int64, error) {
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
