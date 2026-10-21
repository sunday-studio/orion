import { Callout, StatusBadge } from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import type { GetCoreWorkerDiagnostics200 } from "@/orion-sdk";
import { Activity } from "lucide-react";

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

/** A short lead-in and one sentence, shown in the design-system warning banner. */
export const describeCoreWorkerWarning = (worker?: CoreWorkerDiagnostics) => {
  const workerCount = worker?.worker_count ?? 0;
  const onlineCount = worker?.online_count ?? 0;
  const staleCount = worker?.stale_count ?? 0;

  if (!worker || worker.status === "unknown" || workerCount === 0 || onlineCount === 0) {
    return {
      lead: "No Core worker is online.",
      text: "Core-managed monitors will not run until a worker is online.",
    };
  }

  if (staleCount > 0) {
    return {
      lead: "Some Core worker heartbeats are stale.",
      text: "Core-managed checks may be delayed or incomplete.",
    };
  }

  if (worker.status === "degraded") {
    return {
      lead: "Core worker health is degraded.",
      text: "Core-managed checks may be delayed.",
    };
  }

  return undefined;
};

type CoreWorkerWarningProps = {
  worker?: CoreWorkerDiagnostics;
  className?: string;
};

export const CoreWorkerWarning = ({ worker, className }: CoreWorkerWarningProps) => {
  const warning = describeCoreWorkerWarning(worker);
  if (!warning) return null;

  return (
    <Callout tone="warn" lead={warning.lead} className={className}>
      {warning.text}
    </Callout>
  );
};

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
  const worker = coreWorkerDiagnosticsFromPayload(data);
  const workerStatus = worker?.status ?? (error ? "unknown" : "unknown");
  const latestWorker = worker?.workers?.[0];
  const warning = describeCoreWorkerWarning(worker);
  const onlineCount = worker?.online_count ?? 0;
  const workerCount = worker?.worker_count ?? 0;
  const latestHeartbeat = latestWorker?.last_heartbeat_at
    ? formatDate(latestWorker.last_heartbeat_at, DATE_TIME_FORMAT)
    : "none";
  const workerSummary = isLoading
    ? "Loading worker status..."
    : error
      ? "Unable to load worker status."
      : `${onlineCount}/${workerCount} online, latest heartbeat ${latestHeartbeat}`;

  return (
    <section
      aria-label="Core worker diagnostics"
      className="border border-neutral-200 bg-white px-4 py-3"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <Activity className="size-4 shrink-0 text-neutral-500" />
          <h2 className="text-sm font-medium text-neutral-950">Core monitor worker</h2>
          <StatusBadge status={statusToBadge(workerStatus)}>
            {statusToBadge(workerStatus) ?? workerStatusLabel[workerStatus] ?? workerStatus}
          </StatusBadge>
        </div>
        <div className="text-sm text-neutral-600">{workerSummary}</div>
      </div>
      {!isLoading && !error && warning && <CoreWorkerWarning worker={worker} className="mt-3" />}
    </section>
  );
};
