# Product Metrics

How Orion stores metric history and how an app sends its own metrics. Design: [plans/service-correlation-plan.md](plans/service-correlation-plan.md), sections 4.3 and 4.4. Logs are in [product-logs.md](product-logs.md).

## 1. Where the numbers come from

| Source | What it is | Series identity | Cap |
|---|---|---|---|
| `os` | CPU, memory, I/O, network, pids and storage of the processes, units, containers and paths a Node measures (`app_resources` in the Node report) | node + resource key | 200 per node (the Node's own cap) |
| `otlp` | Gauges, sums, histograms (explicit and exponential) and summaries an app sends over OTLP/HTTP | app + metric name + attributes | 100 per app by default, `metric_series_cap` per app up to 10,000 |

Both land in `metrics.db`, a SQLite file next to `orion.db` and `logs.db` (`ORION_DATA_DIR`). It is separate so sample volume never inflates `orion.db` or its migration backups. The latest values of node resources stay in `orion.db` (`node_app_resources`), as before.

Storage is behind the `MetricStore` interface (`apps/core/internal/metricstore`). The SQLite implementation has one writer goroutine; a Node report or an OTLP request only enqueues its samples, so a slow disk or a missing `metrics.db` never fails or delays a report. A full queue drops that minute's samples (the next report fills the gap) and OTLP answers 429 with `Retry-After`.

## 2. Resolution, retention and rollups

- Samples are kept per minute. For a gauge or a measured resource a second sample inside the same minute replaces the first; for a sum, histogram or summary, which hold the change in an interval, readings inside the same minute add up (an exporter that reports every 10 seconds loses nothing).
- One-minute rows are kept `metric_sample_retention_days` (default 7); hourly rollups (average and peak, and for histograms the merged bucket counts) are kept `metric_rollup_retention_days` (default 90). Both are in the data lifecycle settings (`PUT /v1/settings/data-lifecycle`) and apply to every kind of series.
- An hourly job rolls up complete hours and then prunes. It works series by series through the table's `(series_id, ts)` key in short statements, so it never holds the writer up behind a long scan.
- A sample that arrives after its hour was rolled up (an exporter retrying after an outage, up to 24 hours late) marks that hour dirty; the next run summarises it again. Raw rows that are not summarised yet, whether newer than the last rollup or in a dirty hour, are never pruned, so shortening retention cannot delete history that has no summary.
- `metric_rollup_retention_days` must be at least `metric_sample_retention_days`. Keep raw retention at 2 days or more: a range up to 48 hours reads one-minute rows only, so it is empty beyond the raw retention.
- A series that has not reported for longer than the rollup retention is removed with its rows. A series only counts against its cap while it reported in the last 24 hours, so a container redeployed under a new key does not hold a slot for months.
- Linking a thing to a service does not touch stored samples: a service's series are resolved from its links when you read, so unlinking hides the history and linking again brings it back.

## 3. Reading history

`GET /v1/services/{id}/metrics?from=-6h&to=now&max_points=500` (`from` and `to` are RFC 3339 times, `now`, or relative offsets such as `-6h` or `-7d`; `to` defaults to now) returns the series of the service's linked resources and of its linked apps' OTLP metrics. Ranges up to 48 hours read one-minute rows; longer ranges (up to 90 days) read hourly rollups. The step widens so a series returns about `max_points` points (default 500, at most 2,000). A field that was not measured is absent, never zero.

`GET /v1/services/{id}/log-counts?from=-6h&step_seconds=60` returns the apps' log entries per bucket and severity from a per-minute rollup kept in `logs.db`, so a timeline's error lane never counts raw rows. Ranges are limited to 7 days and 2,000 buckets.

What a point holds depends on the series `kind`:

| Kind | Point fields |
|---|---|
| `gauge` | `value` (average in the bucket), `value_max` (peak) |
| `sum` | `value`: the total increase in the bucket (divide by `step_seconds` for a rate); `value_max`: the largest one-minute increase |
| `histogram` | `count`, `sum`, `avg`, `min`, `max`, `p50`, `p95`, `p99` |
| `summary` | `count`, `sum`, `avg` (all three are changes in the bucket), `quantiles` as the app reported them, and `p50`, `p95`, `p99` when it reported those quantiles |

Percentiles of a histogram come from the merged bucket counts of the bucket (linear interpolation inside the bucket that holds the rank, clamped to `min` and `max` when known), never from averaging percentiles. A wide step, and the hourly rollups behind ranges over 48 hours, merge the counts of every row they cover, so a 7 or 30 day chart shows the percentile of that period. The first bucket of a histogram is treated as starting at `min` (or at 0 when the first bound is positive and `min` is unknown) and the last as ending at `max`; the precision is that of the app's buckets, so a coarse bucket layout gives coarse percentiles. A field that cannot be computed is absent: no `avg` without a sum or observations, no percentile for a histogram that sent no buckets or a summary that did not report that quantile. Summary quantiles cannot be merged exactly; inside a bucket they are averaged weighted by observation count and are a representative value, not a quantile of the union.

A response carries at most 100 series, of which at most 50 are app-reported (the most recently seen); `truncated` says when the service has more. The newest one to two hours are not in the hourly rollups yet, so a range over 48 hours ends at the last completed hour. Hourly averages are weighted by the rows they summarise.

The log counts are written in the same transaction as the log entries, so a failure there drops that log batch like any other write failure (after one retry). Counts exist only for logs written after this feature is deployed; there is no backfill of earlier logs.

Both answer 503 when the underlying store is not enabled.

## 4. Send metrics over OpenTelemetry

Point the OTLP/HTTP metrics exporter at Orion with the app's ingest key (the same key as logs):

```sh
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT="$ORION_URL/v1/ingest/otlp/v1/metrics"
export OTEL_EXPORTER_OTLP_METRICS_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_METRICS_PROTOCOL="http/json"   # http/protobuf also works
export OTEL_SERVICE_NAME="checkout-api"
```

- **Supported types:** gauges, sums, histograms with explicit buckets, exponential histograms and summaries.
- A series is a metric name plus its data point attributes (the first 20 by key, so the order an exporter sends them in does not matter) plus the resource attributes `service.name`, `service.instance.id` and `host.name`. Other resource attributes are ignored. A histogram or summary and a gauge or sum of the same name are different series.
- **Temporality.** A monotonic sum or a histogram with cumulative temporality (and a summary, whose count and sum are cumulative) is stored as the change since the previous reading of the same series, so a chart shows a per-minute count or rate and never an ever-growing counter. Delta temporality is stored as sent. An unspecified temporality is read as cumulative. The first cumulative reading of a series has nothing to subtract from: it only sets the baseline and produces no point, so a counter appears after its second export. A reset is detected when the start time of the reading changes or a count goes down (an exporter restart); the new reading is then the change. A non-monotonic sum with cumulative temporality (an up-down counter such as queue depth) is a level and is stored as a `gauge`; a non-monotonic sum with delta temporality is a net change and is stored as a `sum` that may be negative. A cumulative histogram's `min` and `max` cover the exporter's whole life, not the interval, so they are not stored.
- **Order and retries.** Orion keeps the time of the last reading it accepted per series (and, for cumulative series, its value, plus a hash of the reading) in `metrics.db` (`series_state`, one row per series; nothing is held in memory). A row is removed with its series, and the hourly prune also removes rows whose last reading is more than 24 hours old, because a gap that long always starts a new baseline. The table therefore holds the series seen in the last 24 hours; the series cap limits how many of those a single app can have. What counts as a duplicate depends on the temporality:
  - **Cumulative:** a reading that is not newer than the last one accepted is dropped, whether it is a retry or arrived out of order, so a retried request or a restart of Core never counts anything twice. A reading that arrives late is dropped, not merged.
  - **Delta:** only an exact repeat of the last accepted reading (same timestamp and same content) is dropped. A delta reading that is older than the last one, or has the same timestamp and different numbers, is a real change and is added to its minute.
  - **Known limit for delta series:** two exporter processes that share one series identity (the same metric, attributes, `service.instance.id` and `host.name`, which is the case when the last two are not set) and send identical readings at an identical timestamp look like one retry, and the second reading is dropped. A retry of a request other than the last accepted one for its series is not recognised and counts twice. Give every process its own `service.instance.id` so each has its own series.

  Dropped readings are not in `partial_success`, because the check runs in the writer after the request was answered; they are counted in `orion_metrics_dropped_total{reason="stale"}` on `/metrics`, and a batch that fails to write rolls back without counting. The other counters there are `orion_metrics_queue_batches`, `orion_metrics_written_total`, `orion_metrics_dropped_total{reason="full"}` and `{reason="cap"}`, `orion_metrics_batches_failed_total` and `orion_metrics_database_bytes`. A request lists its data points oldest first before they are written, so several minutes in one request are all kept.
- **Histogram storage.** Explicit buckets are stored as sent (bounds and counts as JSON) with count, sum, min and max. An exponential histogram is converted to the same shape: its bucket edges are computed from scale and offset (positive, negative and the zero bucket), and a data point with more than 160 buckets (negative and positive together) is downscaled, merging neighbouring bucket pairs until it fits; counts are never lost, only resolution. Readings of one series whose bounds differ (an exponential histogram that changed scale, a reconfigured exporter) are merged by splitting each bucket over the new bucket edges in proportion to the overlap. A summary keeps its quantile values as sent (at most 20) and no buckets. Bounds are limited to 512 per explicit data point. Expect roughly 300 bytes per histogram series per minute for a typical 15-bucket histogram and up to 8 KB for the largest ones, which the series cap and the retention settings bound.
- Rejected data points (counted in `partial_success.rejected_data_points`, first five distinct reasons in `error_message`): timestamp older than 24 hours or more than 5 minutes ahead, no value or flagged "no recorded value", NaN or infinity anywhere (value, sum, min, max, bound, quantile value), histogram bounds that are not strictly increasing, a number of bucket counts different from bounds plus one, bucket counts that do not add up to the count, counts above the signed 64-bit range, min greater than max, an exponential scale outside -10 to 20 or a bucket index that does not fit a float, a summary quantile outside 0 to 1, a negative value on a monotonic sum.
- Limits: the same request size and gzip handling as logs, at most 10,000 data points per request of any type (413 beyond that), and each data point counts as one unit against the app's ingest rate limit (429 with `Retry-After`, shared with log lines).
- Data points of series beyond the app's cap are refused and reported in `partial_success`. Raise it with `metric_series_cap` on the app (`PATCH /v1/apps/{id}`).
- Responses mirror the request encoding. Error responses (400, 401, 413, 415, 429, 503) are Orion JSON envelopes, as for logs. Metrics do not mark an app as "receiving" in the Logs view; that status stays about log lines.

## 5. Upgrading

Opening an older `metrics.db` adds the new tables (`histogram_samples`, `histogram_hourly`, `series_state`) in place, and adds the `last_hash` column to an existing `series_state`; nothing else about an existing table changes. One behaviour does change: OTLP sums used to be stored as the cumulative value the exporter sent and are now stored as the change per minute. Old rows cannot be told apart from new ones, so the first time Core starts after the upgrade it removes the stored history (one-minute rows and rollups) of the existing OTLP series of kind `sum`; the series themselves stay and fill again from their second export. History of gauges, histograms and measured resources is not touched. A non-monotonic sum that was stored as `sum` before is stored as `gauge` from now on, and is cleared the same way.

If you downgrade to a Core older than this change and later upgrade again, treat the version change as one-way where you can. The old Core writes OTLP sums as the cumulative totals the exporter sent. The one-time cleanup above has already run (its flag is set), so those rows are not removed on the second upgrade and read as huge per-minute changes. To clear them, stop Core and remove the flag, which makes the next start run the same cleanup again:

```sh
sqlite3 "$ORION_DATA_DIR/metrics.db" "DELETE FROM meta WHERE k = 'otlp_sum_deltas';"
```

The next start removes the stored history of every OTLP series of kind `sum`, including the correct per-minute history written since the upgrade; the series stay and fill again from their second export. Gauges, histograms and measured resources are not touched. A copy of `metrics.db` taken before the downgrade avoids all of this.

## 6. Measured size and speed

See [plans/backend-hardening-load-results.md](plans/backend-hardening-load-results.md), section "metrics.db at the series cap".
