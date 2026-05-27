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
import { Textarea } from "@/components/ui/textarea";
import type {
  ApiCoreMonitorConfigResponse,
  ApiMonitorResponse,
  ServiceCoreManagedMonitorCreateRequest,
  ServiceCoreManagedMonitorUpdateRequest,
} from "@/orion-sdk";
import { Save } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";

type CoreMonitorDialogProps = {
  config?: ApiCoreMonitorConfigResponse;
  error?: string;
  isSubmitting?: boolean;
  mode: "create" | "edit";
  monitor?: ApiMonitorResponse;
  onOpenChange: (open: boolean) => void;
  onSubmit: (
    payload: ServiceCoreManagedMonitorCreateRequest | ServiceCoreManagedMonitorUpdateRequest,
  ) => void;
  open: boolean;
};

type FormState = {
  description: string;
  expectedStatus: string;
  intervalSeconds: string;
  name: string;
  paused: boolean;
  timeoutSeconds: string;
  url: string;
};

const defaultForm: FormState = {
  description: "",
  expectedStatus: "200",
  intervalSeconds: "60",
  name: "",
  paused: false,
  timeoutSeconds: "10",
  url: "",
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

const toPositiveInt = (value: string, fallback: number) => {
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
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

  useEffect(() => {
    if (!open) return;
    if (mode === "create") {
      setForm(defaultForm);
      return;
    }
    setForm({
      description: monitor?.description ?? "",
      expectedStatus: readConfigNumber(config, "expected_status") || "200",
      intervalSeconds: String(config?.interval_seconds ?? monitor?.reporting_interval_seconds ?? 60),
      name: monitor?.name ?? "",
      paused: config?.paused ?? false,
      timeoutSeconds: String(config?.timeout_seconds ?? 10),
      url: readConfigString(config, "url"),
    });
  }, [config, mode, monitor, open]);

  const updateForm = (patch: Partial<FormState>) => setForm((current) => ({ ...current, ...patch }));

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const expectedStatus = toPositiveInt(form.expectedStatus, 200);
    const payload = {
      config: {
        expected_status: expectedStatus,
        url: form.url.trim(),
      },
      description: form.description.trim() || undefined,
      interval_seconds: toPositiveInt(form.intervalSeconds, 60),
      kind: "http",
      name: form.name.trim(),
      paused: form.paused,
      timeout_seconds: toPositiveInt(form.timeoutSeconds, 10),
      type: "http",
    };
    onSubmit(payload);
  };

  const title = mode === "create" ? "Create Core Monitor" : "Edit Core Monitor";
  const description =
    mode === "create"
      ? "Add an HTTP check that runs from Orion Core."
      : "Update the Core-owned HTTP check configuration.";

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
            <label className="flex items-center gap-2 pt-7 text-sm">
              <Checkbox
                checked={form.paused}
                onCheckedChange={(checked) => updateForm({ paused: checked === true })}
              />
              Start paused
            </label>
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
            <Button disabled={isSubmitting || !form.name.trim() || !form.url.trim()} type="submit">
              <Save />
              {mode === "create" ? "Create" : "Save"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
