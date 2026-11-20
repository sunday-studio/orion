# Connect OpenTelemetry

Orion receives logs, metrics and traces from any OpenTelemetry SDK or Collector, over OTLP/HTTP and (when you turn it on) OTLP/gRPC. There is no Orion-specific code to add: set a few `OTEL_*` environment variables, or add one exporter to your Collector. This page is the setup guide; the per-signal details are in [product-logs.md](product-logs.md), [product-metrics.md](product-metrics.md), [product-traces.md](product-traces.md) and [otlp-grpc.md](otlp-grpc.md). Plan and as-built notes: [plans/opentelemetry-support.md](plans/opentelemetry-support.md).

## 1. What you need

1. A running Core ([getting started](getting-started.md)). Its address below is `$ORION_URL`, for example `https://orion.example.com`.
2. An **app** and an **ingest key**. In the Console open **Logs, Apps**, create an app, then create a key. It looks like `orion_ik_...`, is shown once, and can only write for that one app. One key covers all three signals. To rotate it, create a second key, deploy it, then revoke the old one.

Everything one app sends lands under that app. Link the app to a [service](plans/service-correlation-plan.md) and the service page shows its metrics, traces, error rate and latency.

## 2. Endpoints and authentication

| Signal | OTLP/HTTP path (on `$ORION_URL`) | OTLP/gRPC service |
|---|---|---|
| Logs | `POST /v1/ingest/otlp/v1/logs` | `opentelemetry.proto.collector.logs.v1.LogsService` |
| Metrics | `POST /v1/ingest/otlp/v1/metrics` | `opentelemetry.proto.collector.metrics.v1.MetricsService` |
| Traces | `POST /v1/ingest/otlp/v1/traces` | `opentelemetry.proto.collector.trace.v1.TraceService` |

- **HTTP** uses Core's normal port. Send `Content-Type: application/x-protobuf` (the SDK default) or `application/json`, optionally with `Content-Encoding: gzip`. There is no separate port 4318.
- **gRPC** is **off by default**. Set `ORION_OTLP_GRPC_ADDR=:4317` on Core, or with Docker Compose layer `deploy/docker-compose.otlp-grpc.yml`, which publishes the port on `127.0.0.1` only. The receiver has no TLS of its own, so put a TLS-terminating proxy in front before exposing it. Setup and status codes: [otlp-grpc.md](otlp-grpc.md).
- **Authentication** is the ingest key in the `Authorization: Bearer <key>` header (the `authorization` metadata key over gRPC). There is no other way to pass it.

The simplest setup uses the **base endpoint** `$ORION_URL/v1/ingest/otlp`: every OpenTelemetry exporter that speaks HTTP appends `/v1/traces`, `/v1/metrics` and `/v1/logs` to it, which are exactly Orion's paths. One endpoint and one header then cover all three signals.

## 3. SDKs with environment variables

Set these for the process that runs your app. They work with any SDK that follows the OpenTelemetry specification.

```sh
export OTEL_SERVICE_NAME="checkout-api"
export OTEL_EXPORTER_OTLP_ENDPOINT="$ORION_URL/v1/ingest/otlp"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"      # or http/json
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_EXPORTER_OTLP_COMPRESSION="gzip"
export OTEL_TRACES_EXPORTER="otlp"
export OTEL_METRICS_EXPORTER="otlp"
export OTEL_LOGS_EXPORTER="otlp"
```

Over gRPC the endpoint is the receiver, with no path:

```sh
export OTEL_EXPORTER_OTLP_ENDPOINT="http://orion.example.com:4317"   # https:// when TLS is in front
export OTEL_EXPORTER_OTLP_PROTOCOL="grpc"
export OTEL_EXPORTER_OTLP_HEADERS="authorization=Bearer $ORION_INGEST_KEY"
```

**The space in the header.** The specification percent-encodes header values. Java, Python and Go decode `%20`, and Python warns about a raw space, so write `Bearer%20<key>` there. The Node.js SDK takes the header as written, so a plain space is right. If you see a 401 with a key you know is good, switch to the other form.

To send one signal elsewhere, or only some signals, use the per-signal variables (`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`, `..._METRICS_...`, `..._LOGS_...`). Those take the **full** URL including the path, for example `$ORION_URL/v1/ingest/otlp/v1/traces`. Set `OTEL_METRICS_EXPORTER=none` or `OTEL_LOGS_EXPORTER=none` to switch a signal off.

### Node.js

```sh
npm install @opentelemetry/api @opentelemetry/auto-instrumentations-node
export OTEL_SERVICE_NAME="checkout-api"
export OTEL_EXPORTER_OTLP_ENDPOINT="$ORION_URL/v1/ingest/otlp"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer $ORION_INGEST_KEY"
export OTEL_TRACES_EXPORTER="otlp" OTEL_METRICS_EXPORTER="otlp" OTEL_LOGS_EXPORTER="otlp"
node --require @opentelemetry/auto-instrumentations-node/register app.js
```

### Python

```sh
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install
export OTEL_SERVICE_NAME="checkout-api"
export OTEL_EXPORTER_OTLP_ENDPOINT="$ORION_URL/v1/ingest/otlp"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer%20$ORION_INGEST_KEY"
export OTEL_TRACES_EXPORTER="otlp" OTEL_METRICS_EXPORTER="otlp" OTEL_LOGS_EXPORTER="otlp"
export OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED="true"   # send the logging module's records as logs
opentelemetry-instrument python app.py
```

### Go

The Go SDK has no automatic setup, but its OTLP exporters read every variable above, so the code carries no Orion settings:

```go
exporter, err := otlptracehttp.New(ctx) // go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
if err != nil {
	log.Fatal(err)
}
provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
defer provider.Shutdown(ctx)
otel.SetTracerProvider(provider)
```

```sh
export OTEL_SERVICE_NAME="checkout-api"
export OTEL_EXPORTER_OTLP_ENDPOINT="$ORION_URL/v1/ingest/otlp"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer%20$ORION_INGEST_KEY"
```

The metric and log exporters (`otlpmetrichttp`, `otlploghttp`) are created the same way. The package `go.opentelemetry.io/contrib/exporters/autoexport` picks the exporter from `OTEL_TRACES_EXPORTER` and the other variables if you prefer that.

### Java

Use the OpenTelemetry Java agent; nothing changes in the code:

```sh
curl -L -O https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar
export OTEL_SERVICE_NAME="checkout-api"
export OTEL_EXPORTER_OTLP_ENDPOINT="$ORION_URL/v1/ingest/otlp"
export OTEL_EXPORTER_OTLP_PROTOCOL="http/protobuf"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer%20$ORION_INGEST_KEY"
export OTEL_TRACES_EXPORTER="otlp" OTEL_METRICS_EXPORTER="otlp" OTEL_LOGS_EXPORTER="otlp"
java -javaagent:./opentelemetry-javaagent.jar -jar app.jar
```

Make sure each process has its own `service.instance.id` (many SDKs set one for you) so metric series from two processes do not collide: [product-metrics.md](product-metrics.md), section 4.

## 4. OpenTelemetry Collector

A Collector in front of your apps can send to Orion over HTTP or gRPC. The `otlphttp` exporter appends `/v1/traces`, `/v1/metrics` and `/v1/logs` to its `endpoint`, and retries on 429, 502, 503 and 504 using the `Retry-After` header.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
      http:

processors:
  batch:

exporters:
  otlphttp/orion:
    endpoint: ${env:ORION_URL}/v1/ingest/otlp
    headers:
      Authorization: "Bearer ${env:ORION_INGEST_KEY}"
    compression: gzip
    # encoding: json          # protobuf is the default and the smaller choice

  # The same data over gRPC instead; needs ORION_OTLP_GRPC_ADDR on Core.
  otlp/orion:
    endpoint: orion.example.com:4317
    tls:
      insecure: true          # drop this when a TLS proxy is in front
    headers:
      authorization: "Bearer ${env:ORION_INGEST_KEY}"
    compression: gzip

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/orion]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/orion]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [otlphttp/orion]
```

An ingest key belongs to one app, so one exporter sends one app's data. A Collector that gathers several apps needs one exporter per app (each with its own key and a pipeline that filters or routes to it), otherwise everything lands under the key's app.

## 5. What is supported, and what is not

| | Supported |
|---|---|
| Signals | Logs, metrics and traces. |
| Transports | OTLP/HTTP with protobuf or JSON bodies, optionally gzip; OTLP/gRPC (opt-in), with gzip. |
| Metric types | Gauges, sums, histograms with explicit buckets, exponential histograms and summaries. Cumulative sums and histograms are turned into per-interval changes. |
| Responses | A response mirrors the request encoding and reports refused records in `partial_success`. |
| JSON ids | `traceId`, `spanId` and `parentSpanId` as hex strings, as the OTLP/JSON specification says (base64 is also read). |
| Console | Traces under **Logs, Traces** (list, filters, waterfall, span details, the logs of a trace), trace and log links in both directions, traces, error rate and latency on the service page, histograms as p50, p95 and p99 lines. |

Not supported:

- **Profiles**, the fourth OTLP signal.
- **Sending data out** of Orion over OTLP. Orion only receives.
- **Sampling and tail sampling.** Orion stores what it is sent. Sample in the SDK, or in a Collector with the `tail_sampling` processor, before you export.
- **Exemplars** on metrics are accepted and ignored.
- **TLS on the gRPC receiver**, and OTLP/HTTP on a port other than Core's. Put a proxy in front for TLS.
- **Per-span alert rules.** Alerts on traces come later.
- Resource attributes of metrics other than `service.name`, `service.instance.id` and `host.name` do not become labels.

## 6. Limits

Limits are the same over HTTP and gRPC and are listed with each signal, so they are not repeated here: [logs](product-logs.md#5-limits-errors-and-retries), [metrics](product-metrics.md#4-send-metrics-over-opentelemetry), [traces](product-traces.md#3-limits-errors-and-retries) and [gRPC connections](otlp-grpc.md#4-limits). All three signals share one rate limit per app (each log record, data point and span counts as one unit), and request bodies are capped at 5 MiB after decompression.

## 7. Troubleshooting

| You see | Cause and fix |
|---|---|
| `401` (HTTP) or `Unauthenticated` (gRPC) | The key is missing, unknown, revoked, or not an ingest key. Check the `Authorization` header. If the header comes from `OTEL_EXPORTER_OTLP_HEADERS`, try the other spelling of the space (`Bearer <key>` or `Bearer%20<key>`, section 3). |
| `404`, or an unexpected page back | The URL is wrong. With `OTEL_EXPORTER_OTLP_ENDPOINT` use `$ORION_URL/v1/ingest/otlp` and let the SDK add `/v1/traces`. The per-signal variables take the full URL, `/v1/traces` included. |
| `415` | The body is neither `application/x-protobuf` nor `application/json`. Set `OTEL_EXPORTER_OTLP_PROTOCOL` to `http/protobuf` or `http/json`. An exporter set to `grpc` sends HTTP/2 gRPC frames to the HTTP port, which Core cannot read; point it at the gRPC receiver instead. |
| `400` | The body does not parse. A trace or span id that is neither hex nor base64 of the right size is one cause (see below). Bad gzip is another. |
| `413` | More than 10,000 spans, data points or log records in a request, or a body over 5 MiB. Lower the batch size of the exporter. |
| `429` with `Retry-After` | The app's rate limit, or a write queue is full. Exporters that follow the specification wait and retry. Lower the volume, or raise the app's rate limit in its settings. |
| `503` | That signal is off on this Core, or its database is out of space. The `Retry-After` header says when to try again. Free disk space or lower retention. |
| `200`, but data is missing | Read `partial_success` in the response: it counts the refused records and gives up to five reasons, for example `invalid_trace_id` or a metric series cap. The rest of the request was stored. |
| Trace or span ids rejected | OTLP/JSON writes ids as **hex** (`"traceId":"5b8efff798038103d269b633813fc60c"`), the protobuf JSON mapping would write base64. Core reads both. An id must be 16 bytes (trace) or 8 bytes (span) and not all zero; a 32 or 16 character string that is not hex, or any other length, is refused. |
| A counter shows nothing | Cumulative sums and histograms store the change since the last reading, so the first export of a series is only a baseline. The counter appears after its second export. A counter that is flat in an interval shows zero change. |
| gRPC exporter cannot connect | The receiver is off by default (`ORION_OTLP_GRPC_ADDR`), and the Compose overlay publishes it on `127.0.0.1` only. Use `http://` for the endpoint unless TLS is in front. |
| Data arrives but the service page is empty | Link the app to the service from the service page. The Traces list under Logs shows every app without a link. |

If an exporter retries a request that Core already stored, a span is stored once and a cumulative metric reading that is not newer than the last one is dropped, so retries do not double count.
