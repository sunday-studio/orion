import { PageHeader } from "@/components/shared/page-header";
import {
  CoreWorkerDiagnosticsPanel,
  coreWorkerDiagnosticsFromPayload,
} from "@/features/monitors/components/core-worker-diagnostics";
import { MonitorList } from "@/features/monitors/components/monitor-list";
import { useGetCoreWorkerDiagnostics } from "@/orion-sdk";

export const MonitorsPage = () => {
  const workerDiagnosticsResponse = useGetCoreWorkerDiagnostics({
    query: { refetchInterval: 30_000 },
  });
  const workerDiagnostics = coreWorkerDiagnosticsFromPayload(workerDiagnosticsResponse.data);

  return (
    <div className="space-y-4">
      <PageHeader title="Monitors" description="Registered checks across all servers and Core." />
      <CoreWorkerDiagnosticsPanel
        data={workerDiagnosticsResponse.data}
        error={workerDiagnosticsResponse.error}
        isLoading={workerDiagnosticsResponse.isLoading}
      />
      <MonitorList workerDiagnostics={workerDiagnostics} />
    </div>
  );
};
