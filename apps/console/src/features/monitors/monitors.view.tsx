import { Button, PageHeader } from "@/design-system";
import { coreWorkerDiagnosticsFromPayload } from "@/features/monitors/components/core-worker-diagnostics";
import {
  CoreMonitorDialog,
  type CoreMonitorSubmitAction,
} from "@/features/monitors/components/core-monitor-dialog";
import { coreMonitorMutationErrorMessage } from "@/features/monitors/components/core-monitor-errors";
import { HeartbeatSetupPanel } from "@/features/monitors/components/heartbeat-setup-panel";
import { MonitorCreateDialog } from "@/features/monitors/create/monitor-create-dialog";
import { MonitorList } from "@/features/monitors/components/monitor-list";
import {
  type ApiCoreMonitorConfigResponse,
  type ApiMonitorResponse,
  type ServiceCoreManagedMonitorCreateRequest,
  getGetMonitorSummaryQueryKey,
  getGetMonitorsQueryKey,
  getMonitorHistory,
  testCoreMonitor,
  useCreateCoreMonitor,
  useGetNodeSummary,
  useGetCoreWorkerDiagnostics,
  useGetMonitorSummary,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";
import { pluralize } from "@/utils/plural";
import { explainMonitorFailure } from "./monitors.domain";

export const MonitorsPage = () => {
  const [createOpen, setCreateOpen] = useState(false);
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [createFeedback, setCreateFeedback] = useState("");
  const [createFeedbackTone, setCreateFeedbackTone] = useState<"neutral" | "error">("neutral");
  const [heartbeatSetup, setHeartbeatSetup] = useState<{
    config: ApiCoreMonitorConfigResponse;
    monitor: ApiMonitorResponse;
    token: string;
  }>();
  const [isTestingCreatedMonitor, setIsTestingCreatedMonitor] = useState(false);
  const queryClient = useQueryClient();
  const summary = useGetMonitorSummary();
  const servers = useGetNodeSummary();
  const workerDiagnosticsResponse = useGetCoreWorkerDiagnostics({
    query: { refetchInterval: 30_000 },
  });
  const workerDiagnostics = coreWorkerDiagnosticsFromPayload(workerDiagnosticsResponse.data);
  const refreshMonitors = () => {
    void queryClient.invalidateQueries({ queryKey: getGetMonitorsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetMonitorSummaryQueryKey() });
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
      setAdvancedOpen(false);
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
    setAdvancedOpen(false);
  };

  const total = summary.data?.total;
  return (
    <div className="w-full pb-8">
      <PageHeader
        title="Monitors"
        description={
          total
            ? `${pluralize(total, "check")} across ${pluralize(servers.data?.total ?? 0, "server")} and Core.`
            : undefined
        }
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus /> Create monitor
          </Button>
        }
      />
      {createFeedback && (
        <p
          className={
            createFeedbackTone === "error"
              ? "mt-2 text-sm text-rose-700"
              : "mt-2 text-sm text-neutral-600"
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
      {total === 0 ? (
        <div className="mt-3 flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <b className="text-[15px] font-medium">No monitors yet</b>
          <p className="text-[13px] text-mid">
            Check a URL, a port, a cron job or a process on a server.
          </p>
          <Button size="sm" className="mt-3" onClick={() => setCreateOpen(true)}>
            Create a monitor
          </Button>
        </div>
      ) : (
        <MonitorList workerDiagnostics={workerDiagnostics} />
      )}
      <MonitorCreateDialog
        open={createOpen}
        error={coreMonitorMutationErrorMessage(
          createMonitor.error,
          "Unable to create the monitor.",
        )}
        pending={createMonitor.isPending}
        onOpenChange={setCreateOpen}
        onCreate={(data) => void handleCreate(data, "save").catch(() => undefined)}
        onMoreTypes={() => {
          setCreateOpen(false);
          setAdvancedOpen(true);
        }}
      />
      <CoreMonitorDialog
        error={coreMonitorMutationErrorMessage(
          createMonitor.error,
          "Unable to create Core monitor.",
        )}
        isSubmitting={createMonitor.isPending || isTestingCreatedMonitor}
        mode="create"
        onOpenChange={setAdvancedOpen}
        onSubmit={(data, action) =>
          void handleCreate(data as ServiceCoreManagedMonitorCreateRequest, action)
        }
        open={advancedOpen}
      />
    </div>
  );
};
