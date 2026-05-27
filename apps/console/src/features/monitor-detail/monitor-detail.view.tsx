import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { PageHeader } from "@/components/page-header";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { HeartbeatSetupPanel } from "@/features/monitors/components/heartbeat-setup-panel";
import { CoreMonitorDialog } from "@/features/monitors/components/core-monitor-dialog";
import { ReportInspectionDrawer } from "@/features/report-inspection/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import {
  type ApiIncidentResponse,
  type ApiMonitorReportResponse,
  type ServiceCoreManagedMonitorUpdateRequest,
  getGetCoreMonitorConfigQueryKey,
  getGetMonitorHistoryQueryKey,
  getGetMonitorQueryKey,
  useDeleteCoreMonitor,
  useGetCoreMonitorConfig,
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
import type { ColumnDef } from "@tanstack/react-table";
import { Pause, Play, RefreshCw, Save, Trash2 } from "lucide-react";
import { parseAsInteger, useQueryStates } from "nuqs";
import { type ReactNode, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";

const HISTORY_LIMIT = 20;
const monitorDetailTabs = ["history", "incidents", "config"] as const;
type MonitorDetailTab = (typeof monitorDetailTabs)[number];

const isMonitorDetailTab = (value: string | null): value is MonitorDetailTab =>
  monitorDetailTabs.includes(value as MonitorDetailTab);

type MonitorPayload = Record<string, unknown>;

const parsePayload = (payload?: string): MonitorPayload => {
  if (!payload) return {};
  try {
    const parsed = JSON.parse(payload);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? parsed : {};
  } catch {
    return {};
  }
};

const readString = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number") return String(value);
  }
  return "—";
};

const readNumber = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "number") return value;
    if (typeof value === "string" && value.trim() !== "" && !Number.isNaN(Number(value))) {
      return Number(value);
    }
  }
  return undefined;
};

const formatLatency = (payload: MonitorPayload) => {
  const latency = readNumber(payload, ["latency_ms", "response_time_ms", "duration_ms"]);
  return typeof latency === "number" ? `${latency} ms` : "—";
};

const payloadSummary = (report: ApiMonitorReportResponse) => {
  const payload = parsePayload(report.payload);
  return (
    readString(payload, [
      "failure_reason",
      "message",
      "error",
      "summary",
      "payload",
      "failure_stage",
      "status",
      "status_code",
    ]) ?? "—"
  );
};

const isHeartbeatPayload = (payload: MonitorPayload) =>
  payload.type === "heartbeat" || payload.runner === "heartbeat";

const heartbeatPayloadContext = (report?: ApiMonitorReportResponse) => {
  if (!report) return "—";
  const payload = parsePayload(report.payload);
  if (!isHeartbeatPayload(payload)) return "—";
  return readString(payload, ["payload", "failure_stage", "status", "message", "error"]);
};

const DetailItem = ({ label, value }: { label: string; value: ReactNode }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);

const DetailGroup = ({ title, children }: { title: string; children: ReactNode }) => (
  <div className="space-y-3 bg-neutral-50 px-3 py-3">
    <h2 className="text-sm font-medium">{title}</h2>
    <div className="space-y-3">{children}</div>
  </div>
);

const formatUptime = (value?: number) => (typeof value === "number" ? `${value.toFixed(1)}%` : "—");

const mutationErrorMessage = (error: unknown, fallback: string) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? fallback);
  }
  return fallback;
};

const isCoreOwnedMonitor = (monitor?: { owner_kind?: string; source?: string }) =>
  monitor?.owner_kind === "core" || monitor?.source === "core";

const formatJSON = (value?: Record<string, unknown>) => JSON.stringify(value ?? {}, null, 2);

const coreConfigValue = (config: { config?: Record<string, unknown> } | undefined, key: string) => {
  const value = config?.config?.[key];
  if (typeof value === "number") return String(value);
  if (typeof value === "string" && value.trim() !== "") return value;
  return "—";
};

const reportTimestamp = (report?: ApiMonitorReportResponse) =>
  report?.created_at ?? report?.collected_at;

const bucketFillClassName = (bucket: { total?: number; uptime_percent?: number }) => {
  const percent = bucket.uptime_percent ?? 0;

  if (!bucket.total) return "bg-neutral-300";
  if (percent >= 99) return "bg-emerald-400";
  if (percent >= 95) return "bg-amber-300";
  return "bg-rose-400";
};

const historyColumns: ColumnDef<ApiMonitorReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.collected_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "health",
    header: "Status",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.health)} />,
  },
  {
    id: "latency",
    header: "Latency",
    cell: ({ row }) => formatLatency(parsePayload(row.original.payload)),
  },
  {
    id: "result",
    header: "Result",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">{payloadSummary(row.original)}</div>
    ),
  },
];

const incidentColumns: ColumnDef<ApiIncidentResponse>[] = [
  {
    accessorKey: "title",
    header: "Incident",
    cell: ({ row }) => (
      <DataTableLink to={`/incidents/${row.original.id}`}>
        {row.original.title ?? row.original.id ?? "Untitled incident"}
      </DataTableLink>
    ),
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.status)} />,
  },
  {
    accessorKey: "opened_at",
    header: "Opened",
    cell: ({ row }) => formatDate(row.original.opened_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "latest_event",
    header: "Latest event",
    cell: ({ row }) => (
      <div className="max-w-[22rem] truncate text-neutral-600">
        {row.original.latest_event ?? "—"}
      </div>
    ),
  },
];

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
    status: "open,acknowledged,resolved",
    limit: 50,
  });
  const highlightedIncidentResponse = useGetIncident(highlightedIncidentId);

  const monitor = monitorResponse.data?.monitor;
  const isCoreMonitor = isCoreOwnedMonitor(monitor);
  const coreConfigResponse = useGetCoreMonitorConfig(monitorId, {
    query: { enabled: Boolean(monitorId && isCoreMonitor) },
  });
  const coreConfig = coreConfigResponse.data?.config;
  const reports = historyQuery.data?.reports ?? [];
  const reportCount = historyQuery.data?.count ?? reports.length;
  const latestReport = monitorResponse.data?.recent_reports?.[0] ?? reports[0];
  const [selectedReport, setSelectedReport] = useState<ApiMonitorReportResponse>();
  const latestPayload = parsePayload(latestReport?.payload);
  const heartbeatReports = reports.filter((report) =>
    isHeartbeatPayload(parsePayload(report.payload)),
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
        setActionFeedback(
          testHealth === "up"
            ? "Core monitor test reported up."
            : `Core monitor test reported ${testHealth}. Review the latest check history row.`,
        );
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
  const latestFailureReason = readString(latestPayload, [
    "failure_reason",
    "message",
    "error",
    "detail",
    "summary",
  ]);
  const latestStatusCode = readString(latestPayload, ["status_code", "code"]);
  const latestResolvedIp = readString(latestPayload, ["resolved_ip", "ip"]);
  const latestTlsExpiry = readString(latestPayload, [
    "tls_days_remaining",
    "tls_expiry",
    "certificate_expiry",
  ]);
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
    mutationErrorMessage(testMonitor.error, "Unable to test Core monitor.") ||
    mutationErrorMessage(pauseMonitor.error, "Unable to pause Core monitor.") ||
    mutationErrorMessage(resumeMonitor.error, "Unable to resume Core monitor.") ||
    mutationErrorMessage(updateMonitor.error, "Unable to save Core monitor.") ||
    mutationErrorMessage(deleteMonitor.error, "Unable to delete Core monitor.");
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
                {isCoreMonitor ? "Core" : "Agent"} · {monitor.type ?? "unknown"} · last checked{" "}
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

      {highlightedIncident && (
        <section className="flex flex-wrap items-center justify-between gap-3 bg-rose-50 px-3 py-2.5 text-sm">
          <div>
            <div className="font-medium text-rose-900">
              Highlighted incident: {highlightedIncident.title ?? highlightedIncident.id}
            </div>
            <div className="text-neutral-600">
              {highlightedIncident.latest_event ?? "No latest event recorded."}
            </div>
          </div>
          <Link
            className="px-2 py-1.5 font-medium text-rose-900 hover:bg-rose-200"
            to={`/incidents/${highlightedIncident.id}`}
          >
            View incident
          </Link>
        </section>
      )}

      <section className="space-y-4">
        <div className="grid gap-3 lg:grid-cols-4">
          <DetailGroup title="Monitor">
            <DetailItem label="health" value={<StatusBadge value={toStatus(health)} />} />
            <DetailItem label="type" value={monitor.type ?? "unknown"} />
            <DetailItem label="interval" value={`${monitor.reporting_interval_seconds ?? 0}s`} />
            <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
          </DetailGroup>

          <DetailGroup title="Owner">
            <DetailItem
              label="owner"
              value={monitor.owner_name ?? monitor.agent_name ?? "Unknown owner"}
            />
            <DetailItem label="source" value={isCoreMonitor ? "Core" : "Agent"} />
            <DetailItem
              label="last success"
              value={formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
            />
            {!isCoreMonitor && monitor.agent_id && (
              <Link
                className="text-sm font-medium hover:text-neutral-600"
                to={`/agents/${monitor.agent_id}?tab=monitors`}
              >
                View agent
              </Link>
            )}
          </DetailGroup>

          <DetailGroup title="Latest Result">
            <DetailItem
              label="checked"
              value={formatDate(reportTimestamp(latestReport), DATE_TIME_FORMAT)}
            />
            <DetailItem label="latency" value={formatLatency(latestPayload)} />
            <DetailItem label="status code" value={latestStatusCode} />
            <DetailItem label="resolved ip" value={latestResolvedIp} />
            <DetailItem label="tls expiry" value={latestTlsExpiry} />
          </DetailGroup>

          <DetailGroup title="Incidents">
            <DetailItem label="active" value={activeIncidents.length} />
            <DetailItem label="related" value={relatedIncidents.length} />
            {relatedIncidents[0] && (
              <Link
                className="text-sm font-medium hover:text-neutral-600"
                to={`/incidents/${relatedIncidents[0].id}`}
              >
                View latest incident
              </Link>
            )}
          </DetailGroup>
        </div>

        <div className="space-y-1">
          <h2 className="text-sm font-medium">Latest Failure Reason</h2>
          <p className="max-w-3xl text-sm text-neutral-600">{latestFailureReason}</p>
        </div>

        {coreConfig?.kind === "heartbeat" && (
          <div className="grid gap-3 lg:grid-cols-2">
            <DetailGroup title="Latest Heartbeat">
              <DetailItem
                label="status"
                value={<StatusBadge value={toStatus(latestHeartbeatReport?.health)} />}
              />
              <DetailItem
                label="time"
                value={formatDate(reportTimestamp(latestHeartbeatReport), DATE_TIME_FORMAT)}
              />
              <DetailItem label="payload" value={heartbeatPayloadContext(latestHeartbeatReport)} />
            </DetailGroup>
            <DetailGroup title="Latest Heartbeat Failure">
              <DetailItem
                label="status"
                value={<StatusBadge value={toStatus(latestHeartbeatFailure?.health)} />}
              />
              <DetailItem
                label="time"
                value={formatDate(reportTimestamp(latestHeartbeatFailure), DATE_TIME_FORMAT)}
              />
              <DetailItem label="payload" value={heartbeatPayloadContext(latestHeartbeatFailure)} />
            </DetailGroup>
          </div>
        )}

        <div className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <DetailItem
              label="90d uptime"
              value={formatUptime(uptimeResponse.data?.uptime_percent)}
            />
            <DetailItem label="days sampled" value={uptimeBuckets.length} />
          </div>
          {recentUptimeBuckets.length > 0 && (
            <div className="flex gap-0.5">
              {recentUptimeBuckets.map((bucket) => (
                <div
                  key={bucket.date}
                  title={`${bucket.date}: ${formatUptime(bucket.uptime_percent)}`}
                  className="flex h-7 w-2 items-end bg-neutral-100"
                >
                  <div
                    className={cn("mt-auto w-full", bucketFillClassName(bucket))}
                    style={{ height: `${Math.max(4, bucket.uptime_percent ?? 0)}%` }}
                  />
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-medium">Operational Data</h2>
        <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-3">
          <TabsList>
            <TabsTrigger value="history">
              Check history <TabCount>{reportCount}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="incidents">
              Incidents <TabCount>{relatedIncidents.length}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="config">Configuration</TabsTrigger>
          </TabsList>
          <TabsContent value="history">
            <div className="space-y-3">
              {historyQuery.error && <div className="text-sm">Unable to load check history.</div>}
              {!historyQuery.error && (
                <DataTable
                  columns={historyColumns}
                  data={reports}
                  emptyMessage="No check history recorded."
                  getRowId={(report, index) =>
                    report.id ?? `${report.monitor_id ?? "monitor"}-${index}`
                  }
                  isLoading={historyQuery.isLoading}
                  loadingMessage="Loading check history..."
                  onRowClick={setSelectedReport}
                />
              )}
              {reportCount > 0 && (
                <ListPagination
                  count={reportCount}
                  limit={HISTORY_LIMIT}
                  offset={historyOffset}
                  onOffsetChange={setHistoryOffset}
                />
              )}
            </div>
          </TabsContent>
          <TabsContent value="incidents">
            {incidentsResponse.error && (
              <div className="text-sm">Unable to load related incidents.</div>
            )}
            {!incidentsResponse.error && (
              <DataTable
                columns={incidentColumns}
                data={relatedIncidents}
                emptyMessage="No related incidents recorded."
                getRowId={(incident, index) => incident.id ?? `incident-${index}`}
                isLoading={incidentsResponse.isLoading}
                loadingMessage="Loading related incidents..."
                rowClassName={(row) =>
                  cn(row.original.id === highlightedIncidentId && "bg-amber-50")
                }
              />
            )}
          </TabsContent>
          <TabsContent value="config">
            <div className="space-y-3">
              {!isCoreMonitor && (
                <div className="grid gap-3 sm:grid-cols-3">
                  <DetailItem label="type" value={monitor.type ?? "unknown"} />
                  <DetailItem
                    label="interval"
                    value={`${monitor.reporting_interval_seconds ?? 0}s`}
                  />
                  <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
                  <DetailItem label="owner" value="Agent configuration" />
                </div>
              )}
              {isCoreMonitor && coreConfigResponse.isLoading && (
                <div className="text-sm text-neutral-600">
                  Loading Core monitor configuration...
                </div>
              )}
              {isCoreMonitor && coreConfigResponse.error && (
                <div className="text-sm">Unable to load Core monitor configuration.</div>
              )}
              {isCoreMonitor && coreConfig && (
                <div className="space-y-4">
                  <div className="grid gap-3 sm:grid-cols-3">
                    <DetailItem label="kind" value={coreConfig.kind ?? monitor.type ?? "unknown"} />
                    <DetailItem label="interval" value={`${coreConfig.interval_seconds ?? 0}s`} />
                    {coreConfig.kind === "heartbeat" ? (
                      <DetailItem
                        label="grace"
                        value={`${coreConfigValue(coreConfig, "grace_seconds")}s`}
                      />
                    ) : (
                      <DetailItem label="timeout" value={`${coreConfig.timeout_seconds ?? 0}s`} />
                    )}
                    <DetailItem
                      label="confirmation"
                      value={`${coreConfig.confirmation_period_seconds ?? 0}s / ${coreConfig.confirmation_check_count ?? 0} checks`}
                    />
                    <DetailItem label="paused" value={coreConfig.paused ? "yes" : "no"} />
                    {coreConfig.kind === "heartbeat" && (
                      <DetailItem
                        label="last signal"
                        value={formatDate(coreConfig.last_signal_at, DATE_TIME_FORMAT)}
                      />
                    )}
                    <DetailItem
                      label="last run"
                      value={formatDate(coreConfig.last_run_at, DATE_TIME_FORMAT)}
                    />
                    <DetailItem
                      label="next run"
                      value={formatDate(coreConfig.next_run_at, DATE_TIME_FORMAT)}
                    />
                  </div>
                  {coreConfig.kind === "heartbeat" && (
                    <HeartbeatSetupPanel config={coreConfig} monitor={monitor} />
                  )}
                  <div className="grid gap-3 lg:grid-cols-2">
                    <div className="space-y-2">
                      <h3 className="text-sm font-medium">Redacted config</h3>
                      <pre className="max-h-80 overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">
                        {formatJSON(coreConfig.config)}
                      </pre>
                    </div>
                    <div className="space-y-2">
                      <h3 className="text-sm font-medium">Secret refs</h3>
                      <pre className="max-h-80 overflow-auto bg-neutral-950 p-3 text-xs text-neutral-50">
                        {formatJSON(coreConfig.secret_refs)}
                      </pre>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </TabsContent>
        </Tabs>
      </section>
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
          error={mutationErrorMessage(updateMonitor.error, "Unable to save Core monitor.")}
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
