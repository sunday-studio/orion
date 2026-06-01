import type {
  ApiCoreMonitorConfigResponse,
  ApiMonitorResponse,
} from "@/orion-sdk";

export type CoreMonitorSubmitAction = "save" | "save_test";

export type CoreMonitorKind =
  | "heartbeat"
  | "http"
  | "http_keyword"
  | "expected_status"
  | "tcp"
  | "udp"
  | "dns"
  | "tls"
  | "api_request"
  | "domain_expiration"
  | "ping"
  | "mail"
  | "smtp"
  | "imap"
  | "pop"
  | "synthetic"
  | "playwright";

export type FormState = {
  advancedConfig: string;
  apiBody: string;
  apiHeaders: string;
  apiJSONAssertions: string;
  apiMethod: "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD" | "OPTIONS";
  confirmationCheckCount: string;
  confirmationPeriodSeconds: string;
  description: string;
  domain: string;
  expectedStatus: string;
  expectedStatuses: string;
  expectedValues: string;
  graceSeconds: string;
  host: string;
  intervalSeconds: string;
  kind: CoreMonitorKind;
  mailProtocol: string;
  mailTlsMode: string;
  name: string;
  paused: boolean;
  pingMethod: string;
  port: string;
  rdapUrl: string;
  recordType: "A" | "AAAA" | "CNAME" | "TXT" | "MX" | "NS";
  requiredContains: string;
  recoveryPeriodSeconds: string;
  serverName: string;
  timeoutSeconds: string;
  udpExpectedResponse: string;
  udpPayload: string;
  url: string;
  warningDays: string;
  whoisServer: string;
};

export const defaultForm: FormState = {
  advancedConfig: '{\n  "steps": []\n}',
  apiBody: "",
  apiHeaders: "",
  apiJSONAssertions: "",
  apiMethod: "GET",
  confirmationCheckCount: "0",
  confirmationPeriodSeconds: "0",
  description: "",
  domain: "",
  expectedStatus: "200",
  expectedStatuses: "",
  expectedValues: "",
  graceSeconds: "60",
  host: "",
  intervalSeconds: "60",
  kind: "http",
  mailProtocol: "smtp",
  mailTlsMode: "none",
  name: "",
  paused: false,
  pingMethod: "tcp",
  port: "",
  rdapUrl: "",
  recordType: "A",
  requiredContains: "",
  recoveryPeriodSeconds: "0",
  serverName: "",
  timeoutSeconds: "10",
  udpExpectedResponse: "",
  udpPayload: "",
  url: "",
  warningDays: "14",
  whoisServer: "",
};

export const coreMonitorKindOptions = [
  { value: "http", label: "HTTP status" },
  { value: "http_keyword", label: "HTTP keyword" },
  { value: "expected_status", label: "Expected status" },
  { value: "api_request", label: "API request" },
  { value: "tcp", label: "TCP port" },
  { value: "udp", label: "UDP response" },
  { value: "dns", label: "DNS record" },
  { value: "tls", label: "TLS certificate" },
  { value: "domain_expiration", label: "Domain expiration" },
  { value: "ping", label: "Ping" },
  { value: "smtp", label: "SMTP" },
  { value: "imap", label: "IMAP" },
  { value: "pop", label: "POP3" },
  { value: "mail", label: "Mail protocol" },
  { value: "synthetic", label: "Synthetic transaction" },
  { value: "playwright", label: "Browser journey" },
  { value: "heartbeat", label: "Heartbeat" },
] as const;

export const apiMethodOptions = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"] as const;
export const dnsRecordTypeOptions = ["A", "AAAA", "CNAME", "TXT", "MX", "NS"] as const;
export const mailProtocols = ["smtp", "imap", "pop"] as const;
export const mailTlsModes = ["none", "implicit", "starttls"] as const;
export const pingMethods = ["tcp", "icmp"] as const;

export const isCoreMonitorKind = (value: string): value is CoreMonitorKind =>
  coreMonitorKindOptions.some((option) => option.value === value);

export const normalizeKind = (kind?: string): CoreMonitorKind => {
  if (!kind) return "http";
  if (isCoreMonitorKind(kind)) return kind;
  if (kind === "pop3") return "pop";
  if (kind === "http_status") return "http";
  if (kind === "tcp_port") return "tcp";
  if (kind === "tls_certificate") return "tls";
  if (kind === "synthetic_multi_step") return "synthetic";
  if (kind === "playwright_transaction") return "playwright";
  return "http";
};

export const isAPIMethod = (value: string): value is FormState["apiMethod"] =>
  apiMethodOptions.includes(value as FormState["apiMethod"]);

export const isDNSRecordType = (value: string): value is FormState["recordType"] =>
  dnsRecordTypeOptions.includes(value as FormState["recordType"]);

const readConfigString = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (typeof value === "string") return value;
  if (typeof value === "number") return String(value);
  return "";
};

const readConfigNumber = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (typeof value === "number") return String(value);
  if (typeof value === "string" && value.trim() !== "") return value;
  return "";
};

const readConfigStringList = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (!Array.isArray(value)) return "";
  return value
    .filter((item): item is string => typeof item === "string" && item.trim() !== "")
    .join("\n");
};

const readConfigIntList = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (!Array.isArray(value)) return "";
  return value
    .filter((item): item is number => typeof item === "number" && Number.isInteger(item))
    .join(", ");
};

const readConfigObjectEntries = (
  config: ApiCoreMonitorConfigResponse | undefined,
  key: string,
) => {
  const value = config?.config?.[key];
  if (typeof value !== "object" || value === null || Array.isArray(value)) return "";
  return Object.entries(value)
    .filter((entry): entry is [string, string] => typeof entry[1] === "string")
    .map(([header, headerValue]) => `${header}: ${headerValue}`)
    .join("\n");
};

const readConfigJSON = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (value === undefined || value === null) return "";
  return JSON.stringify(value, null, 2);
};

export const toPositiveInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};

export const toNonNegativeInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
};

const parseIntList = (value: string, fieldName: string) => {
  const values = value
    .split(/\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);
  if (values.some((item) => !/^\d+$/.test(item))) {
    throw new Error(`${fieldName} must contain only whole numbers.`);
  }
  return values.map((item) => Number.parseInt(item, 10));
};

const parseStringList = (value: string) =>
  value
    .split(/\n|,/)
    .map((item) => item.trim())
    .filter(Boolean);

const parseHeaderMap = (value: string) => {
  const headers: Record<string, string> = {};
  for (const rawLine of value.split("\n")) {
    const line = rawLine.trim();
    if (!line) continue;
    const separatorIndex = line.includes(":") ? line.indexOf(":") : line.indexOf("=");
    if (separatorIndex < 1) throw new Error("Headers must use Name: value lines.");
    const key = line.slice(0, separatorIndex).trim();
    const headerValue = line.slice(separatorIndex + 1).trim();
    if (!key) throw new Error("Header names are required.");
    headers[key] = headerValue;
  }
  return headers;
};

const parseJSONAssertions = (value: string) => {
  if (!value.trim()) return undefined;
  const parsed = JSON.parse(value) as unknown;
  if (!Array.isArray(parsed)) throw new Error("JSON assertions must be a JSON array.");
  return parsed;
};

const formatConfigJSON = (config: ApiCoreMonitorConfigResponse | undefined, fallback: string) => {
  try {
    return JSON.stringify(config?.config ?? JSON.parse(fallback), null, 2);
  } catch {
    return fallback;
  }
};

export const parseJSONConfig = (value: string) => {
  try {
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : undefined;
  } catch {
    return undefined;
  }
};

export const formFromMonitor = (
  config: ApiCoreMonitorConfigResponse | undefined,
  monitor: ApiMonitorResponse | undefined,
) => {
  const kind = normalizeKind(config?.kind);
  const method = readConfigString(config, "method").toUpperCase();
  const recordType = readConfigString(config, "record_type").toUpperCase();
  return {
    advancedConfig: formatConfigJSON(config, defaultForm.advancedConfig),
    apiBody: readConfigString(config, "body"),
    apiHeaders: readConfigObjectEntries(config, "headers"),
    apiJSONAssertions: readConfigJSON(config, "json_assertions"),
    apiMethod: isAPIMethod(method) ? method : "GET",
    confirmationCheckCount: String(config?.confirmation_check_count ?? 0),
    confirmationPeriodSeconds: String(config?.confirmation_period_seconds ?? 0),
    description: monitor?.description ?? "",
    domain: readConfigString(config, "domain"),
    expectedStatus: readConfigNumber(config, "expected_status") || "200",
    expectedStatuses: readConfigIntList(config, "expected_statuses"),
    expectedValues: readConfigStringList(config, "expected_values"),
    graceSeconds: readConfigNumber(config, "grace_seconds") || "60",
    host: readConfigString(config, "host"),
    intervalSeconds: String(config?.interval_seconds ?? monitor?.reporting_interval_seconds ?? 60),
    kind,
    mailProtocol: readConfigString(config, "protocol") || (kind === "mail" ? "smtp" : kind),
    mailTlsMode: readConfigString(config, "tls_mode") || "none",
    name: monitor?.name ?? "",
    paused: config?.paused ?? false,
    pingMethod: readConfigString(config, "method") || "tcp",
    port: readConfigNumber(config, "port"),
    rdapUrl: readConfigString(config, "rdap_url"),
    recordType: isDNSRecordType(recordType) ? recordType : "A",
    requiredContains: readConfigStringList(config, "required_contains"),
    recoveryPeriodSeconds: String(config?.recovery_period_seconds ?? 0),
    serverName: readConfigString(config, "server_name"),
    timeoutSeconds: String(config?.timeout_seconds ?? 10),
    udpExpectedResponse: readConfigString(config, "expected_response"),
    udpPayload: readConfigString(config, "payload"),
    url: readConfigString(config, "url"),
    warningDays: readConfigNumber(config, "warning_days") || "14",
    whoisServer: readConfigString(config, "whois_server"),
  } satisfies FormState;
};

export const buildConfigPayload = (form: FormState): Record<string, unknown> => {
  switch (form.kind) {
    case "heartbeat":
      return { grace_seconds: toPositiveInt(form.graceSeconds, 60) };
    case "http":
    case "http_keyword":
    case "expected_status": {
      const expectedStatus = toPositiveInt(form.expectedStatus, 200);
      const expectedStatuses = parseIntList(form.expectedStatuses, "Expected statuses");
      const requiredContains = parseStringList(form.requiredContains);
      return {
        expected_status: expectedStatus,
        ...(expectedStatuses.length > 0 ? { expected_statuses: expectedStatuses } : {}),
        ...(form.kind === "http_keyword" && requiredContains.length > 0
          ? { required_contains: requiredContains }
          : {}),
        url: form.url.trim(),
      };
    }
    case "api_request": {
      const expectedStatus = toPositiveInt(form.expectedStatus, 200);
      const expectedStatuses = parseIntList(form.expectedStatuses, "Expected statuses");
      const headers = parseHeaderMap(form.apiHeaders);
      const jsonAssertions = parseJSONAssertions(form.apiJSONAssertions);
      return {
        body: form.apiBody,
        expected_status: expectedStatus,
        ...(expectedStatuses.length > 0 ? { expected_statuses: expectedStatuses } : {}),
        ...(Object.keys(headers).length > 0 ? { headers } : {}),
        ...(jsonAssertions ? { json_assertions: jsonAssertions } : {}),
        method: form.apiMethod,
        url: form.url.trim(),
      };
    }
    case "tcp":
      return { host: form.host.trim(), port: toPositiveInt(form.port, 0) };
    case "udp":
      return {
        expected_response: form.udpExpectedResponse,
        host: form.host.trim(),
        payload: form.udpPayload,
        port: toPositiveInt(form.port, 53),
      };
    case "dns": {
      const expectedValues = parseStringList(form.expectedValues);
      return {
        ...(expectedValues.length > 0 ? { expected_values: expectedValues } : {}),
        host: form.host.trim(),
        record_type: form.recordType,
      };
    }
    case "tls":
      return {
        host: form.host.trim(),
        ...(form.port.trim() ? { port: toPositiveInt(form.port, 443) } : {}),
        ...(form.serverName.trim() ? { server_name: form.serverName.trim() } : {}),
        warning_days: toNonNegativeInt(form.warningDays, 14),
      };
    case "domain_expiration":
      return {
        domain: form.domain.trim(),
        ...(form.rdapUrl.trim() ? { rdap_url: form.rdapUrl.trim() } : {}),
        warning_days: toNonNegativeInt(form.warningDays, 14),
        ...(form.whoisServer.trim() ? { whois_server: form.whoisServer.trim() } : {}),
      };
    case "ping":
      return {
        host: form.host.trim(),
        method: form.pingMethod,
        ...(form.pingMethod === "tcp" && form.port.trim()
          ? { port: toPositiveInt(form.port, 443) }
          : {}),
      };
    case "mail":
    case "smtp":
    case "imap":
    case "pop":
      return {
        host: form.host.trim(),
        ...(form.kind === "mail" ? { protocol: form.mailProtocol } : {}),
        ...(form.port.trim() ? { port: toPositiveInt(form.port, 25) } : {}),
        tls_mode: form.mailTlsMode,
      };
    case "synthetic":
    case "playwright":
      return parseJSONConfig(form.advancedConfig) ?? {};
  }
};
