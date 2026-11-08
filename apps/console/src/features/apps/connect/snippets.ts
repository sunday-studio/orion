import type { CodeTab } from "@/design-system";

/** "Send a log" examples: plain HTTP or OpenTelemetry. Orion has no SDKs. */
export const logSnippets = (origin: string): CodeTab[] => [
  {
    id: "curl",
    label: "curl",
    code: `curl -X POST ${origin}/v1/ingest/logs \\
  -H "Authorization: Bearer $ORION_KEY" \\
  -d '{"body":"Hello from Orion"}'`,
  },
  {
    id: "otel",
    label: "OpenTelemetry",
    code: `export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=${origin}/v1/ingest/otlp/v1/logs
export OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=http/json
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer $ORION_KEY"`,
  },
];
