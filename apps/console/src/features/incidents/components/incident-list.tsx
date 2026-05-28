import { IncidentSummary, type IncidentSummaryStatus } from "./incident-summary";
import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import {
  NotificationBadge,
  SeverityBadge,
  StatusBadge,
  toNotificationStatus,
  toSeverity,
  toStatus,
} from "@/components/status-badges";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { ListPagination } from "@/components/list-pagination";
import { type ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, parseAsString, parseAsStringLiteral, useQueryStates } from "nuqs";

const INCIDENT_LIMIT = 20;
const incidentStatuses = ["all", "open", "acknowledged", "covered", "resolved", "errors"] as const;
const allIncidentStatuses = "open,acknowledged,covered,resolved";

const incidentAgentPath = (incident: ApiIncidentResponse) =>
  incident.agent_id
    ? `/agents/${incident.agent_id}?tab=monitors&incident=${encodeURIComponent(incident.id ?? "")}`
    : undefined;

const incidentMonitorPath = (incident: ApiIncidentResponse) =>
  incident.monitor_id
    ? `/monitors/${incident.monitor_id}?incident=${encodeURIComponent(incident.id ?? "")}`
    : undefined;

const componentLabel = (
  component: NonNullable<ApiIncidentResponse["impacted_components"]>[number],
) => component.component_name || component.component_id || "Unnamed component";

const componentImpactLabel = (
  component: NonNullable<ApiIncidentResponse["impacted_components"]>[number],
) => component.impact || component.status || "";

const statusParam = (status: IncidentSummaryStatus) => {
  if (status === "all" || status === "errors") return allIncidentStatuses;
  return status;
};

const columns: ColumnDef<ApiIncidentResponse>[] = [
  {
    accessorKey: "title",
    header: "Incident",
    cell: ({ row }) => {
      const incident = row.original;
      return (
        <div className="min-w-56">
          <DataTableLink to={`/incidents/${incident.id}`} truncate>
            {incident.title ?? "Untitled incident"}
          </DataTableLink>
        </div>
      );
    },
  },
  {
    accessorKey: "severity",
    header: "Severity",
    cell: ({ row }) => <SeverityBadge value={toSeverity(row.original.severity)} />,
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge value={toStatus(row.original.status)} />,
  },
  {
    accessorKey: "agent_name",
    header: "Agent",
    cell: ({ row }) => {
      const incident = row.original;
      const path = incidentAgentPath(incident);
      if (!path) return incident.agent_name ?? "Unknown agent";

      return <DataTableLink to={path}>{incident.agent_name ?? "Unknown agent"}</DataTableLink>;
    },
  },
  {
    accessorKey: "monitor_name",
    header: "Monitor",
    cell: ({ row }) => {
      const incident = row.original;
      const path = incidentMonitorPath(incident);
      if (!path) return incident.monitor_name ?? "Unknown monitor";

      return <DataTableLink to={path}>{incident.monitor_name ?? "Unknown monitor"}</DataTableLink>;
    },
  },
  {
    accessorKey: "impacted_components",
    header: "Components",
    cell: ({ row }) => {
      const components = row.original.impacted_components ?? [];
      if (components.length === 0) {
        return <span className="text-neutral-500">No components</span>;
      }

      return (
        <div className="max-w-48 space-y-1">
          {components.slice(0, 2).map((component, index) => {
            const impact = componentImpactLabel(component);
            return (
              <div
                key={`${component.component_id ?? component.component_name ?? "component"}-${index}`}
                className="truncate text-sm"
              >
                <span className="font-medium">{componentLabel(component)}</span>
                {impact && <span className="text-neutral-500"> / {impact}</span>}
              </div>
            );
          })}
          {components.length > 2 && (
            <div className="text-xs text-neutral-500">+{components.length - 2} more</div>
          )}
        </div>
      );
    },
  },
  {
    accessorKey: "notification_status",
    header: "Notification",
    cell: ({ row }) => (
      <NotificationBadge
        value={toNotificationStatus(row.original.notification_status)}
        fallback="—"
      />
    ),
  },
  {
    accessorKey: "opened_at",
    header: "Opened",
    cell: ({ row }) => formatDate(row.original.opened_at, DATE_TIME_FORMAT),
  },
];

export const IncidentList = () => {
  const [{ page, status, agent }, setIncidentQuery] = useQueryStates({
    agent: parseAsString.withDefault(""),
    page: parseAsInteger.withDefault(1),
    status: parseAsStringLiteral(incidentStatuses).withDefault("all"),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * INCIDENT_LIMIT;
  const incidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    status: allIncidentStatuses,
    limit: INCIDENT_LIMIT,
    offset,
  });
  const filteredIncidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    needs_review: status === "errors" ? true : undefined,
    status: statusParam(status),
    limit: INCIDENT_LIMIT,
    offset,
  });

  const openIncidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    status: "open",
    limit: 1,
    offset: 0,
  });
  const acknowledgedIncidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    status: "acknowledged",
    limit: 1,
    offset: 0,
  });
  const resolvedIncidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    status: "resolved",
    limit: 1,
    offset: 0,
  });
  const coveredIncidentsResponse = useGetIncidents({
    agent_id: agent || undefined,
    status: "covered",
    limit: 1,
    offset: 0,
  });
  const responseIncidents = filteredIncidentsResponse.data?.incidents ?? [];
  const incidents = responseIncidents;
  const count = filteredIncidentsResponse.data?.count ?? incidents.length;

  const setStatus = (nextStatus: string) => {
    if (!incidentStatuses.includes(nextStatus as (typeof incidentStatuses)[number])) return;
    void setIncidentQuery({ status: nextStatus as (typeof incidentStatuses)[number], page: 1 });
  };

  const setOffset = (nextOffset: number) => {
    void setIncidentQuery({ page: Math.floor(nextOffset / INCIDENT_LIMIT) + 1 });
  };

  if (incidentsResponse.isLoading || filteredIncidentsResponse.isLoading) {
    return <div className="py-3 text-sm text-neutral-600">Loading incidents...</div>;
  }

  if (incidentsResponse.error || filteredIncidentsResponse.error) {
    return <div className="py-3 text-sm">Unable to load incidents.</div>;
  }

  return (
    <div className="space-y-5">
      <IncidentSummary
        totalCount={incidentsResponse.data?.count ?? count}
        openCount={openIncidentsResponse.data?.count ?? 0}
        acknowledgedCount={acknowledgedIncidentsResponse.data?.count ?? 0}
        coveredCount={coveredIncidentsResponse.data?.count ?? 0}
        resolvedCount={resolvedIncidentsResponse.data?.count ?? 0}
        visibleIncidents={incidents}
        selectedStatus={status}
        onStatusChange={setStatus}
      />
      <div>
        <DataTable
          columns={columns}
          data={incidents}
          emptyMessage="No incidents recorded."
          getRowId={(incident) => incident.id ?? ""}
        />
      </div>
      {count > 0 && (
        <ListPagination
          count={count}
          limit={INCIDENT_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
