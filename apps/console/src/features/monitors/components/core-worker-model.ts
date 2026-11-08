// Which worker warning the Monitors page shows. Pure, no React.
import type { GetCoreWorkerDiagnostics200 } from "../../../orion-sdk/index.ts";

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

/**
 * Whether the banner has anything to say. A Core that has never had a worker is not reported here:
 * a local Core often runs without a separate one.
 */
export const shouldWarnForCoreWorker = (worker?: CoreWorkerDiagnostics) =>
  describeCoreWorkerWarning(worker) !== undefined;

/** A short lead-in and one sentence, shown in the design-system warning banner. */
export const describeCoreWorkerWarning = (worker?: CoreWorkerDiagnostics) => {
  // No worker has ever registered: nothing to report. Workers that registered and went quiet still
  // are (Core counts only fresh heartbeats as online, so a dead worker has online_count 0).
  if (!worker || (worker.worker_count ?? 0) === 0) return undefined;

  if ((worker.stale_count ?? 0) > 0) {
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
