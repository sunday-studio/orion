import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import type {
  ApiCoreMonitorConfigResponse,
  ApiMonitorResponse,
  ServiceCoreManagedMonitorCreateRequest,
  ServiceCoreManagedMonitorUpdateRequest,
} from "@/orion-sdk";
import { Play, Save } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";

export type CoreMonitorSubmitAction = "save" | "save_test";

type CoreMonitorDialogProps = {
  config?: ApiCoreMonitorConfigResponse;
  error?: string;
  isSubmitting?: boolean;
  mode: "create" | "edit";
  monitor?: ApiMonitorResponse;
  onOpenChange: (open: boolean) => void;
  onSubmit: (
    payload: ServiceCoreManagedMonitorCreateRequest | ServiceCoreManagedMonitorUpdateRequest,
    action: CoreMonitorSubmitAction,
  ) => void;
  open: boolean;
};

type FormState = {
  advancedConfig: string;
  apiRequestMethod: string;
  confirmationCheckCount: string;
  confirmationPeriodSeconds: string;
  description: string;
  domain: string;
  expectedStatus: string;
  graceSeconds: string;
  host: string;
  intervalSeconds: string;
  kind: CoreMonitorKind;
  mailProtocol: string;
  mailTlsMode: string;
  method: string;
  name: string;
  paused: boolean;
  pingMethod: string;
  port: string;
  rdapUrl: string;
  recordType: string;
  requiredContains: string;
  recoveryPeriodSeconds: string;
  timeoutSeconds: string;
  udpExpectedResponse: string;
  udpPayload: string;
  url: string;
  warningDays: string;
  whoisServer: string;
};

type CoreMonitorKind =
  | "heartbeat"
  | "http"
  | "http_keyword"
  | "expected_status"
  | "api_request"
  | "tcp"
  | "udp"
  | "dns"
  | "tls"
  | "domain_expiration"
  | "ping"
  | "mail"
  | "smtp"
  | "imap"
  | "pop"
  | "synthetic"
  | "playwright";

const defaultForm: FormState = {
  advancedConfig: "{\n  \"steps\": []\n}",
  apiRequestMethod: "GET",
  confirmationCheckCount: "0",
  confirmationPeriodSeconds: "0",
  description: "",
  domain: "",
  expectedStatus: "200",
  graceSeconds: "60",
  host: "",
  intervalSeconds: "60",
  kind: "http",
  mailProtocol: "smtp",
  mailTlsMode: "none",
  method: "GET",
  name: "",
  paused: false,
  pingMethod: "tcp",
  port: "",
  rdapUrl: "",
  recordType: "A",
  requiredContains: "",
  recoveryPeriodSeconds: "0",
  timeoutSeconds: "10",
  udpExpectedResponse: "",
  udpPayload: "",
  url: "",
  warningDays: "30",
  whoisServer: "",
};

const coreMonitorKindOptions = [
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

const httpMethods = ["GET", "HEAD"] as const;
const apiRequestMethods = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"] as const;
const dnsRecordTypes = ["A", "AAAA", "CNAME", "TXT", "MX", "NS"] as const;
const mailProtocols = ["smtp", "imap", "pop"] as const;
const mailTlsModes = ["none", "implicit", "starttls"] as const;
const pingMethods = ["tcp", "icmp"] as const;

const isCoreMonitorKind = (value: string): value is CoreMonitorKind =>
  coreMonitorKindOptions.some((option) => option.value === value);

const normalizeKind = (kind?: string): CoreMonitorKind => {
  switch (kind) {
    case "heartbeat":
    case "http":
    case "http_keyword":
    case "expected_status":
    case "api_request":
    case "tcp":
    case "udp":
    case "dns":
    case "tls":
    case "domain_expiration":
    case "ping":
    case "mail":
    case "smtp":
    case "imap":
    case "pop":
    case "synthetic":
    case "playwright":
      return kind;
    case "pop3":
      return "pop";
    case "http_status":
      return "http";
    case "tcp_port":
      return "tcp";
    case "tls_certificate":
      return "tls";
    case "synthetic_multi_step":
      return "synthetic";
    case "playwright_transaction":
      return "playwright";
    default:
      return "http";
  }
};

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
  if (Array.isArray(value)) {
    return value
      .filter((item): item is string => typeof item === "string" && item.trim() !== "")
      .join("\n");
  }
  return "";
};

const formatConfigJSON = (config: ApiCoreMonitorConfigResponse | undefined, fallback: string) => {
  try {
    return JSON.stringify(config?.config ?? JSON.parse(fallback), null, 2);
  } catch {
    return fallback;
  }
};

const parseJSONConfig = (value: string) => {
  const parsed = JSON.parse(value);
  return parsed && typeof parsed === "object" && !Array.isArray(parsed)
    ? (parsed as Record<string, unknown>)
    : undefined;
};

const toPositiveInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};

const toNonNegativeInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
};

export const CoreMonitorDialog = ({
  config,
  error,
  isSubmitting = false,
  mode,
  monitor,
  onOpenChange,
  onSubmit,
  open,
}: CoreMonitorDialogProps) => {
  const [form, setForm] = useState<FormState>(defaultForm);
  const [submitAction, setSubmitAction] = useState<CoreMonitorSubmitAction>("save");

  useEffect(() => {
    if (!open) return;
    if (mode === "create") {
      setForm(defaultForm);
      return;
    }
    const kind = normalizeKind(config?.kind);
    setForm({
      advancedConfig: formatConfigJSON(
        config,
        kind === "playwright"
          ? "{\n  \"url\": \"https://example.com\",\n  \"browser\": \"chromium\",\n  \"steps\": []\n}"
          : "{\n  \"steps\": []\n}",
      ),
      apiRequestMethod: readConfigString(config, "method") || "GET",
      confirmationCheckCount: String(config?.confirmation_check_count ?? 0),
      confirmationPeriodSeconds: String(config?.confirmation_period_seconds ?? 0),
      description: monitor?.description ?? "",
      domain: readConfigString(config, "domain"),
      expectedStatus: readConfigNumber(config, "expected_status") || "200",
      graceSeconds: readConfigNumber(config, "grace_seconds") || "60",
      host: readConfigString(config, "host"),
      intervalSeconds: String(
        config?.interval_seconds ?? monitor?.reporting_interval_seconds ?? 60,
      ),
      kind,
      mailProtocol: readConfigString(config, "protocol") || (kind === "mail" ? "smtp" : kind),
      mailTlsMode: readConfigString(config, "tls_mode") || "none",
      method: readConfigString(config, "method") || "GET",
      name: monitor?.name ?? "",
      paused: config?.paused ?? false,
      pingMethod: readConfigString(config, "method") || "tcp",
      port: readConfigNumber(config, "port"),
      rdapUrl: readConfigString(config, "rdap_url"),
      recordType: readConfigString(config, "record_type") || "A",
      requiredContains: readConfigStringList(config, "required_contains"),
      recoveryPeriodSeconds: String(config?.recovery_period_seconds ?? 0),
      timeoutSeconds: String(config?.timeout_seconds ?? 10),
      udpExpectedResponse: readConfigString(config, "expected_response"),
      udpPayload: readConfigString(config, "payload"),
      url: readConfigString(config, "url"),
      warningDays: readConfigNumber(config, "warning_days") || "30",
      whoisServer: readConfigString(config, "whois_server"),
    });
  }, [config, mode, monitor, open]);

  const updateForm = (patch: Partial<FormState>) =>
    setForm((current) => ({ ...current, ...patch }));

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const expectedStatus = toPositiveInt(form.expectedStatus, 200);
    const isHeartbeat = form.kind === "heartbeat";
    const requiredContains = form.requiredContains
      .split(/\n|,/)
      .map((value) => value.trim())
      .filter(Boolean);
    const configPayload = (() => {
      switch (form.kind) {
        case "heartbeat":
          return { grace_seconds: toPositiveInt(form.graceSeconds, 60) };
        case "http":
        case "http_keyword":
        case "expected_status":
          return {
            expected_status: expectedStatus,
            method: form.method,
            ...(form.kind === "http_keyword" && requiredContains.length > 0
              ? { required_contains: requiredContains }
              : {}),
            url: form.url.trim(),
          };
        case "api_request":
          return {
            expected_status: expectedStatus,
            method: form.apiRequestMethod,
            url: form.url.trim(),
          };
        case "tcp":
          return { host: form.host.trim(), port: toPositiveInt(form.port, 443) };
        case "udp":
          return {
            expected_response: form.udpExpectedResponse,
            host: form.host.trim(),
            payload: form.udpPayload,
            port: toPositiveInt(form.port, 53),
          };
        case "dns":
          return { host: form.host.trim(), record_type: form.recordType };
        case "tls":
          return {
            host: form.host.trim(),
            ...(form.port.trim() ? { port: toPositiveInt(form.port, 443) } : {}),
            warning_days: toNonNegativeInt(form.warningDays, 30),
          };
        case "domain_expiration":
          return {
            domain: form.domain.trim(),
            ...(form.rdapUrl.trim() ? { rdap_url: form.rdapUrl.trim() } : {}),
            warning_days: toNonNegativeInt(form.warningDays, 30),
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
          return parseJSONConfig(form.advancedConfig);
      }
    })();

    if (!configPayload) return;
    const payload = {
      config: configPayload,
      description: form.description.trim() || undefined,
      confirmation_check_count: toNonNegativeInt(form.confirmationCheckCount, 0),
      confirmation_period_seconds: toNonNegativeInt(form.confirmationPeriodSeconds, 0),
      interval_seconds: toPositiveInt(form.intervalSeconds, 60),
      kind: form.kind,
      name: form.name.trim(),
      paused: form.paused,
      recovery_period_seconds: toNonNegativeInt(form.recoveryPeriodSeconds, 0),
      ...(isHeartbeat ? {} : { timeout_seconds: toPositiveInt(form.timeoutSeconds, 10) }),
      type: form.kind,
    };
    onSubmit(payload, submitAction);
  };

  const title = mode === "create" ? "Create Core Monitor" : "Edit Core Monitor";
  const description =
    mode === "create"
      ? "Add a check that runs from Orion Core."
      : "Update the Core-owned check configuration.";
  const isHeartbeat = form.kind === "heartbeat";
  const usesUrl = ["http", "http_keyword", "expected_status", "api_request"].includes(form.kind);
  const usesHost = ["tcp", "udp", "dns", "tls", "ping", "mail", "smtp", "imap", "pop"].includes(
    form.kind,
  );
  const usesDomain = form.kind === "domain_expiration";
  const usesAdvancedJSON = form.kind === "synthetic" || form.kind === "playwright";
  const advancedConfigError = (() => {
    if (!usesAdvancedJSON) return "";
    try {
      return parseJSONConfig(form.advancedConfig) ? "" : "Configuration JSON must be an object.";
    } catch (error) {
      return error instanceof Error ? error.message : "Configuration JSON is invalid.";
    }
  })();
  const canSubmit =
    form.name.trim() &&
    !advancedConfigError &&
    (isHeartbeat ||
      (usesUrl && form.url.trim()) ||
      (usesHost && form.host.trim()) ||
      (usesDomain && form.domain.trim()) ||
      usesAdvancedJSON);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <form className="space-y-5" onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 sm:grid-cols-2">
            <label className="space-y-1 text-sm">
              <span className="font-medium">Name</span>
              <Input
                required
                value={form.name}
                onChange={(event) => updateForm({ name: event.target.value })}
                placeholder="Core public API"
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-medium">Type</span>
              <Select
                value={form.kind}
                onValueChange={(value) => {
                  if (isCoreMonitorKind(value)) {
                    updateForm({ kind: value });
                  }
                }}
              >
                <SelectTrigger className="w-full">
                  <span data-slot="select-value">
                    {coreMonitorKindOptions.find((option) => option.value === form.kind)?.label}
                  </span>
                </SelectTrigger>
                <SelectContent>
                  {coreMonitorKindOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <select
                className="sr-only"
                aria-label="Core monitor type"
                value={form.kind}
                onChange={(event) => {
                  const value = event.target.value;
                  if (isCoreMonitorKind(value)) {
                    updateForm({ kind: value });
                  }
                }}
              >
                {coreMonitorKindOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            {usesUrl && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">URL</span>
                  <Input
                    required
                    type="url"
                    value={form.url}
                    onChange={(event) => updateForm({ url: event.target.value })}
                    placeholder="https://example.com/health"
                  />
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Method</span>
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={form.kind === "api_request" ? form.apiRequestMethod : form.method}
                    onChange={(event) =>
                      form.kind === "api_request"
                        ? updateForm({ apiRequestMethod: event.target.value })
                        : updateForm({ method: event.target.value })
                    }
                  >
                    {(form.kind === "api_request" ? apiRequestMethods : httpMethods).map(
                      (method) => (
                        <option key={method} value={method}>
                          {method}
                        </option>
                      ),
                    )}
                  </select>
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Expected status</span>
                  <Input
                    inputMode="numeric"
                    min={100}
                    max={599}
                    type="number"
                    value={form.expectedStatus}
                    onChange={(event) => updateForm({ expectedStatus: event.target.value })}
                  />
                </label>
              </>
            )}
            {usesHost && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Host</span>
                  <Input
                    required
                    value={form.host}
                    onChange={(event) => updateForm({ host: event.target.value })}
                    placeholder="example.com"
                  />
                </label>
                {form.kind !== "dns" && (
                  <label className="space-y-1 text-sm">
                    <span className="font-medium">Port</span>
                    <Input
                      inputMode="numeric"
                      min={1}
                      max={65535}
                      type="number"
                      value={form.port}
                      onChange={(event) => updateForm({ port: event.target.value })}
                      placeholder={
                        form.kind === "udp"
                          ? "53"
                          : form.kind === "smtp"
                            ? "25"
                            : form.kind === "imap"
                              ? "143"
                              : form.kind === "pop"
                                ? "110"
                                : "443"
                      }
                    />
                  </label>
                )}
              </>
            )}
            {form.kind === "udp" && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Payload</span>
                  <Input
                    required
                    value={form.udpPayload}
                    onChange={(event) => updateForm({ udpPayload: event.target.value })}
                    placeholder="ping"
                  />
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Expected response</span>
                  <Input
                    required
                    value={form.udpExpectedResponse}
                    onChange={(event) => updateForm({ udpExpectedResponse: event.target.value })}
                    placeholder="pong"
                  />
                </label>
              </>
            )}
            {form.kind === "dns" && (
              <label className="space-y-1 text-sm">
                <span className="font-medium">Record type</span>
                <select
                  className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                  value={form.recordType}
                  onChange={(event) => updateForm({ recordType: event.target.value })}
                >
                  {dnsRecordTypes.map((recordType) => (
                    <option key={recordType} value={recordType}>
                      {recordType}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {form.kind === "ping" && (
              <label className="space-y-1 text-sm">
                <span className="font-medium">Method</span>
                <select
                  className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                  value={form.pingMethod}
                  onChange={(event) => updateForm({ pingMethod: event.target.value })}
                >
                  {pingMethods.map((method) => (
                    <option key={method} value={method}>
                      {method.toUpperCase()}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {["mail", "smtp", "imap", "pop"].includes(form.kind) && (
              <>
                {form.kind === "mail" && (
                  <label className="space-y-1 text-sm">
                    <span className="font-medium">Protocol</span>
                    <select
                      className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                      value={form.mailProtocol}
                      onChange={(event) => updateForm({ mailProtocol: event.target.value })}
                    >
                      {mailProtocols.map((protocol) => (
                        <option key={protocol} value={protocol}>
                          {protocol.toUpperCase()}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
                <label className="space-y-1 text-sm">
                  <span className="font-medium">TLS mode</span>
                  <select
                    className="h-9 w-full border border-neutral-200 bg-white px-3 text-sm"
                    value={form.mailTlsMode}
                    onChange={(event) => updateForm({ mailTlsMode: event.target.value })}
                  >
                    {mailTlsModes.map((tlsMode) => (
                      <option key={tlsMode} value={tlsMode}>
                        {tlsMode}
                      </option>
                    ))}
                  </select>
                </label>
              </>
            )}
            {(form.kind === "tls" || form.kind === "domain_expiration") && (
              <label className="space-y-1 text-sm">
                <span className="font-medium">Warning days</span>
                <Input
                  inputMode="numeric"
                  min={0}
                  type="number"
                  value={form.warningDays}
                  onChange={(event) => updateForm({ warningDays: event.target.value })}
                />
              </label>
            )}
            {form.kind === "domain_expiration" && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Domain</span>
                  <Input
                    required
                    value={form.domain}
                    onChange={(event) => updateForm({ domain: event.target.value })}
                    placeholder="example.com"
                  />
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">RDAP URL</span>
                  <Input
                    type="url"
                    value={form.rdapUrl}
                    onChange={(event) => updateForm({ rdapUrl: event.target.value })}
                    placeholder="https://rdap.example.com/domain/example.com"
                  />
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">WHOIS server</span>
                  <Input
                    value={form.whoisServer}
                    onChange={(event) => updateForm({ whoisServer: event.target.value })}
                    placeholder="whois.example.com:43"
                  />
                </label>
              </>
            )}
            {usesAdvancedJSON && (
              <label className="space-y-1 text-sm sm:col-span-2">
                <span className="font-medium">Configuration JSON</span>
                <Textarea
                  value={form.advancedConfig}
                  onChange={(event) => updateForm({ advancedConfig: event.target.value })}
                  rows={10}
                  spellCheck={false}
                />
                {advancedConfigError && (
                  <span className="block text-rose-700">{advancedConfigError}</span>
                )}
              </label>
            )}
            <label className="space-y-1 text-sm">
              <span className="font-medium">Interval seconds</span>
              <Input
                inputMode="numeric"
                min={10}
                type="number"
                value={form.intervalSeconds}
                onChange={(event) => updateForm({ intervalSeconds: event.target.value })}
              />
            </label>
            {isHeartbeat ? (
              <label className="space-y-1 text-sm">
                <span className="font-medium">Grace seconds</span>
                <Input
                  inputMode="numeric"
                  min={0}
                  type="number"
                  value={form.graceSeconds}
                  onChange={(event) => updateForm({ graceSeconds: event.target.value })}
                />
              </label>
            ) : (
              <label className="space-y-1 text-sm">
                <span className="font-medium">Timeout seconds</span>
                <Input
                  inputMode="numeric"
                  min={1}
                  type="number"
                  value={form.timeoutSeconds}
                  onChange={(event) => updateForm({ timeoutSeconds: event.target.value })}
                />
              </label>
            )}
            <label className="space-y-1 text-sm">
              <span className="font-medium">Confirmation seconds</span>
              <Input
                inputMode="numeric"
                min={0}
                type="number"
                value={form.confirmationPeriodSeconds}
                onChange={(event) => updateForm({ confirmationPeriodSeconds: event.target.value })}
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-medium">Confirmation checks</span>
              <Input
                inputMode="numeric"
                min={0}
                type="number"
                value={form.confirmationCheckCount}
                onChange={(event) => updateForm({ confirmationCheckCount: event.target.value })}
              />
            </label>
            <label className="space-y-1 text-sm">
              <span className="font-medium">Recovery seconds</span>
              <Input
                inputMode="numeric"
                min={0}
                type="number"
                value={form.recoveryPeriodSeconds}
                onChange={(event) => updateForm({ recoveryPeriodSeconds: event.target.value })}
              />
            </label>
            <label className="flex items-center gap-2 pt-7 text-sm">
              <Checkbox
                checked={form.paused}
                onCheckedChange={(checked) => updateForm({ paused: checked === true })}
              />
              Start paused
            </label>
            {form.kind === "http_keyword" && (
              <label className="space-y-1 text-sm sm:col-span-2">
                <span className="font-medium">Required response text</span>
                <Textarea
                  value={form.requiredContains}
                  onChange={(event) => updateForm({ requiredContains: event.target.value })}
                  placeholder="One keyword per line"
                />
              </label>
            )}
            <label className="space-y-1 text-sm sm:col-span-2">
              <span className="font-medium">Description</span>
              <Textarea
                value={form.description}
                onChange={(event) => updateForm({ description: event.target.value })}
                placeholder="What this monitor protects"
              />
            </label>
          </div>

          {error && (
            <div className="border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-800">
              {error}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              disabled={isSubmitting || !canSubmit}
              type="submit"
              onClick={() => setSubmitAction("save")}
            >
              <Save />
              {mode === "create" ? "Create" : "Save"}
            </Button>
            {mode === "create" && !isHeartbeat && (
              <Button
                disabled={isSubmitting || !canSubmit}
                type="submit"
                onClick={() => setSubmitAction("save_test")}
              >
                <Play />
                Create and test
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
