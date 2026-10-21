import { Callout } from "@/design-system";
import type { GetCoreWorkerDiagnostics200 } from "@/orion-sdk";

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
const describeCoreWorkerWarning = (worker?: CoreWorkerDiagnostics) => {
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
