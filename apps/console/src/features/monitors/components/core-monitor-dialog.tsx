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
  apiBody: string;
  apiHeaders: string;
  apiJSONAssertions: string;
  apiMethod: "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD" | "OPTIONS";
  confirmationCheckCount: string;
  confirmationPeriodSeconds: string;
  description: string;
  expectedStatus: string;
  expectedStatuses: string;
  expectedValues: string;
  graceSeconds: string;
  host: string;
  intervalSeconds: string;
  kind: CoreMonitorKind;
  name: string;
  paused: boolean;
  port: string;
  recordType: "A" | "AAAA" | "CNAME" | "TXT" | "MX" | "NS";
  requiredContains: string;
  recoveryPeriodSeconds: string;
  serverName: string;
  timeoutSeconds: string;
  url: string;
  warningDays: string;
};

type CoreMonitorKind =
  | "heartbeat"
  | "http"
  | "http_keyword"
  | "tcp"
  | "dns"
  | "tls"
  | "api_request";

const defaultForm: FormState = {
  apiBody: "",
  apiHeaders: "",
  apiJSONAssertions: "",
  apiMethod: "GET",
  confirmationCheckCount: "0",
  confirmationPeriodSeconds: "0",
  description: "",
  expectedStatus: "200",
  expectedStatuses: "",
  expectedValues: "",
  graceSeconds: "60",
  host: "",
  intervalSeconds: "60",
  kind: "http",
  name: "",
  paused: false,
  port: "",
  recordType: "A",
  requiredContains: "",
  recoveryPeriodSeconds: "0",
  serverName: "",
  timeoutSeconds: "10",
  url: "",
  warningDays: "14",
};

const coreMonitorKindOptions = [
  { value: "http", label: "HTTP status" },
  { value: "http_keyword", label: "HTTP keyword" },
  { value: "api_request", label: "API request" },
  { value: "tcp", label: "TCP port" },
  { value: "dns", label: "DNS" },
  { value: "tls", label: "TLS certificate" },
  { value: "heartbeat", label: "Heartbeat" },
] as const;

const apiMethodOptions = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"] as const;
const dnsRecordTypeOptions = ["A", "AAAA", "CNAME", "TXT", "MX", "NS"] as const;

const isCoreMonitorKind = (value: string): value is CoreMonitorKind =>
  coreMonitorKindOptions.some((option) => option.value === value);

const isAPIMethod = (value: string): value is FormState["apiMethod"] =>
  apiMethodOptions.includes(value as FormState["apiMethod"]);

const isDNSRecordType = (value: string): value is FormState["recordType"] =>
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
  if (Array.isArray(value)) {
    return value
      .filter((item): item is string => typeof item === "string" && item.trim() !== "")
      .join("\n");
  }
  return "";
};

const readConfigIntList = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
  const value = config?.config?.[key];
  if (Array.isArray(value)) {
    return value
      .filter((item): item is number => typeof item === "number" && Number.isInteger(item))
      .join(", ");
  }
  return "";
};

const readConfigObjectEntries = (config: ApiCoreMonitorConfigResponse | undefined, key: string) => {
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

const toPositiveInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
};

const toNonNegativeInt = (value: string, fallback: number) => {
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
    if (separatorIndex < 1) {
      throw new Error("Headers must use Name: value lines.");
    }
    const key = line.slice(0, separatorIndex).trim();
    const headerValue = line.slice(separatorIndex + 1).trim();
    if (!key) {
      throw new Error("Header names are required.");
    }
    headers[key] = headerValue;
  }
  return headers;
};

const parseJSONAssertions = (value: string) => {
  if (!value.trim()) return undefined;
  const parsed = JSON.parse(value) as unknown;
  if (!Array.isArray(parsed)) {
    throw new Error("JSON assertions must be a JSON array.");
  }
  return parsed;
};

const buildConfigPayload = (form: FormState): Record<string, unknown> => {
  switch (form.kind) {
    case "heartbeat":
      return {
        grace_seconds: toPositiveInt(form.graceSeconds, 60),
      };
    case "http": {
      const expectedStatus = toPositiveInt(form.expectedStatus, 200);
      const expectedStatuses = parseIntList(form.expectedStatuses, "Expected statuses");
      return {
        expected_status: expectedStatus,
        ...(expectedStatuses.length > 0 ? { expected_statuses: expectedStatuses } : {}),
        url: form.url.trim(),
      };
    }
    case "http_keyword": {
      const expectedStatus = toPositiveInt(form.expectedStatus, 200);
      const expectedStatuses = parseIntList(form.expectedStatuses, "Expected statuses");
      const requiredContains = parseStringList(form.requiredContains);
      return {
        expected_status: expectedStatus,
        ...(expectedStatuses.length > 0 ? { expected_statuses: expectedStatuses } : {}),
        ...(requiredContains.length > 0 ? { required_contains: requiredContains } : {}),
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
      return {
        host: form.host.trim(),
        port: toPositiveInt(form.port, 0),
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
  }
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
  const [localError, setLocalError] = useState("");
  const [submitAction, setSubmitAction] = useState<CoreMonitorSubmitAction>("save");

  useEffect(() => {
    if (!open) return;
    setLocalError("");
    if (mode === "create") {
      setForm(defaultForm);
      return;
    }
    const kind = isCoreMonitorKind(config?.kind ?? "") ? (config?.kind as CoreMonitorKind) : "http";
    const method = readConfigString(config, "method").toUpperCase();
    const recordType = readConfigString(config, "record_type").toUpperCase();
    setForm({
      apiBody: readConfigString(config, "body"),
      apiHeaders: readConfigObjectEntries(config, "headers"),
      apiJSONAssertions: readConfigJSON(config, "json_assertions"),
      apiMethod: isAPIMethod(method) ? method : "GET",
      confirmationCheckCount: String(config?.confirmation_check_count ?? 0),
      confirmationPeriodSeconds: String(config?.confirmation_period_seconds ?? 0),
      description: monitor?.description ?? "",
      expectedStatus: readConfigNumber(config, "expected_status") || "200",
      expectedStatuses: readConfigIntList(config, "expected_statuses"),
      expectedValues: readConfigStringList(config, "expected_values"),
      graceSeconds: readConfigNumber(config, "grace_seconds") || "60",
      host: readConfigString(config, "host"),
      intervalSeconds: String(
        config?.interval_seconds ?? monitor?.reporting_interval_seconds ?? 60,
      ),
      kind,
      name: monitor?.name ?? "",
      paused: config?.paused ?? false,
      port: readConfigNumber(config, "port"),
      recordType: isDNSRecordType(recordType) ? recordType : "A",
      requiredContains: readConfigStringList(config, "required_contains"),
      recoveryPeriodSeconds: String(config?.recovery_period_seconds ?? 0),
      serverName: readConfigString(config, "server_name"),
      timeoutSeconds: String(config?.timeout_seconds ?? 10),
      url: readConfigString(config, "url"),
      warningDays: readConfigNumber(config, "warning_days") || "14",
    });
  }, [config, mode, monitor, open]);

  const updateForm = (patch: Partial<FormState>) =>
    setForm((current) => ({ ...current, ...patch }));

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    try {
      const configPayload = buildConfigPayload(form);
      setLocalError("");
      const isHeartbeat = form.kind === "heartbeat";
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
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : "Monitor configuration is invalid.");
    }
  };

  const title = mode === "create" ? "Create Core Monitor" : "Edit Core Monitor";
  const description =
    mode === "create"
      ? "Add a check that runs from Orion Core."
      : "Update the Core-owned check configuration.";
  const isHeartbeat = form.kind === "heartbeat";
  const isURLMonitor =
    form.kind === "http" || form.kind === "http_keyword" || form.kind === "api_request";
  const isHostMonitor = form.kind === "tcp" || form.kind === "dns" || form.kind === "tls";
  const requiresPort = form.kind === "tcp";
  const canSubmit =
    form.name.trim() &&
    (isHeartbeat ||
      (isURLMonitor && form.url.trim()) ||
      (isHostMonitor && form.host.trim() && (!requiresPort || form.port.trim())));
  const visibleError = localError || error;

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
            {isURLMonitor && (
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
            {form.kind === "api_request" && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Method</span>
                  <Select
                    value={form.apiMethod}
                    onValueChange={(value) => {
                      if (isAPIMethod(value)) updateForm({ apiMethod: value });
                    }}
                  >
                    <SelectTrigger className="w-full">
                      <span data-slot="select-value">{form.apiMethod}</span>
                    </SelectTrigger>
                    <SelectContent>
                      {apiMethodOptions.map((method) => (
                        <SelectItem key={method} value={method}>
                          {method}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </label>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Expected statuses</span>
                  <Input
                    value={form.expectedStatuses}
                    onChange={(event) => updateForm({ expectedStatuses: event.target.value })}
                    placeholder="200, 201"
                  />
                </label>
                <label className="space-y-1 text-sm sm:col-span-2">
                  <span className="font-medium">Headers</span>
                  <Textarea
                    value={form.apiHeaders}
                    onChange={(event) => updateForm({ apiHeaders: event.target.value })}
                    placeholder="X-Trace: trace-1"
                  />
                </label>
                <label className="space-y-1 text-sm sm:col-span-2">
                  <span className="font-medium">Body</span>
                  <Textarea
                    value={form.apiBody}
                    onChange={(event) => updateForm({ apiBody: event.target.value })}
                    placeholder='{"status":"check"}'
                  />
                </label>
                <label className="space-y-1 text-sm sm:col-span-2">
                  <span className="font-medium">JSON assertions</span>
                  <Textarea
                    value={form.apiJSONAssertions}
                    onChange={(event) => updateForm({ apiJSONAssertions: event.target.value })}
                    placeholder='[{"path":"$.ok","equals":true}]'
                  />
                </label>
              </>
            )}
            {isHostMonitor && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Host</span>
                  <Input
                    required
                    value={form.host}
                    onChange={(event) => updateForm({ host: event.target.value })}
                    placeholder="api.example.com"
                  />
                </label>
                {(form.kind === "tcp" || form.kind === "tls") && (
                  <label className="space-y-1 text-sm">
                    <span className="font-medium">Port</span>
                    <Input
                      required={form.kind === "tcp"}
                      inputMode="numeric"
                      min={1}
                      max={65535}
                      type="number"
                      value={form.port}
                      onChange={(event) => updateForm({ port: event.target.value })}
                      placeholder={form.kind === "tls" ? "443" : "5432"}
                    />
                  </label>
                )}
              </>
            )}
            {form.kind === "dns" && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Record type</span>
                  <Select
                    value={form.recordType}
                    onValueChange={(value) => {
                      if (isDNSRecordType(value)) updateForm({ recordType: value });
                    }}
                  >
                    <SelectTrigger className="w-full">
                      <span data-slot="select-value">{form.recordType}</span>
                    </SelectTrigger>
                    <SelectContent>
                      {dnsRecordTypeOptions.map((recordType) => (
                        <SelectItem key={recordType} value={recordType}>
                          {recordType}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </label>
                <label className="space-y-1 text-sm sm:col-span-2">
                  <span className="font-medium">Expected values</span>
                  <Textarea
                    value={form.expectedValues}
                    onChange={(event) => updateForm({ expectedValues: event.target.value })}
                    placeholder="203.0.113.10"
                  />
                </label>
              </>
            )}
            {form.kind === "tls" && (
              <>
                <label className="space-y-1 text-sm">
                  <span className="font-medium">Server name</span>
                  <Input
                    value={form.serverName}
                    onChange={(event) => updateForm({ serverName: event.target.value })}
                    placeholder="api.example.com"
                  />
                </label>
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
              </>
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

          {visibleError && (
            <p className="text-sm text-rose-700" role="alert">
              {visibleError}
            </p>
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
