# Upgrade notes: 0.5.0

0.5.0 adds full OpenTelemetry support: traces, richer metrics and an optional OTLP/gRPC receiver, with trace
views in the Console. Nothing is removed from the API. The Node has no feature changes; it is rebuilt with a
newer Go release, and a Node on 0.4.0 keeps working with this Core.

## Core

Upgrade Core first, then Nodes, as for any minor release.

```sh
ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:0.5.0 docker compose up -d
```

- Take a backup first ([SQLite backup and restore](../sqlite-backup-restore.md)). Core applies migration
  `000062` on first start; it only adds the `trace_retention_days` data lifecycle setting (default 7 days).
  Core also copies `orion.db` into `backups/` before it migrates.
- Core creates `traces.db` next to the other databases for spans and their per-trace summaries. It is optional to
  back up, because it refills as apps keep sending. Spans are much larger than log lines, so plan for more disk
  if you turn traces on, and see [Product traces](../product-traces.md) for retention and the storage cap per app.
- **Old OTLP counter history in `metrics.db` is deleted once**, on the first start after the upgrade. Earlier
  versions stored OTLP sums as the raw cumulative value; 0.5.0 stores the change per minute, and the two cannot
  be told apart afterwards. The metric series stay and fill again as apps send; a counter shows its first
  change after its second export. Gauges and the metrics of Nodes are not touched.
- Metrics gain histograms (explicit and exponential buckets, charted as p50, p95 and p99) and summaries. See
  [Product metrics](../product-metrics.md).
- The OTLP/gRPC receiver is **off by default**. To use it, set `ORION_OTLP_GRPC_ADDR` (for example `:4317`), or
  layer `deploy/docker-compose.otlp-grpc.yml`, which publishes the port on `127.0.0.1` only. The receiver has no
  TLS of its own, so put a proxy in front before exposing it. See [OTLP/gRPC](../otlp-grpc.md).
- OTLP/JSON trace and span ids are now read as hex, as the specification says, for logs, metrics and traces.
  Senders that used base64 still work.
- Go moves to 1.26.11 for Core and the Node, which fixes GO-2026-6859 in `net/http`.

## Console

- New **Logs, Traces** page: a list with filters, a span waterfall, span details and the logs of a trace, with
  links between traces, logs and services. A service page shows its traces, error rate, latency and app metrics.
- To send data, follow [Connect OpenTelemetry](../opentelemetry.md): one ingest key per app covers logs, metrics
  and traces. Write the space in the `Authorization` header as `%20` in `OTEL_EXPORTER_OTLP_HEADERS`.

## Nodes

No change is needed. Update them when convenient, from the Console or with `sudo orion-node update`.
