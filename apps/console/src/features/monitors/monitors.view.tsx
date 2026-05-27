import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/page-header";
import {
  CoreMonitorDialog,
  type CoreMonitorSubmitAction,
} from "@/features/monitors/components/core-monitor-dialog";
import { MonitorList } from "@/features/monitors/components/monitor-list";
import {
  type ApiMonitorReportResponse,
  type ServiceCoreManagedMonitorCreateRequest,
  getMonitorHistory,
  testCoreMonitor,
  useCreateCoreMonitor,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { useState } from "react";

const mutationErrorMessage = (error: unknown, fallback: string) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? fallback);
  }
  return fallback;
};

const parseReportPayload = (report?: ApiMonitorReportResponse) => {
  if (!report?.payload) return {};
  try {
    const parsed = JSON.parse(report.payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
};

const describeTestResult = (health: string, report?: ApiMonitorReportResponse) => {
  const payload = parseReportPayload(report);
  const statusCode = payload.status_code;
  const expectedStatus = payload.expected_status;
  const error = payload.error;
  if (health === "up") return "Core monitor test reported up.";
  if (typeof statusCode === "number" && expectedStatus !== undefined) {
    return `Core monitor test reported ${health}: received HTTP ${statusCode}, expected ${String(expectedStatus)}.`;
  }
  if (typeof error === "string" && error.trim() !== "") {
    return `Core monitor test reported ${health}: ${error}`;
  }
  return `Core monitor test reported ${health}. Review the latest check history row.`;
};

export const MonitorsPage = () => {
  const [createOpen, setCreateOpen] = useState(false);
  const [createFeedback, setCreateFeedback] = useState("");
  const [createFeedbackTone, setCreateFeedbackTone] = useState<"neutral" | "error">("neutral");
  const [isTestingCreatedMonitor, setIsTestingCreatedMonitor] = useState(false);
  const queryClient = useQueryClient();
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
    refreshMonitors();
    if (action === "save_test" && created.monitor?.id) {
      setIsTestingCreatedMonitor(true);
      try {
        const tested = await testCoreMonitor(created.monitor.id);
        const history = await getMonitorHistory(created.monitor.id, { limit: 1, offset: 0 });
        const health =
          tested.monitor?.computed_health ?? tested.monitor?.health ?? tested.result?.status ?? "unknown";
        setCreateFeedback(describeTestResult(health, history.reports?.[0]));
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
        <PageHeader title="Monitors" description="Registered checks across all agents and Core." />
        <Button onClick={() => setCreateOpen(true)}>
          <Plus />
          Core monitor
        </Button>
      </div>
      {createFeedback && (
        <p className={createFeedbackTone === "error" ? "text-sm text-rose-700" : "text-sm text-neutral-600"}>
          {createFeedback}
        </p>
      )}
      <MonitorList />
      <CoreMonitorDialog
        error={mutationErrorMessage(createMonitor.error, "Unable to create Core monitor.")}
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
