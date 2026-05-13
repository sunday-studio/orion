import { PageBreadcrumbs } from "@/components/page-breadcrumbs";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link, useParams } from "react-router-dom";

const INCIDENT_LOOKUP_LIMIT = 1000;

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
  const incidentsResponse = useGetIncidents({ limit: INCIDENT_LOOKUP_LIMIT });
  const incident = (incidentsResponse.data?.incidents ?? []).find((item) => item.id === incidentId);

  if (incidentsResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading incident...</div>;
  }

  if (incidentsResponse.error) {
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
            <p className="text-sm text-neutral-600">
              {incident.status ?? "unknown"} · {incident.severity ?? "unknown"}
            </p>
          </div>
          <div className="text-sm font-medium">
            {incident.notification_status ?? "no notification status"}
          </div>
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Summary</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          <DetailItem label="status" value={incident.status ?? "unknown"} />
          <DetailItem label="severity" value={incident.severity ?? "unknown"} />
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
