# Product Metrics

How Orion stores metric history and how an app sends its own metrics. Design: [plans/service-correlation-plan.md](plans/service-correlation-plan.md), sections 4.3 and 4.4. Logs are in [product-logs.md](product-logs.md).

## 1. Where the numbers come from

| Source | What it is | Series identity | Cap |
|---|---|---|---|
| `os` | CPU, memory, I/O, network, pids and storage of the processes, units, containers and paths a Node measures (`app_resources` in the Node report) | node + resource key | 200 per node (the Node's own cap) |
| `otlp` | Gauges and sums an app sends over OTLP/HTTP | app + metric name + attributes | 100 per app by default, `metric_series_cap` per app up to 10,000 |

Both land in `metrics.db`, a SQLite file next to `orion.db` and `logs.db` (`ORION_DATA_DIR`). It is separate so sample volume never inflates `orion.db` or its migration backups. The latest values of node resources stay in `orion.db` (`node_app_resources`), as before.

Storage is behind the `MetricStore` interface (`apps/core/internal/metricstore`). The SQLite implementation has one writer goroutine; a Node report or an OTLP request only enqueues its samples, so a slow disk or a missing `metrics.db` never fails or delays a report. A full queue drops that minute's samples (the next report fills the gap) and OTLP answers 429 with `Retry-After`.

## 2. Resolution, retention and rollups

- Samples are kept per minute: a second sample inside the same minute replaces the first.
- One-minute rows are kept `metric_sample_retention_days` (default 7); hourly rollups (average and peak) are kept `metric_rollup_retention_days` (default 90). Both are in the data lifecycle settings (`PUT /v1/settings/data-lifecycle`).
- An hourly job rolls up complete hours and then prunes. Raw rows that have not been rolled up yet are never pruned, so shortening retention cannot delete history that has no summary.
- A series that has not reported for longer than the rollup retention is removed with its rows. A series only counts against its cap while it reported in the last 24 hours, so a container redeployed under a new key does not hold a slot for months.
- Linking a thing to a service does not touch stored samples: a service's series are resolved from its links when you read, so unlinking hides the history and linking again brings it back.

## 3. Reading history

`GET /v1/services/{id}/metrics?from=-6h&to=now&max_points=500` returns the series of the service's linked resources and of its linked apps' OTLP metrics. Ranges up to 48 hours read one-minute rows; longer ranges (up to 90 days) read hourly rollups. The step widens so a series returns about `max_points` points (default 500, at most 2,000). A field that was not measured is absent, never zero.

`GET /v1/services/{id}/log-counts?from=-6h&step_seconds=60` returns the apps' log entries per bucket and severity from a per-minute rollup kept in `logs.db`, so a timeline's error lane never counts raw rows. Ranges are limited to 7 days and 2,000 buckets.

Both answer 503 when the underlying store is not enabled.

## 4. Send metrics over OpenTelemetry

Point the OTLP/HTTP metrics exporter at Orion with the app's ingest key (the same key as logs):

```sh
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="$ORION_URL/v1/ingest/otlp/v1/metrics"
export OTEL_EXPORTER_OTLP_METRICS_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_METRICS_PROTOCOL="http/json"   # http/protobuf also works
export OTEL_SERVICE_NAME="checkout-api"
```

- Gauges and sums are stored. Histograms, exponential histograms and summaries are not supported yet: their data points are counted in `partial_success.rejected_data_points` with a message.
- A series is a metric name plus its data point attributes plus the resource attributes `service.name`, `service.instance.id` and `host.name`. Other resource attributes are ignored.
- Cumulative and delta sums are stored as sent; Orion does not convert them to rates yet.
- Data points older than 24 hours, more than 5 minutes ahead, without a value or with NaN or infinity are rejected.
- Limits: the same request size and gzip handling as logs, at most 10,000 data points per request (413 beyond that), and each data point counts as one unit against the app's ingest rate limit (429 with `Retry-After`, shared with log lines).
- Data points of series beyond the app's cap are refused and reported in `partial_success`. Raise it with `metric_series_cap` on the app (`PATCH /v1/apps/{id}`).
- Responses mirror the request encoding. Error responses (400, 401, 413, 415, 429, 503) are Orion JSON envelopes, as for logs. Metrics do not mark an app as "receiving" in the Logs view; that status stays about log lines.

## 5. Measured size and speed

See [plans/backend-hardening-load-results.md](plans/backend-hardening-load-results.md), section "metrics.db at the series cap".
