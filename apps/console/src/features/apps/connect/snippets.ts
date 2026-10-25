import type { CodeTab } from "@/design-system";

/** "Send a log" examples; every one targets POST /v1/ingest/logs or the OTLP endpoint. */
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
  {
    id: "typescript",
    label: "TypeScript",
    code: `await fetch("${origin}/v1/ingest/logs", {
  method: "POST",
  headers: { Authorization: \`Bearer \${process.env.ORION_KEY}\` },
  body: JSON.stringify({ body: "Hello from Orion" }),
});`,
  },
  {
    id: "go",
    label: "Go",
    code: `req, _ := http.NewRequest("POST", "${origin}/v1/ingest/logs",
    strings.NewReader(\`{"body":"Hello from Orion"}\`))
req.Header.Set("Authorization", "Bearer "+os.Getenv("ORION_KEY"))
http.DefaultClient.Do(req)`,
  },
  {
    id: "python",
    label: "Python",
    code: `import os, requests

requests.post(
    "${origin}/v1/ingest/logs",
    headers={"Authorization": f"Bearer {os.environ['ORION_KEY']}"},
    json={"body": "Hello from Orion"},
)`,
  },
];
