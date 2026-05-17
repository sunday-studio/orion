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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiAlertDeliveryResponse,
  type ApiIncidentResponse,
  type ApiIncidentTimelineItemResponse,
  type ApiMonitorReportResponse,
  useGetIncident,
} from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import type { ReactNode } from "react";
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
  const [searchParams, setSearchParams] = useSearchParams();
  const incidentResponse = useGetIncident(incidentId);
  const incident = incidentResponse.data?.incident;
  const timeline = incidentResponse.data?.timeline ?? [];
  const alertDeliveries = incidentResponse.data?.alert_deliveries ?? [];
  const monitorReports = incidentResponse.data?.monitor_reports ?? [];
  const requestedTab = searchParams.get("tab");
  const activeTab: DetailTab = isDetailTab(requestedTab) ? requestedTab : "timeline";

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
        <div className="space-y-2">
          <h1 className="text-base font-medium">{incident.title ?? "Untitled incident"}</h1>
          <p className="max-w-3xl text-sm text-neutral-600">
            {incident.latest_event ?? "No latest event recorded."}
          </p>
        </div>

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

      <section className="space-y-4">
        <h2 className="text-sm font-medium">Operational Data</h2>
        <Tabs value={activeTab} onValueChange={handleTabChange} className="space-y-3">
          <TabsList>
            <TabsTrigger value="timeline">Timeline ({timeline.length})</TabsTrigger>
            <TabsTrigger value="notifications">
              Notifications ({alertDeliveries.length})
            </TabsTrigger>
            <TabsTrigger value="monitor-reports">
              Monitor reports ({monitorReports.length})
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
            />
          </TabsContent>
        </Tabs>
      </section>
    </div>
  );
};
