import { PageBreadcrumbs } from "@/components/shared/page-breadcrumbs";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge, toStatus } from "@/components/shared/status-badges";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  CoreWorkerWarning,
  coreWorkerDiagnosticsFromPayload,
  shouldWarnForCoreWorker,
} from "@/features/monitors/components/core-worker-diagnostics";
import { coreMonitorMutationErrorMessage } from "@/features/monitors/components/core-monitor-errors";
import { CoreMonitorDialog } from "@/features/monitors/components/core-monitor-dialog";
import {
  explainMonitorFailure,
  parseMonitorPayload,
  summarizeMonitorResult,
} from "@/features/monitors/monitors.domain";
import { ReportInspectionDrawer } from "@/features/report-inspection/components/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { cn } from "@/utils/cn";
import {
  type ApiMonitorReportResponse,
  type ServiceCoreManagedMonitorUpdateRequest,
  getGetCoreMonitorConfigQueryKey,
  getGetMonitorHistoryQueryKey,
  getGetMonitorQueryKey,
  getMonitorHistory,
  useDeleteCoreMonitor,
  useGetCoreMonitorConfig,
  useGetCoreWorkerDiagnostics,
  useGetIncident,
  useGetIncidents,
  useGetMonitor,
  useGetMonitorHistory,
  useGetMonitorUptime,
  usePauseCoreMonitor,
  useResumeCoreMonitor,
  useTestCoreMonitor,
  useUpdateCoreMonitor,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Pause, Play, RefreshCw, Save, Trash2 } from "lucide-react";
import { parseAsInteger, useQueryStates } from "nuqs";
import { useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { MonitorDetailOperationalData } from "./components/monitor-detail-operational-data";
import {
  HighlightedIncidentBanner,
  MonitorDetailOverview,
} from "./components/monitor-detail-overview";
import {
  HISTORY_LIMIT,
  type MonitorDetailTab,
  isCoreOwnedMonitor,
  isHeartbeatPayload,
  isMonitorDetailTab,
  reportTimestamp,
} from "./components/monitor-detail-shared";

export const MonitorDetailPage = () => {
  const { monitorId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [{ historyPage }, setHistoryQuery] = useQueryStates({
    historyPage: parseAsInteger.withDefault(1),
  });
  const historyOffset = (Math.max(historyPage, 1) - 1) * HISTORY_LIMIT;
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedTab = searchParams.get("tab");
  const activeTab: MonitorDetailTab = isMonitorDetailTab(selectedTab) ? selectedTab : "history";
  const highlightedIncidentId = searchParams.get("incident") ?? "";
  const monitorResponse = useGetMonitor(monitorId);
  const uptimeResponse = useGetMonitorUptime(monitorId, { period: "90d" });
  const historyQuery = useGetMonitorHistory(monitorId, {
    limit: HISTORY_LIMIT,
    offset: historyOffset,
  });
  const incidentsResponse = useGetIncidents({
    monitor_id: monitorId,
    status: "open,acknowledged,covered,resolved",
    limit: 50,
  });
  const highlightedIncidentResponse = useGetIncident(highlightedIncidentId);

  const monitor = monitorResponse.data?.monitor;
  const isCoreMonitor = isCoreOwnedMonitor(monitor);
  const workerDiagnosticsResponse = useGetCoreWorkerDiagnostics({
    query: { enabled: Boolean(isCoreMonitor), refetchInterval: 30_000 },
  });
  const workerDiagnostics = coreWorkerDiagnosticsFromPayload(workerDiagnosticsResponse.data);
  const coreConfigResponse = useGetCoreMonitorConfig(monitorId, {
    query: { enabled: Boolean(monitorId && isCoreMonitor) },
  });
  const coreConfig = coreConfigResponse.data?.config;
  const reports = historyQuery.data?.reports ?? [];
  const reportCount = historyQuery.data?.count ?? reports.length;
  const latestReport = monitorResponse.data?.recent_reports?.[0] ?? reports[0];
  const [selectedReport, setSelectedReport] = useState<ApiMonitorReportResponse>();
  const monitorKind = coreConfig?.kind ?? monitor?.type;
  const latestSummary = summarizeMonitorResult(latestReport, monitorKind);
  const heartbeatReports = reports.filter((report) =>
    isHeartbeatPayload(parseMonitorPayload(report.payload)),
  );
  const latestHeartbeatReport = heartbeatReports[0];
  const latestHeartbeatFailure = heartbeatReports.find(
    (report) => report.health && report.health !== "up",
  );
  const uptimeBuckets = uptimeResponse.data?.daily_buckets ?? [];
  const recentUptimeBuckets = uptimeBuckets.slice(-7);
  const health =
    monitorResponse.data?.computed_health ??
    monitor?.computed_health ??
    monitor?.health ??
    "unknown";
  const relatedIncidents = incidentsResponse.data?.incidents ?? [];
  const activeIncidents = relatedIncidents.filter((incident) =>
    ["open", "acknowledged"].includes(incident.status ?? ""),
  );
  const highlightedIncidentFromList = relatedIncidents.find(
    (incident) => incident.id === highlightedIncidentId,
  );
  const highlightedIncident =
    highlightedIncidentResponse.data?.incident?.monitor_id === monitorId
      ? highlightedIncidentResponse.data.incident
      : highlightedIncidentFromList;
  const incidentContext = highlightedIncident ?? activeIncidents[0] ?? relatedIncidents[0];
  const incidentContextLabel = highlightedIncident
    ? "highlighted"
    : activeIncidents[0]
      ? "active"
      : relatedIncidents[0]
        ? "latest"
        : "none";
  const [editOpen, setEditOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [actionFeedback, setActionFeedback] = useState("");
  const refreshMonitor = () => {
    void queryClient.invalidateQueries({ queryKey: getGetMonitorQueryKey(monitorId) });
    void queryClient.invalidateQueries({ queryKey: getGetCoreMonitorConfigQueryKey(monitorId) });
    void queryClient.invalidateQueries({ queryKey: getGetMonitorHistoryQueryKey(monitorId) });
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors"] });
    void queryClient.invalidateQueries({ queryKey: ["/v1/monitors/summary"] });
  };
  const updateMonitor = useUpdateCoreMonitor({
    mutation: {
      onSuccess: () => {
        setActionFeedback("Core monitor saved.");
        setEditOpen(false);
        refreshMonitor();
      },
    },
  });
  const testMonitor = useTestCoreMonitor({
    mutation: {
      onSuccess: (result) => {
        const testHealth =
          result.monitor?.computed_health ??
          result.monitor?.health ??
          result.result?.status ??
          "unknown";
        void getMonitorHistory(monitorId, { limit: 1, offset: 0 })
          .then((history) => {
            const explanation = explainMonitorFailure(history.reports?.[0], result.config?.kind);
            setActionFeedback(
              testHealth === "up"
                ? "Core monitor test reported up."
                : `Core monitor test reported ${testHealth}: ${explanation}`,
            );
          })
          .catch(() => {
            setActionFeedback(
              testHealth === "up"
                ? "Core monitor test reported up."
                : `Core monitor test reported ${testHealth}. Review the latest check history row.`,
            );
          });
        refreshMonitor();
      },
    },
  });
  const pauseMonitor = usePauseCoreMonitor({
    mutation: {
      onSuccess: () => {
        setActionFeedback("Core monitor paused.");
        refreshMonitor();
      },
    },
  });
  const resumeMonitor = useResumeCoreMonitor({
    mutation: {
      onSuccess: () => {
        setActionFeedback("Core monitor resumed.");
        refreshMonitor();
      },
    },
  });
  const deleteMonitor = useDeleteCoreMonitor({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: ["/v1/monitors"] });
        void queryClient.invalidateQueries({ queryKey: ["/v1/monitors/summary"] });
        setDeleteOpen(false);
        navigate("/monitors");
      },
    },
  });
  const setHistoryOffset = (nextOffset: number) => {
    void setHistoryQuery({ historyPage: Math.floor(nextOffset / HISTORY_LIMIT) + 1 });
  };
  const handleTabChange = (tab: string) => {
    if (!isMonitorDetailTab(tab)) return;
    setSearchParams(
      (params) => {
        if (tab === "history") {
          params.delete("tab");
        } else {
          params.set("tab", tab);
        }
        return params;
      },
      { replace: true },
    );
  };
  const actionError =
    coreMonitorMutationErrorMessage(testMonitor.error, "Unable to test Core monitor.") ||
    coreMonitorMutationErrorMessage(pauseMonitor.error, "Unable to pause Core monitor.") ||
    coreMonitorMutationErrorMessage(resumeMonitor.error, "Unable to resume Core monitor.") ||
    coreMonitorMutationErrorMessage(updateMonitor.error, "Unable to save Core monitor.") ||
    coreMonitorMutationErrorMessage(deleteMonitor.error, "Unable to delete Core monitor.");
  const isActionPending =
    testMonitor.isPending ||
    pauseMonitor.isPending ||
    resumeMonitor.isPending ||
    updateMonitor.isPending ||
    deleteMonitor.isPending;

  if (monitorResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading monitor...</div>;
  }

  if (monitorResponse.error || !monitor) {
    return <div className="py-3 text-sm">Unable to load monitor.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-2">
        <PageBreadcrumbs
          items={[{ label: "Monitors", to: "/monitors" }, { label: monitor.name ?? "Monitor" }]}
        />
        <div className="flex flex-wrap items-start justify-between gap-3">
          <PageHeader
            title={monitor.name ?? monitor.id ?? "Unknown monitor"}
            description={
              <p className="text-sm text-neutral-600">
                <StatusBadge className="px-1.5 py-0.5 text-[13px]" value={toStatus(health)} /> ·{" "}
                {isCoreMonitor ? "Core" : "Server"} · {monitor.type ?? "unknown"} · last checked{" "}
                {formatDate(reportTimestamp(latestReport), DATE_TIME_FORMAT)}
              </p>
            }
          />
          {isCoreMonitor && (
            <div className="flex flex-wrap gap-2">
              <Button
                disabled={isActionPending}
                size="sm"
                variant="outline"
                onClick={() => testMonitor.mutate({ id: monitor.id ?? "" })}
              >
                <Play />
                Test
              </Button>
              {coreConfig?.paused ? (
                <Button
                  disabled={isActionPending}
                  size="sm"
                  variant="outline"
                  onClick={() => resumeMonitor.mutate({ id: monitor.id ?? "" })}
                >
                  <RefreshCw />
                  Resume
                </Button>
              ) : (
                <Button
                  disabled={isActionPending}
                  size="sm"
                  variant="outline"
                  onClick={() => pauseMonitor.mutate({ id: monitor.id ?? "" })}
                >
                  <Pause />
                  Pause
                </Button>
              )}
              <Button
                disabled={isActionPending || coreConfigResponse.isLoading}
                size="sm"
                variant="outline"
                onClick={() => setEditOpen(true)}
              >
                <Save />
                Edit
              </Button>
              <Button
                disabled={isActionPending}
                size="sm"
                variant="destructive"
                onClick={() => setDeleteOpen(true)}
              >
                <Trash2 />
                Delete
              </Button>
            </div>
          )}
        </div>
        {(actionFeedback || actionError) && (
          <p className={cn("text-sm", actionError ? "text-rose-700" : "text-neutral-600")}>
            {actionError || actionFeedback}
          </p>
        )}
      </div>

      {isCoreMonitor && shouldWarnForCoreWorker(workerDiagnostics) && (
        <CoreWorkerWarning worker={workerDiagnostics} />
      )}

      <HighlightedIncidentBanner incident={highlightedIncident} />

      <MonitorDetailOverview
        activeIncidents={activeIncidents}
        coreConfig={coreConfig}
        health={health}
        highlightedIncident={highlightedIncident}
        incidentContext={incidentContext}
        incidentContextLabel={incidentContextLabel}
        isCoreMonitor={isCoreMonitor}
        latestHeartbeatFailure={latestHeartbeatFailure}
        latestHeartbeatReport={latestHeartbeatReport}
        latestReport={latestReport}
        latestSummary={latestSummary}
        monitor={monitor}
        recentUptimeBuckets={recentUptimeBuckets}
        relatedIncidents={relatedIncidents}
        setSelectedReport={setSelectedReport}
        uptimeBucketCount={uptimeBuckets.length}
        uptimePercent={uptimeResponse.data?.uptime_percent}
      />

      <MonitorDetailOperationalData
        activeTab={activeTab}
        coreConfig={coreConfig}
        coreConfigError={coreConfigResponse.error}
        coreConfigLoading={coreConfigResponse.isLoading}
        handleTabChange={handleTabChange}
        highlightedIncidentId={highlightedIncidentId}
        historyError={historyQuery.error}
        historyLoading={historyQuery.isLoading}
        historyOffset={historyOffset}
        incidentsError={incidentsResponse.error}
        incidentsLoading={incidentsResponse.isLoading}
        isCoreMonitor={isCoreMonitor}
        monitor={monitor}
        onHistoryOffsetChange={setHistoryOffset}
        onReportClick={setSelectedReport}
        relatedIncidents={relatedIncidents}
        reportCount={reportCount}
        reports={reports}
      />
      <ReportInspectionDrawer
        kind="monitor"
        report={selectedReport}
        onOpenChange={(open) => {
          if (!open) setSelectedReport(undefined);
        }}
      />
      {isCoreMonitor && (
        <CoreMonitorDialog
          config={coreConfig}
          error={coreMonitorMutationErrorMessage(
            updateMonitor.error,
            "Unable to save Core monitor.",
          )}
          isSubmitting={updateMonitor.isPending}
          mode="edit"
          monitor={monitor}
          onOpenChange={setEditOpen}
          onSubmit={(data) =>
            updateMonitor.mutate({
              id: monitor.id ?? "",
              data: data as ServiceCoreManagedMonitorUpdateRequest,
            })
          }
          open={editOpen}
        />
      )}
      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete Core Monitor</DialogTitle>
            <DialogDescription>
              Delete {monitor.name ?? "this monitor"} and stop future Core checks.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={deleteMonitor.isPending}
              variant="destructive"
              onClick={() => deleteMonitor.mutate({ id: monitor.id ?? "" })}
            >
              <Trash2 />
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};
