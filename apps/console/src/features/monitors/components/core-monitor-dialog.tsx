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
  confirmationCheckCount: string;
  confirmationPeriodSeconds: string;
  description: string;
  expectedStatus: string;
  graceSeconds: string;
  intervalSeconds: string;
  kind: "heartbeat" | "http" | "http_keyword";
  name: string;
  paused: boolean;
  requiredContains: string;
  recoveryPeriodSeconds: string;
  timeoutSeconds: string;
  url: string;
};

const defaultForm: FormState = {
  confirmationCheckCount: "0",
  confirmationPeriodSeconds: "0",
  description: "",
  expectedStatus: "200",
  graceSeconds: "60",
  intervalSeconds: "60",
  kind: "http",
  name: "",
  paused: false,
  requiredContains: "",
  recoveryPeriodSeconds: "0",
  timeoutSeconds: "10",
  url: "",
};

const coreMonitorKindOptions = [
  { value: "http", label: "HTTP status" },
  { value: "http_keyword", label: "HTTP keyword" },
  { value: "heartbeat", label: "Heartbeat" },
] as const;

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
    setForm({
      confirmationCheckCount: String(config?.confirmation_check_count ?? 0),
      confirmationPeriodSeconds: String(config?.confirmation_period_seconds ?? 0),
      description: monitor?.description ?? "",
      expectedStatus: readConfigNumber(config, "expected_status") || "200",
      graceSeconds: readConfigNumber(config, "grace_seconds") || "60",
      intervalSeconds: String(
        config?.interval_seconds ?? monitor?.reporting_interval_seconds ?? 60,
      ),
      kind:
        config?.kind === "heartbeat"
          ? "heartbeat"
          : config?.kind === "http_keyword"
            ? "http_keyword"
            : "http",
      name: monitor?.name ?? "",
      paused: config?.paused ?? false,
      requiredContains: readConfigStringList(config, "required_contains"),
      recoveryPeriodSeconds: String(config?.recovery_period_seconds ?? 0),
      timeoutSeconds: String(config?.timeout_seconds ?? 10),
      url: readConfigString(config, "url"),
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
    const payload = {
      config: isHeartbeat
        ? {
            grace_seconds: toPositiveInt(form.graceSeconds, 60),
          }
        : {
            expected_status: expectedStatus,
            ...(form.kind === "http_keyword" && requiredContains.length > 0
              ? { required_contains: requiredContains }
              : {}),
            url: form.url.trim(),
          },
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
  const canSubmit = form.name.trim() && (isHeartbeat || form.url.trim());

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
                  if (value === "http" || value === "http_keyword" || value === "heartbeat") {
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
                  if (value === "http" || value === "http_keyword" || value === "heartbeat") {
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
            {!isHeartbeat && (
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

          {error && <p className="text-sm text-rose-700">{error}</p>}

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
