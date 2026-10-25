import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
  Callout,
  EmptyState,
  FieldPanel,
  Input,
  Select,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  type FormState,
  buildConfigPayload,
  defaultForm,
  formFromMonitor,
} from "@/features/monitors/components/core-monitor-dialog-state";
import { CoreMonitorDialog } from "@/features/monitors/components/core-monitor-dialog";
import {
  type ServiceCoreManagedMonitorUpdateRequest,
  getGetCoreMonitorConfigQueryKey,
  getGetMonitorQueryKey,
  getGetMonitorsQueryKey,
  useGetCoreMonitorConfig,
  useGetMonitor,
  useUpdateCoreMonitor,
} from "@/orion-sdk";
import { NodeMonitorTiming } from "./node-monitor-timing";
import { DeleteMonitorDialog } from "../detail/delete-monitor-dialog";
import { useMonitorActions } from "../detail/use-monitor-actions";

const pick = (values: readonly number[], unit: (value: number) => string, current: string) => {
  const list = values.includes(Number(current)) ? values : [...values, Number(current)];
  return [...new Set(list)]
    .sort((a, b) => a - b)
    .map((value) => ({ value: String(value), label: unit(value) }));
};
const every = (value: number) =>
  value >= 3600
    ? `${value / 3600} ${value === 3600 ? "hour" : "hours"}`
    : value >= 60
      ? `${value / 60} ${value === 60 ? "minute" : "minutes"}`
      : `${value} seconds`;
const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"].map((value) => ({
  value,
  label: value,
}));

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";

export const MonitorEditPage = () => {
  const { monitorId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const monitorQuery = useGetMonitor(monitorId);
  const monitor = monitorQuery.data?.monitor;
  const summary = monitorQuery.data?.summary;
  const core = monitor?.owner_kind === "core" || monitor?.source === "core";
  const configQuery = useGetCoreMonitorConfig(monitorId, { query: { enabled: Boolean(core) } });
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [form, setForm] = useState<FormState>(defaultForm);
  const [runbook, setRunbook] = useState("");
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);
  const actions = useMonitorActions(monitorId, monitor?.name ?? "Monitor");
  const update = useUpdateCoreMonitor();

  useEffect(() => {
    if (!monitor || !configQuery.data?.config) return;
    setForm(formFromMonitor(configQuery.data.config, monitor));
    setRunbook(summary?.runbook_url ?? monitor.runbook_url ?? "");
    // Seed once when the data first arrives.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [monitor?.id, configQuery.data?.config?.monitor_id]);

  if (monitorQuery.isPending || (core && configQuery.isPending)) {
    return <Skeleton className="mt-10 h-60 w-full" />;
  }
  if (monitorQuery.isError || !monitor) {
    return <EmptyState tone="error" title="Unable to load monitor" />;
  }

  if (!core) {
    return (
      <div className="w-full pb-8">
        <div className="flex items-start justify-between gap-3 pb-4 pt-7">
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
            Configure {monitor.name}
          </h1>
          <Link to={`/monitors/${monitorId}`}>
            <Button size="sm" variant="secondary">
              Back
            </Button>
          </Link>
        </div>
        <NodeMonitorTiming
          nodeId={monitor.node_id ?? monitor.owner_id ?? ""}
          monitorId={monitorId}
        />
      </div>
    );
  }

  const patch = (next: Partial<FormState>) => setForm((current) => ({ ...current, ...next }));
  const http = ["http", "http_keyword", "expected_status", "api_request"].includes(form.kind);
  const tcp = form.kind === "tcp";
  const heartbeat = form.kind === "heartbeat";
  const paused = Boolean(summary?.paused ?? monitor.paused);

  const save = () => {
    setError("");
    try {
      update.mutate(
        {
          id: monitorId,
          data: {
            name: form.name.trim(),
            interval_seconds: Number(form.intervalSeconds),
            ...(heartbeat ? {} : { timeout_seconds: Number(form.timeoutSeconds) }),
            confirmation_check_count: Number(form.confirmationCheckCount),
            config: buildConfigPayload(form),
            runbook_url: runbook.trim(),
          },
        },
        {
          onSuccess: () => {
            for (const queryKey of [
              getGetMonitorQueryKey(monitorId),
              getGetCoreMonitorConfigQueryKey(monitorId),
              getGetMonitorsQueryKey(),
            ]) {
              void queryClient.invalidateQueries({ queryKey });
            }
            toast.add({ title: "Changes saved", description: "They apply from the next check." });
            navigate(`/monitors/${monitorId}`);
          },
          onError: (failure) =>
            setError((failure as Error).message || "Unable to save the monitor. Try again."),
        },
      );
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "The monitor is invalid.");
    }
  };

  return (
    <div className="w-full pb-8">
      <div className="flex items-start justify-between gap-3 pb-4 pt-7">
        <div>
          <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">
            Configure {monitor.name}
          </h1>
          <p className="mt-0.5 text-[13.5px] text-quiet">Changes apply from the next check.</p>
        </div>
        <div className="flex items-center gap-1.5">
          <Link to={`/monitors/${monitorId}`}>
            <Button size="sm" variant="secondary">
              Cancel
            </Button>
          </Link>
          <Button
            size="sm"
            disabled={!core || update.isPending || !form.name.trim()}
            onClick={save}
          >
            Save changes
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-3">
        {error && <Callout tone="warn">{error}</Callout>}
        <FieldPanel title="General">
          <div className="flex flex-col gap-3">
            <label className={label}>
              Name
              <Input
                className="font-normal"
                value={form.name}
                disabled={!core}
                onChange={(event) => patch({ name: event.target.value })}
              />
            </label>
            <div className="grid grid-cols-2 gap-4">
              <div className={label}>
                Check every
                <Select
                  aria-label="Check every"
                  className="w-full font-normal"
                  disabled={!core}
                  options={pick(
                    [30, 60, 120, 300, 600, 1800, 3600, 86400],
                    every,
                    form.intervalSeconds,
                  )}
                  value={form.intervalSeconds}
                  onValueChange={(next) => patch({ intervalSeconds: next })}
                />
              </div>
              {!heartbeat && (
                <div className={label}>
                  Timeout
                  <Select
                    aria-label="Timeout"
                    className="w-full font-normal"
                    disabled={!core}
                    options={pick([5, 10, 15, 30, 60], every, form.timeoutSeconds)}
                    value={form.timeoutSeconds}
                    onValueChange={(next) => patch({ timeoutSeconds: next })}
                  />
                </div>
              )}
            </div>
          </div>
        </FieldPanel>
        {(http || tcp || heartbeat) && (
          <FieldPanel title="Request" aside={http ? "HTTP" : tcp ? "TCP" : "Heartbeat"}>
            <div className="flex flex-col gap-3">
              {http && (
                <>
                  <div className="grid grid-cols-2 gap-4">
                    <div className={label}>
                      Method
                      <Select
                        aria-label="Method"
                        className="w-full font-normal"
                        disabled={!core || form.kind !== "api_request"}
                        options={METHODS}
                        value={form.apiMethod}
                        onValueChange={(next) =>
                          patch({ apiMethod: next as FormState["apiMethod"] })
                        }
                      />
                    </div>
                    <label className={label}>
                      Expected status
                      <Input
                        className="font-normal"
                        value={form.expectedStatus}
                        disabled={!core}
                        onChange={(event) => patch({ expectedStatus: event.target.value })}
                      />
                    </label>
                  </div>
                  <label className={label}>
                    URL
                    <Input
                      className="font-normal"
                      value={form.url}
                      disabled={!core}
                      onChange={(event) => patch({ url: event.target.value })}
                    />
                  </label>
                </>
              )}
              {tcp && (
                <div className="grid grid-cols-[1fr_96px] gap-4">
                  <label className={label}>
                    Host
                    <Input
                      className="font-normal"
                      value={form.host}
                      disabled={!core}
                      onChange={(event) => patch({ host: event.target.value })}
                    />
                  </label>
                  <label className={label}>
                    Port
                    <Input
                      className="font-normal"
                      value={form.port}
                      disabled={!core}
                      onChange={(event) => patch({ port: event.target.value })}
                    />
                  </label>
                </div>
              )}
              {heartbeat && (
                <div className={label}>
                  Grace period
                  <Select
                    aria-label="Grace period"
                    className="w-full font-normal"
                    disabled={!core}
                    options={pick([60, 300, 600, 1800, 3600], every, form.graceSeconds)}
                    value={form.graceSeconds}
                    onValueChange={(next) => patch({ graceSeconds: next })}
                  />
                </div>
              )}
            </div>
          </FieldPanel>
        )}
        {!http && !tcp && !heartbeat && (
          <FieldPanel title="Check settings" aside={form.kind}>
            <Button size="sm" variant="secondary" onClick={() => setAdvancedOpen(true)}>
              Edit {form.kind} settings
            </Button>
          </FieldPanel>
        )}
        <FieldPanel title="Alerting" aside="When it fails">
          <div className={label}>
            Fail after
            <Select
              aria-label="Fail after"
              className="w-full font-normal"
              disabled={!core || heartbeat}
              options={pick(
                [1, 2, 3, 5],
                (value) => `${value} ${value === 1 ? "check" : "checks"}`,
                form.confirmationCheckCount || "3",
              )}
              value={form.confirmationCheckCount || "3"}
              onValueChange={(next) => patch({ confirmationCheckCount: next })}
            />
          </div>
        </FieldPanel>
        <FieldPanel title="Runbook" aside="Shown on incidents">
          <label className={label}>
            Link
            <Input
              className="font-normal"
              value={runbook}
              disabled={!core}
              placeholder="https://wiki.example.com/runbooks/api-down"
              onChange={(event) => setRunbook(event.target.value)}
            />
          </label>
        </FieldPanel>
        <div className="flex justify-end gap-1.5">
          {paused ? (
            <Button
              size="sm"
              variant="secondary"
              disabled={actions.pending}
              onClick={actions.resume}
            >
              Resume
            </Button>
          ) : (
            <Button
              size="sm"
              variant="secondary"
              disabled={actions.pending}
              onClick={actions.pause}
            >
              Pause
            </Button>
          )}
          <Button size="sm" variant="secondary" disabled={actions.pending} onClick={actions.clone}>
            Clone
          </Button>
          <Button size="sm" variant="destructive" onClick={() => setDeleting(true)}>
            Delete monitor
          </Button>
        </div>
      </div>
      <CoreMonitorDialog
        mode="edit"
        open={advancedOpen}
        monitor={monitor}
        config={configQuery.data?.config}
        isSubmitting={update.isPending}
        error={error}
        onOpenChange={setAdvancedOpen}
        onSubmit={(data) =>
          update.mutate(
            { id: monitorId, data: data as ServiceCoreManagedMonitorUpdateRequest },
            {
              onSuccess: () => {
                for (const queryKey of [
                  getGetMonitorQueryKey(monitorId),
                  getGetCoreMonitorConfigQueryKey(monitorId),
                  getGetMonitorsQueryKey(),
                ]) {
                  void queryClient.invalidateQueries({ queryKey });
                }
                setAdvancedOpen(false);
                toast.add({ title: "Changes saved" });
              },
              onError: (failure) => setError((failure as Error).message || "Unable to save."),
            },
          )
        }
      />
      <DeleteMonitorDialog
        open={deleting}
        name={monitor.name ?? "This monitor"}
        pending={actions.pending}
        error={actions.removeError}
        onOpenChange={setDeleting}
        onConfirm={() => actions.remove(false)}
      />
    </div>
  );
};
