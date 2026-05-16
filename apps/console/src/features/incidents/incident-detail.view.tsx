import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import {
  NotificationBadge,
  SeverityBadge,
  StatusBadge,
  toNotificationStatus,
  toSeverity,
  toStatus,
} from "@/components/status-badges";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { type ApiIncidentResponse, useGetIncident } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link, useParams } from "react-router-dom";

const DetailItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
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

export const IncidentDetailPage = () => {
  const { incidentId = "" } = useParams();
  const incidentResponse = useGetIncident(incidentId);
  const incident = incidentResponse.data?.incident;
  const timeline = incidentResponse.data?.timeline ?? [];
  const alertDeliveries = incidentResponse.data?.alert_deliveries ?? [];
  const monitorReports = incidentResponse.data?.monitor_reports ?? [];

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
        <div className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-base font-medium">{incident.title ?? "Untitled incident"}</h1>
            <div className="mt-2 flex flex-wrap gap-2">
              <StatusBadge value={toStatus(incident.status)} />
              <SeverityBadge value={toSeverity(incident.severity)} />
            </div>
          </div>
          <NotificationBadge
            value={toNotificationStatus(incident.notification_status)}
            fallback="no notification status"
          />
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Summary</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <div>
            <div className="text-sm text-neutral-600">status</div>
            <StatusBadge value={toStatus(incident.status)} />
          </div>
          <div>
            <div className="text-sm text-neutral-600">severity</div>
            <SeverityBadge value={toSeverity(incident.severity)} />
          </div>
          <DetailItem label="duration" value={durationLabel(incident)} />
          <DetailItem label="opened" value={formatDate(incident.opened_at, DATE_TIME_FORMAT)} />
          <DetailItem label="resolved" value={formatDate(incident.resolved_at, DATE_TIME_FORMAT)} />
          <DetailItem
            label="latest event"
            value={formatDate(incident.last_event_at, DATE_TIME_FORMAT)}
          />
        </div>
        <p className="text-sm text-neutral-600">{incident.latest_event ?? "No latest event."}</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Affected Data</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="agent" value={incident.agent_name ?? "Unknown agent"} />
          <DetailItem label="monitor" value={incident.monitor_name ?? "Unknown monitor"} />
          <DetailItem label="monitor type" value={incident.monitor_type ?? "unknown"} />
        </div>
        <div className="flex flex-wrap gap-4 text-sm">
          {incident.agent_id && (
            <Link
              to={`/agents/${incident.agent_id}`}
              className="font-medium hover:text-neutral-600"
            >
              View agent
            </Link>
          )}
          {incident.monitor_id && (
            <Link
              to={`/monitors/${incident.monitor_id}`}
              className="font-medium hover:text-neutral-600"
            >
              View monitor
            </Link>
          )}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Timeline</h2>
        {timeline.length === 0 && (
          <div className="text-sm text-neutral-600">No timeline events recorded.</div>
        )}
        {timeline.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Source</TableHead>
                <TableHead>Message</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {timeline.map((item) => (
                <TableRow key={item.id}>
                  <TableCell className="font-medium">
                    {formatDate(item.created_at, DATE_TIME_FORMAT)}
                  </TableCell>
                  <TableCell>{item.type ?? "unknown"}</TableCell>
                  <TableCell>{item.source ?? "unknown"}</TableCell>
                  <TableCell className="max-w-[28rem] truncate text-neutral-600">
                    {item.message ?? "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Linked Data</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="alert deliveries" value={alertDeliveries.length} />
          <DetailItem label="monitor reports" value={monitorReports.length} />
          <DetailItem label="timeline events" value={timeline.length} />
        </div>
        {alertDeliveries.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Channel</TableHead>
                <TableHead>Event</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Error</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {alertDeliveries.map((delivery) => (
                <TableRow key={delivery.id}>
                  <TableCell className="font-medium">
                    {formatDate(delivery.created_at, DATE_TIME_FORMAT)}
                  </TableCell>
                  <TableCell>{delivery.channel ?? "none"}</TableCell>
                  <TableCell>{delivery.event_type ?? "unknown"}</TableCell>
                  <TableCell>
                    <NotificationBadge value={toNotificationStatus(delivery.status)} />
                  </TableCell>
                  <TableCell className="max-w-[24rem] truncate text-neutral-600">
                    {delivery.error ?? "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
        {monitorReports.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Health</TableHead>
                <TableHead>Report ID</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {monitorReports.map((report) => (
                <TableRow key={report.id}>
                  <TableCell className="font-medium">
                    {formatDate(report.created_at ?? report.collected_at, DATE_TIME_FORMAT)}
                  </TableCell>
                  <TableCell>{report.health ?? "unknown"}</TableCell>
                  <TableCell className="max-w-[24rem] truncate text-neutral-600">
                    {report.id ?? "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Record</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="created" value={formatDate(incident.created_at, DATE_TIME_FORMAT)} />
          <DetailItem label="updated" value={formatDate(incident.updated_at, DATE_TIME_FORMAT)} />
          <DetailItem label="incident id" value={incident.id ?? "—"} />
        </div>
      </section>
    </div>
  );
};
