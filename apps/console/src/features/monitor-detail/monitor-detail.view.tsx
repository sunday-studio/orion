import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { PageHeader } from "@/components/page-header";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type ApiIncidentResponse,
  type ApiMonitorReportResponse,
  useGetIncident,
  useGetIncidents,
  useGetMonitor,
  useGetMonitorHistory,
  useGetMonitorUptime,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import type { ColumnDef } from "@tanstack/react-table";
import { type ReactNode, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";

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
      "status",
      "status_code",
    ]) ?? "—"
  );
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

const reportTimestamp = (report?: ApiMonitorReportResponse) =>
  report?.created_at ?? report?.collected_at;

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
  const [historyPage, setHistoryPage] = useState(1);
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
  const incidentsResponse = useGetIncidents({ limit: 100 });
  const highlightedIncidentResponse = useGetIncident(highlightedIncidentId);

  const monitor = monitorResponse.data?.monitor;
  const reports = historyQuery.data?.reports ?? [];
  const reportCount = historyQuery.data?.count ?? reports.length;
  const latestReport = monitorResponse.data?.recent_reports?.[0] ?? reports[0];
  const latestPayload = parsePayload(latestReport?.payload);
  const uptimeBuckets = uptimeResponse.data?.daily_buckets ?? [];
  const recentUptimeBuckets = uptimeBuckets.slice(-7);
  const health =
    monitorResponse.data?.computed_health ??
    monitor?.computed_health ??
    monitor?.health ??
    "unknown";
  const relatedIncidents = (incidentsResponse.data?.incidents ?? []).filter(
    (incident) => incident.monitor_id === monitorId,
  );
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
    setHistoryPage(Math.floor(nextOffset / HISTORY_LIMIT) + 1);
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
        <PageHeader
          title={monitor.name ?? monitor.id ?? "Unknown monitor"}
          description={
            <p className="text-sm text-neutral-600">
              <StatusBadge className="px-1.5 py-0.5 text-[13px]" value={toStatus(health)} /> ·{" "}
              {monitor.type ?? "unknown"} · last checked{" "}
              {formatDate(reportTimestamp(latestReport), DATE_TIME_FORMAT)}
            </p>
          }
        />
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
            <DetailItem label="agent" value={monitor.agent_name ?? "Unknown agent"} />
            <DetailItem
              label="last success"
              value={formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
            />
            {monitor.agent_id && (
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

        <div className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-3">
            <DetailItem
              label="90d uptime"
              value={formatUptime(uptimeResponse.data?.uptime_percent)}
            />
            <DetailItem label="days sampled" value={uptimeBuckets.length} />
          </div>
          {recentUptimeBuckets.length > 0 && (
            <div className="grid gap-2 sm:grid-cols-7">
              {recentUptimeBuckets.map((bucket) => (
                <div key={bucket.date} className="text-sm">
                  <div className="text-neutral-600">{bucket.date ?? "—"}</div>
                  <div className="font-medium">{formatUptime(bucket.uptime_percent)}</div>
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
              <div className="grid gap-3 sm:grid-cols-3">
                <DetailItem label="type" value={monitor.type ?? "unknown"} />
                <DetailItem
                  label="interval"
                  value={`${monitor.reporting_interval_seconds ?? 0}s`}
                />
                <DetailItem label="lifecycle" value={monitor.lifecycle ?? "unknown"} />
                <DetailItem label="expected status/body/regex" value="config-owned" />
                <DetailItem label="thresholds" value="config-owned" />
                <DetailItem label="alerts" value="config-owned" />
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </section>
    </div>
  );
};
