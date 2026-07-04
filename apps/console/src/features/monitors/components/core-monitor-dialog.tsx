import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type {
  ApiCoreMonitorConfigResponse,
  ApiMonitorResponse,
  ServiceCoreManagedMonitorCreateRequest,
  ServiceCoreManagedMonitorUpdateRequest,
} from "@/orion-sdk";
import { Play, Save } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import { CoreMonitorDialogFields } from "./core-monitor-dialog-fields";
import {
  type CoreMonitorSubmitAction,
  type FormState,
  buildConfigPayload,
  defaultForm,
  formFromMonitor,
  parseJSONConfig,
  toNonNegativeInt,
  toPositiveInt,
} from "./core-monitor-dialog-state";

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

export type { CoreMonitorSubmitAction };

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

  useEffect(() => {
    if (!open) return;
    setLocalError("");
    setForm(mode === "create" ? defaultForm : formFromMonitor(config, monitor));
  }, [config, mode, monitor, open]);

  const updateForm = (patch: Partial<FormState>) =>
    setForm((current) => ({ ...current, ...patch }));

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const action: CoreMonitorSubmitAction = submitter?.value === "save_test" ? "save_test" : "save";
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
      onSubmit(payload, action);
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
    form.kind === "http" ||
    form.kind === "http_keyword" ||
    form.kind === "expected_status" ||
    form.kind === "api_request";
  const isHostMonitor = [
    "tcp",
    "udp",
    "dns",
    "tls",
    "ping",
    "mail",
    "smtp",
    "imap",
    "pop",
  ].includes(form.kind);
  const isDomainMonitor = form.kind === "domain_expiration";
  const usesAdvancedJSON = form.kind === "synthetic" || form.kind === "playwright";
  const advancedConfigError =
    usesAdvancedJSON && !parseJSONConfig(form.advancedConfig)
      ? "Configuration JSON must be an object."
      : "";
  const requiresPort = form.kind === "tcp" || form.kind === "udp";
  const canSubmit =
    form.name.trim() &&
    !advancedConfigError &&
    (isHeartbeat ||
      (isURLMonitor && form.url.trim()) ||
      (isHostMonitor && form.host.trim() && (!requiresPort || form.port.trim())) ||
      (isDomainMonitor && form.domain.trim()) ||
      usesAdvancedJSON);
  const visibleError = localError || error;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[min(90vh,900px)] overflow-y-auto sm:max-w-3xl">
        <form className="space-y-5" onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>

          <CoreMonitorDialogFields
            advancedConfigError={advancedConfigError}
            form={form}
            isHeartbeat={isHeartbeat}
            updateForm={updateForm}
          />

          {visibleError && (
            <p className="text-sm text-rose-700" role="alert">
              {visibleError}
            </p>
          )}

          <DialogFooter className="gap-2">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button disabled={isSubmitting || !canSubmit} type="submit" value="save">
              <Save />
              {mode === "create" ? "Create" : "Save"}
            </Button>
            {mode === "create" && !isHeartbeat && (
              <Button disabled={isSubmitting || !canSubmit} type="submit" value="save_test">
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
