import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/page-header";
import {
  CoreWorkerDiagnosticsPanel,
  coreWorkerDiagnosticsFromPayload,
} from "@/features/monitors/components/core-worker-diagnostics";
import {
  CoreMonitorDialog,
  type CoreMonitorSubmitAction,
} from "@/features/monitors/components/core-monitor-dialog";
import { coreMonitorMutationErrorMessage } from "@/features/monitors/components/core-monitor-errors";
import { HeartbeatSetupPanel } from "@/features/monitors/components/heartbeat-setup-panel";
import { MonitorList } from "@/features/monitors/components/monitor-list";
import {
  type ApiCoreMonitorConfigResponse,
  type ApiMonitorResponse,
  type ServiceCoreManagedMonitorCreateRequest,
  getMonitorHistory,
  testCoreMonitor,
  useCreateCoreMonitor,
  useGetCoreWorkerDiagnostics,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import { explainMonitorFailure } from "./monitor-result-summary";

export const MonitorsPage = () => {
  const [createOpen, setCreateOpen] = useState(false);
  const [createFeedback, setCreateFeedback] = useState("");
  const [createFeedbackTone, setCreateFeedbackTone] = useState<"neutral" | "error">("neutral");
  const [heartbeatSetup, setHeartbeatSetup] = useState<{
    config: ApiCoreMonitorConfigResponse;
    monitor: ApiMonitorResponse;
    token: string;
  }>();
  const [isTestingCreatedMonitor, setIsTestingCreatedMonitor] = useState(false);
  const queryClient = useQueryClient();
  const workerDiagnosticsResponse = useGetCoreWorkerDiagnostics({
    query: { refetchInterval: 30_000 },
  });
  const workerDiagnostics = coreWorkerDiagnosticsFromPayload(workerDiagnosticsResponse.data);
  const refreshMonitors = () => {
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors"] });
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors/summary"] });
  };
  const createMonitor = useCreateCoreMonitor({
    mutation: {},
  });

  const handleCreate = async (
    data: ServiceCoreManagedMonitorCreateRequest,
    action: CoreMonitorSubmitAction,
  ) => {
    setCreateFeedback("");
    setCreateFeedbackTone("neutral");
    const created = await createMonitor.mutateAsync({ data });
    if (created.config?.kind === "heartbeat" && created.config.heartbeat_token && created.monitor) {
      setHeartbeatSetup({
        config: created.config,
        monitor: created.monitor,
        token: created.config.heartbeat_token,
      });
      setCreateFeedback("Heartbeat monitor created.");
      refreshMonitors();
      setCreateOpen(false);
      return;
    }
    setHeartbeatSetup(undefined);
    refreshMonitors();
    if (action === "save_test" && created.monitor?.id) {
      setIsTestingCreatedMonitor(true);
      try {
        const tested = await testCoreMonitor(created.monitor.id);
        const history = await getMonitorHistory(created.monitor.id, { limit: 1, offset: 0 });
        const health =
          tested.monitor?.computed_health ??
          tested.monitor?.health ??
          tested.result?.status ??
          "unknown";
        const explanation = explainMonitorFailure(history.reports?.[0], created.config?.kind);
        setCreateFeedback(
          health === "up"
            ? "Core monitor test reported up."
            : `Core monitor test reported ${health}: ${explanation}`,
        );
        setCreateFeedbackTone(health === "up" ? "neutral" : "error");
      } finally {
        setIsTestingCreatedMonitor(false);
      }
    } else {
      setCreateFeedback("Core monitor created.");
    }
    refreshMonitors();
    setCreateOpen(false);
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <PageHeader title="Monitors" description="Registered checks across all servers and Core." />
        <Button onClick={() => setCreateOpen(true)}>
          <Plus />
          Core monitor
        </Button>
      </div>
      {createFeedback && (
        <p
          className={
            createFeedbackTone === "error" ? "text-sm text-rose-700" : "text-sm text-neutral-600"
          }
        >
          {createFeedback}
        </p>
      )}
      {heartbeatSetup && (
        <HeartbeatSetupPanel
          config={heartbeatSetup.config}
          monitor={heartbeatSetup.monitor}
          token={heartbeatSetup.token}
        />
      )}
      <CoreWorkerDiagnosticsPanel
        data={workerDiagnosticsResponse.data}
        error={workerDiagnosticsResponse.error}
        isLoading={workerDiagnosticsResponse.isLoading}
      />
      <MonitorList workerDiagnostics={workerDiagnostics} />
      <CoreMonitorDialog
        error={coreMonitorMutationErrorMessage(
          createMonitor.error,
          "Unable to create Core monitor.",
        )}
        isSubmitting={createMonitor.isPending || isTestingCreatedMonitor}
        mode="create"
        onOpenChange={setCreateOpen}
        onSubmit={(data, action) =>
          void handleCreate(data as ServiceCoreManagedMonitorCreateRequest, action)
        }
        open={createOpen}
      />
    </div>
  );
};
