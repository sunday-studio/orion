import { DataTable } from "@/components/data-table";
import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import {
  NotificationBadge,
  SeverityBadge,
  StatusBadge,
  toNotificationStatus,
  toSeverity,
  toStatus,
} from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { TabCount, Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ReportInspectionDrawer } from "@/features/report-inspection/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiAlertDeliveryResponse,
  type ApiIncidentResponse,
  type ApiIncidentTimelineItemResponse,
  type ApiMonitorReportResponse,
  getGetIncidentQueryKey,
  getGetIncidentTimelineQueryKey,
  useAcknowledgeIncident,
  useGetIncident,
  useResolveIncident,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { CheckIcon, CircleCheckIcon } from "lucide-react";
import { type ReactNode, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";

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

const durationLabel = (incident: ApiIncidentResponse) => {
  const start = incident.opened_at ? new Date(incident.opened_at).getTime() : undefined;
  const end = incident.resolved_at ? new Date(incident.resolved_at).getTime() : Date.now();
  if (!start || Number.isNaN(start) || Number.isNaN(end)) return "—";
  const seconds = Math.max(0, Math.floor((end - start) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
};

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

const readPayloadValue = (payload: MonitorPayload, keys: string[]) => {
  for (const key of keys) {
    const value = payload[key];
    if (typeof value === "string" && value.trim() !== "") return value;
    if (typeof value === "number") return String(value);
    if (typeof value === "boolean") return value ? "true" : "false";
  }
  return "—";
};

const reportTimestamp = (report?: ApiMonitorReportResponse) =>
  report?.created_at ?? report?.collected_at;

const reportReason = (report?: ApiMonitorReportResponse) => {
  if (!report) return "No linked monitor report.";
  const payload = parsePayload(report.payload);
  return readPayloadValue(payload, [
    "failure_reason",
    "error",
    "message",
    "summary",
    "status",
    "status_code",
  ]);
};

const reportSortTime = (report: ApiMonitorReportResponse) => {
  const timestamp = reportTimestamp(report);
  const value = timestamp ? new Date(timestamp).getTime() : 0;
  return Number.isNaN(value) ? 0 : value;
};

const detailTabs = ["timeline", "notifications", "monitor-reports"] as const;
type DetailTab = (typeof detailTabs)[number];

const isDetailTab = (value: string | null): value is DetailTab =>
  detailTabs.includes(value as DetailTab);

const timelineColumns: ColumnDef<ApiIncidentTimelineItemResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "source",
    header: "Source",
    cell: ({ row }) => row.original.source ?? "unknown",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[28rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
];

const notificationColumns: ColumnDef<ApiAlertDeliveryResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "channel",
    header: "Channel",
    cell: ({ row }) => row.original.channel ?? "none",
  },
  {
    accessorKey: "event_type",
    header: "Event",
    cell: ({ row }) => row.original.event_type ?? "unknown",
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <NotificationBadge value={toNotificationStatus(row.original.status)} />,
  },
  {
    accessorKey: "error",
    header: "Error",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.error ?? "—"}</div>
    ),
  },
];

const monitorReportColumns: ColumnDef<ApiMonitorReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.collected_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.health)} />,
  },
  {
    accessorKey: "id",
    header: "Report ID",
    cell: ({ row }) => (
      <div className="max-w-[24rem] truncate text-neutral-600">{row.original.id ?? "—"}</div>
    ),
  },
];

export const IncidentDetailPage = () => {
  const { incidentId = "" } = useParams();
  const queryClient = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const refreshIncident = () => {
    void queryClient.invalidateQueries({ queryKey: getGetIncidentQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: getGetIncidentTimelineQueryKey(incidentId) });
    void queryClient.invalidateQueries({ queryKey: ["/v1/incidents"] });
  };
  const incidentResponse = useGetIncident(incidentId);
  const acknowledgeIncident = useAcknowledgeIncident({ mutation: { onSuccess: refreshIncident } });
  const resolveIncident = useResolveIncident({ mutation: { onSuccess: refreshIncident } });
  const incident = incidentResponse.data?.incident;
  const timeline = incidentResponse.data?.timeline ?? [];
  const alertDeliveries = incidentResponse.data?.alert_deliveries ?? [];
  const monitorReports = incidentResponse.data?.monitor_reports ?? [];
  const [selectedMonitorReport, setSelectedMonitorReport] = useState<ApiMonitorReportResponse>();
  const sortedMonitorReports = [...monitorReports].sort(
    (a, b) => reportSortTime(a) - reportSortTime(b),
  );
  const triggeringReport =
    sortedMonitorReports.find((report) => report.health && report.health !== "up") ??
    sortedMonitorReports[0];
  const latestReport = sortedMonitorReports.at(-1);
  const latestTimelineItem = timeline[0];
  const requestedTab = searchParams.get("tab");
  const activeTab: DetailTab = isDetailTab(requestedTab) ? requestedTab : "timeline";
  const canAcknowledge = incident?.status === "open";
  const canResolve = incident?.status !== "resolved";
  const actionPending = acknowledgeIncident.isPending || resolveIncident.isPending;

  const handleTabChange = (tab: string) => {
    if (!isDetailTab(tab)) return;
    setSearchParams(
      (params) => {
        params.set("tab", tab);
        return params;
      },
      { replace: true },
    );
  };

  if (incidentResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading incident...</div>;
  }

  if (incidentResponse.error) {
    return <div className="py-3 text-sm">Unable to load incident.</div>;
  }

  if (!incident) {
    return <div className="py-3 text-sm text-neutral-600">Incident not found.</div>;
  }

  return (
    <div className="space-y-7">
      <div className="space-y-1">
        <PageBreadcrumbs
          items={[
            { label: "Incidents", to: "/incidents" },
            { label: incident.title ?? "Incident" },
          ]}
        />
      </div>

      <section className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-2">
            <h1 className="text-base font-medium">{incident.title ?? "Untitled incident"}</h1>
            <p className="max-w-3xl text-sm text-neutral-600">
              {incident.latest_event ?? "No latest event recorded."}
            </p>
          </div>
          {canResolve && (
            <div className="flex flex-wrap gap-2">
              {canAcknowledge && (
                <Button
                  variant="outline"
                  disabled={actionPending}
                  onClick={() => acknowledgeIncident.mutate({ id: incident.id ?? "" })}
                >
                  <CheckIcon />
                  Acknowledge
                </Button>
              )}
              <Button
                disabled={actionPending}
                onClick={() => resolveIncident.mutate({ id: incident.id ?? "" })}
              >
                <CircleCheckIcon />
                Resolve
              </Button>
            </div>
          )}
        </div>
        {(acknowledgeIncident.error || resolveIncident.error) && (
          <div className="text-sm text-rose-700">Unable to update incident.</div>
        )}

        <div className="grid gap-3 lg:grid-cols-3">
          <DetailGroup title="Incident">
            <DetailItem label="status" value={<StatusBadge value={toStatus(incident.status)} />} />
            <DetailItem
              label="severity"
              value={<SeverityBadge value={toSeverity(incident.severity)} />}
            />
            <DetailItem
              label="notification"
              value={
                <NotificationBadge
                  value={toNotificationStatus(incident.notification_status)}
                  fallback="no notification status"
                />
              }
            />
            <DetailItem label="duration" value={durationLabel(incident)} />
          </DetailGroup>

          <DetailGroup title="Affected">
            <DetailItem label="agent" value={incident.agent_name ?? "Unknown agent"} />
            <DetailItem label="monitor" value={incident.monitor_name ?? "Unknown monitor"} />
            <DetailItem label="monitor type" value={incident.monitor_type ?? "unknown"} />
            <div className="flex flex-wrap gap-4 text-sm">
              {incident.agent_id && (
                <Link
                  to={`/agents/${incident.agent_id}?tab=monitors&incident=${encodeURIComponent(incident.id ?? "")}`}
                  className="font-medium hover:text-neutral-600"
                >
                  View agent
                </Link>
              )}
              {incident.monitor_id && (
                <Link
                  to={`/monitors/${incident.monitor_id}?incident=${encodeURIComponent(incident.id ?? "")}`}
                  className="font-medium hover:text-neutral-600"
                >
                  View monitor
                </Link>
              )}
            </div>
          </DetailGroup>

          <DetailGroup title="Timing">
            <DetailItem label="opened" value={formatDate(incident.opened_at, DATE_TIME_FORMAT)} />
            <DetailItem
              label="latest event"
              value={formatDate(incident.last_event_at, DATE_TIME_FORMAT)}
            />
            <DetailItem
              label="resolved"
              value={formatDate(incident.resolved_at, DATE_TIME_FORMAT)}
            />
          </DetailGroup>
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Cause / Evidence</h2>
        <div className="grid gap-3 lg:grid-cols-3">
          <DetailGroup title="Trigger">
            <DetailItem
              label="first failing result"
              value={
                <span className="inline-flex items-center gap-2">
                  <StatusBadge value={toStatus(triggeringReport?.health)} />
                  <span>{formatDate(reportTimestamp(triggeringReport), DATE_TIME_FORMAT)}</span>
                </span>
              }
            />
            <DetailItem label="reason" value={reportReason(triggeringReport)} />
          </DetailGroup>

          <DetailGroup title="Current Result">
            <DetailItem
              label="latest report"
              value={
                <span className="inline-flex items-center gap-2">
                  <StatusBadge value={toStatus(latestReport?.health)} />
                  <span>{formatDate(reportTimestamp(latestReport), DATE_TIME_FORMAT)}</span>
                </span>
              }
            />
            <DetailItem label="latest reason" value={reportReason(latestReport)} />
          </DetailGroup>

          <DetailGroup title="Latest Timeline Event">
            <DetailItem label="type" value={latestTimelineItem?.type ?? "—"} />
            <DetailItem
              label="time"
              value={formatDate(latestTimelineItem?.created_at, DATE_TIME_FORMAT)}
            />
            <DetailItem label="message" value={latestTimelineItem?.message ?? "—"} />
          </DetailGroup>
        </div>
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-medium">Operational Data</h2>
        <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-3">
          <TabsList>
            <TabsTrigger value="timeline">
              Timeline <TabCount>{timeline.length}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="notifications">
              Notifications <TabCount>{alertDeliveries.length}</TabCount>
            </TabsTrigger>
            <TabsTrigger value="monitor-reports">
              Monitor reports <TabCount>{monitorReports.length}</TabCount>
            </TabsTrigger>
          </TabsList>
          <TabsContent value="timeline">
            <DataTable
              columns={timelineColumns}
              data={timeline}
              emptyMessage="No timeline events recorded."
              getRowId={(item, index) => item.id ?? `timeline-${index}`}
            />
          </TabsContent>
          <TabsContent value="notifications">
            <DataTable
              columns={notificationColumns}
              data={alertDeliveries}
              emptyMessage="No notification deliveries recorded."
              getRowId={(delivery, index) => delivery.id ?? `notification-${index}`}
            />
          </TabsContent>
          <TabsContent value="monitor-reports">
            <DataTable
              columns={monitorReportColumns}
              data={monitorReports}
              emptyMessage="No monitor reports linked."
              getRowId={(report, index) => report.id ?? `monitor-report-${index}`}
              onRowClick={setSelectedMonitorReport}
            />
          </TabsContent>
        </Tabs>
      </section>
      <ReportInspectionDrawer
        kind="monitor"
        report={selectedMonitorReport}
        onOpenChange={(open) => {
          if (!open) setSelectedMonitorReport(undefined);
        }}
      />
    </div>
  );
};
