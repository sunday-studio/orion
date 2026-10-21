import type { ApiMonitorReportResponse } from "@/orion-sdk";

type MonitorPayload = Record<string, unknown>;

const EMPTY_VALUE = "—";

const parseMonitorPayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const readMonitorPayloadString = (payload: MonitorPayload, keys: string[]): string | undefined => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number" && Number.isFinite(value)) return String(value);
    if (typeof value === "boolean") return value ? "yes" : "no";
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
