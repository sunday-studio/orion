import type { ApiMonitorReportResponse } from "@/orion-sdk";

export type MonitorPayload = Record<string, unknown>;

export type MonitorResultItem = {
  label: string;
  value: string;
};

export type MonitorResultSummary = {
  kind: string;
  kindLabel: string;
  headline: string;
  items: MonitorResultItem[];
  explanation: string;
  payload: MonitorPayload;
};

const EMPTY_VALUE = "—";

export const parseMonitorPayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

export const readMonitorPayloadString = (
  payload: MonitorPayload,
  keys: string[],
): string | undefined => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number" && Number.isFinite(value)) return String(value);
    if (typeof value === "boolean") return value ? "yes" : "no";
  }
  return undefined;
};

export const readMonitorPayloadNumber = (
  payload: MonitorPayload,
  keys: string[],
): number | undefined => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "number" && Number.isFinite(value)) return value;
    if (typeof value === "string" && value.trim() !== "" && !Number.isNaN(Number(value))) {
      return Number(value);
    }
  }
  return undefined;
};

const readMonitorPayloadList = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (Array.isArray(value) && value.length > 0) {
      return value.map((item) => String(item)).join(", ");
    }
    if (typeof value === "string" && value.trim() !== "") return value;
  }
  return undefined;
};

const formatLatencyValue = (value?: number) =>
  typeof value === "number" ? `${Math.round(value)} ms` : EMPTY_VALUE;

export const formatMonitorLatency = (payload: MonitorPayload) =>
  formatLatencyValue(
    readMonitorPayloadNumber(payload, ["latency_ms", "response_time_ms", "duration_ms"]),
  );

const normalizeMonitorKind = (kind?: string) => {
  switch (kind) {
    case "http_status":
    case "http_keyword":
    case "expected_status":
      return "http";
    case "tcp_port":
      return "tcp";
    case "tls_certificate":
      return "tls";
    default:
      return kind ?? "unknown";
  }
};

const monitorKind = (payload: MonitorPayload, fallbackKind?: string) => {
  const payloadKind = readMonitorPayloadString(payload, ["type"]);
  const runner = readMonitorPayloadString(payload, ["runner"]);
  if (payloadKind === "heartbeat" || runner === "heartbeat") return "heartbeat";
  return normalizeMonitorKind(payloadKind ?? fallbackKind);
};

const kindLabel = (kind: string) => {
  switch (kind) {
    case "api_request":
      return "API request";
    case "dns":
      return "DNS";
    case "heartbeat":
      return "Heartbeat";
    case "http":
      return "HTTP";
    case "tcp":
      return "TCP";
    case "tls":
      return "TLS";
    default:
      return kind === "unknown" ? "Monitor" : kind.replaceAll("_", " ");
  }
};

const item = (label: string, value?: string | number | boolean): MonitorResultItem => ({
  label,
  value:
    typeof value === "number"
      ? String(value)
      : typeof value === "boolean"
        ? value
          ? "yes"
          : "no"
        : value?.trim() || EMPTY_VALUE,
});

const expectedStatusLabel = (payload: MonitorPayload) =>
  readMonitorPayloadList(payload, ["expected_statuses"]) ??
  readMonitorPayloadString(payload, ["expected_status"]) ??
  EMPTY_VALUE;

const statusHeadline = (payload: MonitorPayload, noun: string) => {
  const statusCode = readMonitorPayloadString(payload, ["status_code", "code"]);
  const expected = expectedStatusLabel(payload);
  if (statusCode && expected !== EMPTY_VALUE) return `${noun} ${statusCode}, expected ${expected}`;
  if (statusCode) return `${noun} ${statusCode}`;
  return `${noun} unavailable`;
};

const resultHeadline = (
  kind: string,
  payload: MonitorPayload,
  report?: ApiMonitorReportResponse,
) => {
  const health = report?.health ?? readMonitorPayloadString(payload, ["status", "health"]);
  if (kind === "http") return statusHeadline(payload, "HTTP");
  if (kind === "api_request") return statusHeadline(payload, "API");
  if (kind === "tcp") {
    const address = readMonitorPayloadString(payload, ["address"]);
    return health === "up"
      ? `Connected${address ? ` to ${address}` : ""}`
      : `Connection ${health ?? "unknown"}`;
  }
  if (kind === "dns") {
    const answers = readMonitorPayloadList(payload, ["answers"]);
    return answers ? `Resolved ${answers}` : `DNS ${health ?? "unknown"}`;
  }
  if (kind === "tls") {
    const days = readMonitorPayloadString(payload, ["tls_days_remaining"]);
    return days ? `Certificate expires in ${days} days` : `TLS ${health ?? "unknown"}`;
  }
  if (kind === "heartbeat") {
    const missedAfter = readMonitorPayloadString(payload, ["missed_after"]);
    return missedAfter
      ? `Missed heartbeat after ${missedAfter}`
      : `Heartbeat ${health ?? "unknown"}`;
  }
  return (
    readMonitorPayloadString(payload, [
      "summary",
      "message",
      "failure_reason",
      "error",
      "status",
    ]) ?? `Monitor ${health ?? "unknown"}`
  );
};

const monitorResultItems = (kind: string, payload: MonitorPayload): MonitorResultItem[] => {
  const latency = formatMonitorLatency(payload);
  switch (kind) {
    case "http":
      return [
        item("target", readMonitorPayloadString(payload, ["target_url", "url"])),
        item("method", readMonitorPayloadString(payload, ["method"])),
        item("status", readMonitorPayloadString(payload, ["status_code", "code"])),
        item("expected", expectedStatusLabel(payload)),
        item("latency", latency),
        item("final host", readMonitorPayloadString(payload, ["final_host"])),
      ];
    case "api_request":
      return [
        item("target", readMonitorPayloadString(payload, ["target_url", "url"])),
        item("method", readMonitorPayloadString(payload, ["method"])),
        item("status", readMonitorPayloadString(payload, ["status_code", "code"])),
        item("expected", expectedStatusLabel(payload)),
        item("latency", latency),
        item("assertion", readMonitorPayloadString(payload, ["assertion_path"])),
      ];
    case "tcp":
      return [
        item("host", readMonitorPayloadString(payload, ["host"])),
        item("port", readMonitorPayloadString(payload, ["port"])),
        item("address", readMonitorPayloadString(payload, ["address"])),
        item("latency", latency),
        item("connected", readMonitorPayloadString(payload, ["connected", "ok"])),
      ];
    case "dns":
      return [
        item("host", readMonitorPayloadString(payload, ["host"])),
        item("record", readMonitorPayloadString(payload, ["record_type"])),
        item("resolver", readMonitorPayloadString(payload, ["resolver"])),
        item("answers", readMonitorPayloadList(payload, ["answers"])),
        item("missing", readMonitorPayloadList(payload, ["missing_values"])),
        item("latency", latency),
      ];
    case "tls":
      return [
        item("host", readMonitorPayloadString(payload, ["host", "server_name"])),
        item("port", readMonitorPayloadString(payload, ["port"])),
        item(
          "expires",
          readMonitorPayloadString(payload, ["not_after", "tls_expiry", "certificate_expiry"]),
        ),
        item("days left", readMonitorPayloadString(payload, ["tls_days_remaining"])),
        item("chain", readMonitorPayloadString(payload, ["chain_verified"])),
        item("latency", latency),
      ];
    case "heartbeat":
      return [
        item("last signal", readMonitorPayloadString(payload, ["last_signal_at", "collected_at"])),
        item("missed after", readMonitorPayloadString(payload, ["missed_after"])),
        item("interval", readMonitorPayloadString(payload, ["interval_seconds"])),
        item("grace", readMonitorPayloadString(payload, ["grace_seconds"])),
        item("payload", readMonitorPayloadString(payload, ["payload", "message", "error"])),
      ];
    default:
      return [
        item("latency", latency),
        item("status", readMonitorPayloadString(payload, ["status", "status_code", "code"])),
        item("target", readMonitorPayloadString(payload, ["target_url", "url", "host", "address"])),
        item("failure stage", readMonitorPayloadString(payload, ["failure_stage"])),
      ];
  }
};

const stageExplanation = (stage: string, kind: string, payload: MonitorPayload) => {
  switch (stage) {
    case "body_forbidden":
      return "The response body contained text that this monitor is configured to reject.";
    case "body_required":
      return "The response body did not contain the required text.";
    case "certificate":
      return "The TLS certificate could not be verified for this host.";
    case "config":
      return "The saved monitor configuration is invalid; edit the monitor before retrying.";
    case "connect":
      return "Core could not open a network connection to the target.";
    case "dns":
    case "lookup":
      return "DNS resolution failed before the monitor could reach the target.";
    case "expected_values":
      return "DNS resolved, but one or more expected records were missing.";
    case "expired":
      return "The TLS certificate has already expired.";
    case "expiry_threshold":
      return "The TLS certificate is inside the configured expiry warning window.";
    case "http_body":
    case "response_body":
      return "Core received a response, but could not read or validate the body.";
    case "http_request":
    case "transport":
      return "Core could not complete the HTTP request to the target.";
    case "http_response":
    case "status":
      return statusHeadline(payload, kind === "api_request" ? "API" : "HTTP");
    case "json_assertion": {
      const path = readMonitorPayloadString(payload, ["assertion_path"]);
      return path
        ? `The JSON assertion at ${path} did not match.`
        : "A JSON assertion did not match.";
    }
    case "missed_signal":
      return "No heartbeat signal arrived before the interval plus grace period expired.";
    case "timeout":
      return "The check timed out before Core received a usable response.";
    default:
      return (
        readMonitorPayloadString(payload, ["failure_reason", "message", "error", "summary"]) ??
        `The ${kindLabel(kind)} check failed at ${stage}.`
      );
  }
};

export const explainMonitorFailure = (
  report?: ApiMonitorReportResponse,
  fallbackKind?: string,
): string => {
  if (!report) return "No check result has been recorded yet.";
  const payload = parseMonitorPayload(report.payload);
  const kind = monitorKind(payload, fallbackKind);
  const stage = readMonitorPayloadString(payload, ["failure_stage"]);
  const health = report.health ?? readMonitorPayloadString(payload, ["status", "health"]);
  if (health === "up") return "Latest check passed.";
  if (stage) return stageExplanation(stage, kind, payload);
  return (
    readMonitorPayloadString(payload, ["failure_reason", "message", "error", "summary"]) ??
    `Latest ${kindLabel(kind)} check reported ${health ?? "unknown"}.`
  );
};

export const summarizeMonitorResult = (
  report?: ApiMonitorReportResponse,
  fallbackKind?: string,
): MonitorResultSummary => {
  const payload = parseMonitorPayload(report?.payload);
  const kind = monitorKind(payload, fallbackKind);
  return {
    kind,
    kindLabel: kindLabel(kind),
    headline: report ? resultHeadline(kind, payload, report) : "No check result recorded",
    items: monitorResultItems(kind, payload),
    explanation: explainMonitorFailure(report, fallbackKind),
    payload,
  };
};
