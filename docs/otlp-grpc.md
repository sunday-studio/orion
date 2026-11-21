# Send OpenTelemetry to Orion over gRPC

Many OpenTelemetry SDKs and the Collector default to OTLP/gRPC on port 4317. Core can receive logs, metrics and traces that way, with the same app ingest keys, limits, redaction and rate limits as OTLP/HTTP. Plan: [plans/opentelemetry-support.md](plans/opentelemetry-support.md), phase 2. The setup for all three signals over HTTP and gRPC, with examples for Node.js, Python, Go, Java and the Collector, is in [opentelemetry.md](opentelemetry.md). The signals themselves are described in [product-logs.md](product-logs.md), [product-metrics.md](product-metrics.md) and [product-traces.md](product-traces.md).

The receiver is **off by default**. gRPC is not part of the OpenAPI contract (`apps/core/openapi.yaml` describes the HTTP API); the three services are the standard `opentelemetry.proto.collector.{logs,metrics,trace}.v1` ones.

## 1. Turn it on

Set `ORION_OTLP_GRPC_ADDR` on the Core API to the address to listen on, for example `:4317`. Core logs `Starting OTLP/gRPC receiver` with the address when it is up, and refuses to start if the address cannot be opened.

With the Docker Compose install, layer the opt-in file over the base one; it sets the address and publishes the port (`ORION_OTLP_GRPC_PORT`, default 4317, on `ORION_OTLP_GRPC_HOST`, default `127.0.0.1`):

```sh
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.otlp-grpc.yml --env-file .env up -d
```

The default install is unchanged: without that file nothing listens and nothing extra is published.

**The receiver speaks plaintext HTTP/2 and has no TLS of its own**, like the HTTP port. The ingest key travels in every request, so do not expose the port publicly without something in front that terminates TLS (a reverse proxy that forwards gRPC, for example nginx `grpc_pass grpc://127.0.0.1:4317` or Caddy `reverse_proxy h2c://127.0.0.1:4317`). Listening on `127.0.0.1:4317` and putting the proxy on the same host is the safe layout. `ORION_LISTEN_HOST` does not apply to this port: the address is exactly what you set.

## 2. Connect an app

Use the app's ingest key (the one for logs and metrics, from **Logs > Apps**), sent as the `authorization` header:

```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="http://orion.example.com:4317"   # https:// when TLS is in front
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer%20$ORION_INGEST_KEY"
export OTEL_SERVICE_NAME="checkout-api"
```

That one set covers all three signals. The space is written as `%20` because the SDKs percent-decode the variable (an older SDK that does not decode it needs a plain space instead). In a Collector config the header is sent as written, so there it is a real space. Per-signal variables (`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `..._METRICS_...`, `..._LOGS_...`) work as usual; with gRPC the endpoint has no path.

An OpenTelemetry Collector exports with the `otlp` exporter:

```yaml
exporters:
  otlp/orion:
    endpoint: orion.example.com:4317
    tls:
      insecure: true            # drop this when TLS is in front
    headers:
      authorization: "Bearer ${env:ORION_INGEST_KEY}"
```

gzip compression is accepted (`OTEL_EXPORTER_OTLP_COMPRESSION=gzip`, or `compression: gzip` on the Collector exporter).

## 3. How requests are answered

A request goes through the same code as OTLP/HTTP: the same parsing and validation, the same limits per request, span, attribute and series, the same redaction, the same per-app rate limit (spans, data points and log records share one budget) and the same bounded write queues. A request that is accepted gets an empty response, or one with `partial_success` when some records, data points or spans were refused; the rest of the request is stored.

A request that is refused as a whole gets a gRPC status. Those worth retrying carry the wait in a `retry-after` trailer (whole seconds, as the HTTP header) and in the standard `RetryInfo` error detail, which OpenTelemetry exporters read; the others do not, so exporters give up on them as they should.

| gRPC status | When | Retry |
|---|---|---|
| `Unauthenticated` | no `authorization` metadata, or a key that is unknown, revoked or not an ingest key | no: fix the key. Checked before the message is read. |
| `InvalidArgument` | the message does not parse, or it has more than 10,000 spans, data points or log records | no: fix or split the request |
| `ResourceExhausted` with `RetryInfo` | the app's ingest rate limit, or the write queue is full | yes, after the `retry-after` seconds |
| `ResourceExhausted` without `RetryInfo` | the message is over 5 MiB (after decompression) | no: send smaller batches |
| `Unavailable` with `RetryInfo` | the store is out of space; nothing was accepted | yes, after `retry-after`; free disk space or lower retention |
| `Unavailable` | that signal is off on this Core (for example no `traces.db`) | exporters retry with their own backoff; the cause is on the Core side |
| `Internal` | a fault in Core; details are in Core's log, not in the response | exporters do not retry it |

The same cases answer 401, 400 or 413, 429, 503 on OTLP/HTTP.

## 4. Limits

| Limit | Value |
|---|---|
| Message size | 5 MiB, the OTLP/HTTP body cap, also after gzip decompression |
| Open connections | 1,024 (more wait to be accepted) |
| Requests in flight per connection | 64 |
| Request headers | 32 KiB |
| New connection setup | 10 seconds to finish the HTTP/2 handshake |
| Idle connection | closed after 5 minutes; clients reconnect on their next export |
| Connection age | closed after 30 minutes (30 seconds to finish what is running); clients reconnect on their own |
| Silent client | pinged after 2 minutes, dropped if it does not answer in 20 seconds; a client may not ping more often than every 10 seconds |

Per-request limits (10,000 entries, attributes, spans of 256 KiB and so on) are listed with each signal: [logs](product-logs.md), [metrics](product-metrics.md), [traces](product-traces.md).

## 5. Shutdown

On SIGINT or SIGTERM Core stops accepting new gRPC connections and lets the exports in flight finish for up to 10 seconds, then closes what is left, before it flushes the write queues and exits. An exporter whose call was cut gets `Unavailable` and retries it.
