import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import type { GetCoreWorkerDiagnostics200 } from "@/orion-sdk";
import { Activity, AlertTriangle } from "lucide-react";

type WorkerStatus = "healthy" | "degraded" | "unknown" | "error" | string;

type CoreWorkerRow = {
  worker_id?: string;
  process_kind?: string;
  hostname?: string;
  status?: string;
  health?: string;
  version?: string;
  started_at?: string;
  last_heartbeat_at?: string;
  heartbeat_age_seconds?: number;
  last_error?: string;
  created_at?: string;
  updated_at?: string;
};

export type CoreWorkerDiagnostics = {
  status?: WorkerStatus;
  stale_after_seconds?: number;
  worker_count?: number;
  online_count?: number;
  stale_count?: number;
  workers?: CoreWorkerRow[];
};

type CoreWorkerDiagnosticsPayload = GetCoreWorkerDiagnostics200 & {
  api?: {
    status?: string;
    service?: string;
  };
  worker?: CoreWorkerDiagnostics;
};

const workerStatusLabel: Record<string, string> = {
  healthy: "healthy",
  degraded: "degraded",
  unknown: "unknown",
  error: "error",
};

const statusToBadge = (status?: string) => {
  if (status === "healthy") return "up";
  if (status === "error") return "down";
  return status;
};

export const coreWorkerDiagnosticsFromPayload = (
  payload?: GetCoreWorkerDiagnostics200,
): CoreWorkerDiagnostics | undefined => {
  return (payload as CoreWorkerDiagnosticsPayload | undefined)?.worker;
};

export const coreWorkerAPIStatusFromPayload = (
  payload?: GetCoreWorkerDiagnostics200,
): string | undefined => {
  return (payload as CoreWorkerDiagnosticsPayload | undefined)?.api?.status;
};

export const shouldWarnForCoreWorker = (worker?: CoreWorkerDiagnostics) => {
  const status = worker?.status ?? "unknown";
  const workerCount = worker?.worker_count ?? 0;
  const onlineCount = worker?.online_count ?? 0;
  const staleCount = worker?.stale_count ?? 0;

  return (
    status === "unknown" ||
    status === "degraded" ||
    workerCount === 0 ||
    onlineCount === 0 ||
    staleCount > 0
  );
};

export const describeCoreWorkerWarning = (worker?: CoreWorkerDiagnostics) => {
  const workerCount = worker?.worker_count ?? 0;
  const onlineCount = worker?.online_count ?? 0;
  const staleCount = worker?.stale_count ?? 0;

  if (!worker || worker.status === "unknown" || workerCount === 0 || onlineCount === 0) {
    return "No Core monitor worker heartbeat is available. Core-managed monitors will not run until a worker is online.";
  }

  if (staleCount > 0) {
    return "Some Core monitor worker heartbeats are stale. Core-managed checks may be delayed or incomplete.";
  }

  if (worker.status === "degraded") {
    return "Core monitor worker health is degraded. Core-managed checks may be delayed.";
  }

  return "";
};

type CoreWorkerWarningProps = {
  worker?: CoreWorkerDiagnostics;
  className?: string;
};

export const CoreWorkerWarning = ({ worker, className }: CoreWorkerWarningProps) => {
  const warning = describeCoreWorkerWarning(worker);
  if (!warning) return null;

  return (
    <div
      className={cn(
        "flex items-start gap-2 border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-950",
        className,
      )}
      role="status"
    >
      <AlertTriangle className="mt-0.5 size-4 shrink-0" />
      <span>{warning}</span>
    </div>
  );
};

type MetricProps = {
  label: string;
  value: string | number;
};

const Metric = ({ label, value }: MetricProps) => (
  <div className="min-w-24">
    <div className="text-xs text-neutral-500">{label}</div>
    <div className="text-sm font-medium text-neutral-950">{value}</div>
  </div>
);

type CoreWorkerDiagnosticsPanelProps = {
  data?: GetCoreWorkerDiagnostics200;
  isLoading?: boolean;
  error?: unknown;
};

export const CoreWorkerDiagnosticsPanel = ({
  data,
  isLoading = false,
  error,
}: CoreWorkerDiagnosticsPanelProps) => {
  const apiStatus = coreWorkerAPIStatusFromPayload(data) ?? (error ? "unknown" : "healthy");
  const worker = coreWorkerDiagnosticsFromPayload(data);
  const workerStatus = worker?.status ?? (error ? "unknown" : "unknown");
  const latestWorker = worker?.workers?.[0];
  const warning = describeCoreWorkerWarning(worker);

  return (
    <section
      aria-label="Core worker diagnostics"
      className="border border-neutral-200 bg-white px-4 py-3"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <Activity className="mt-1 size-4 text-neutral-500" />
          <div>
            <h2 className="text-sm font-medium text-neutral-950">Core execution health</h2>
            <p className="text-sm text-neutral-600">
              API availability and monitor worker capacity are tracked separately.
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs text-neutral-500">Core API</span>
          <StatusBadge value={toStatus(statusToBadge(apiStatus))} fallback={apiStatus} />
          <span className="ml-2 text-xs text-neutral-500">Monitor worker</span>
          <StatusBadge
            value={toStatus(statusToBadge(workerStatus))}
            fallback={workerStatusLabel[workerStatus] ?? workerStatus}
          />
        </div>
      </div>

      {isLoading && (
        <div className="mt-3 text-sm text-neutral-600">Loading worker diagnostics...</div>
      )}
      {Boolean(error) && (
        <div className="mt-3 text-sm text-rose-700">Unable to load worker diagnostics.</div>
      )}
      {!isLoading && !error && warning && <CoreWorkerWarning worker={worker} className="mt-3" />}

      <div className="mt-3 flex flex-wrap gap-x-8 gap-y-3">
        <Metric label="workers" value={worker?.worker_count ?? 0} />
        <Metric label="online" value={worker?.online_count ?? 0} />
        <Metric label="stale" value={worker?.stale_count ?? 0} />
        <Metric label="stale after" value={`${worker?.stale_after_seconds ?? 0}s`} />
        <Metric
          label="latest heartbeat"
          value={
            latestWorker?.last_heartbeat_at
              ? formatDate(latestWorker.last_heartbeat_at, DATE_TIME_FORMAT)
              : "none"
          }
        />
        <Metric label="latest state" value={latestWorker?.health ?? "unknown"} />
      </div>
    </section>
  );
};
