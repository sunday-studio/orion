import { IncidentSummary } from "./incident-summary";
import { DataTable } from "@/components/data-table";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { type ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, parseAsStringLiteral, useQueryStates } from "nuqs";
import { Link } from "react-router-dom";

const INCIDENT_LIMIT = 20;
const incidentStatuses = ["all", "open", "acknowledged", "resolved"] as const;

const incidentAgentPath = (incident: ApiIncidentResponse) =>
  incident.agent_id
    ? `/agents/${incident.agent_id}?tab=monitors&incident=${encodeURIComponent(incident.id ?? "")}`
    : undefined;

const incidentMonitorPath = (incident: ApiIncidentResponse) =>
  incident.monitor_id
    ? `/monitors/${incident.monitor_id}?incident=${encodeURIComponent(incident.id ?? "")}`
    : undefined;

const columns: ColumnDef<ApiIncidentResponse>[] = [
  {
    accessorKey: "title",
    header: "Incident",
    cell: ({ row }) => {
      const incident = row.original;
      return (
        <div className="min-w-56">
          <Link to={`/incidents/${incident.id}`} className="block truncate hover:text-neutral-600">
            {incident.title ?? "Untitled incident"}
          </Link>
          {/* <div className="truncate text-neutral-600">
            {incident.latest_event ?? "No recent event"}
          </div> */}
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

      return (
        <Link to={path} className="hover:text-neutral-600">
          {incident.agent_name ?? "Unknown agent"}
        </Link>
      );
    },
  },
  {
    accessorKey: "monitor_name",
    header: "Monitor",
    cell: ({ row }) => {
      const incident = row.original;
      const path = incidentMonitorPath(incident);
      if (!path) return incident.monitor_name ?? "Unknown monitor";

      return (
        <Link to={path} className="hover:text-neutral-600">
          {incident.monitor_name ?? "Unknown monitor"}
        </Link>
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
  const [{ page, status }, setIncidentQuery] = useQueryStates({
    page: parseAsInteger.withDefault(1),
    status: parseAsStringLiteral(incidentStatuses).withDefault("all"),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * INCIDENT_LIMIT;
  const incidentsResponse = useGetIncidents({ limit: INCIDENT_LIMIT, offset });
  const filteredIncidentsResponse = useGetIncidents({
    status: status === "all" ? undefined : status,
    limit: INCIDENT_LIMIT,
    offset,
  });

  const openIncidentsResponse = useGetIncidents({ status: "open", limit: 1 });
  const acknowledgedIncidentsResponse = useGetIncidents({ status: "acknowledged", limit: 1 });
  const resolvedIncidentsResponse = useGetIncidents({ status: "resolved", limit: 1 });
  const incidents = filteredIncidentsResponse.data?.incidents ?? [];
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
    <div className="space-y-3">
      <IncidentSummary
        totalCount={incidentsResponse.data?.count ?? count}
        openCount={openIncidentsResponse.data?.count ?? 0}
        acknowledgedCount={acknowledgedIncidentsResponse.data?.count ?? 0}
        resolvedCount={resolvedIncidentsResponse.data?.count ?? 0}
        visibleIncidents={incidents}
      />
      <div className="flex flex-wrap items-center gap-2">
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="w-44 rounded-full text-xs">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="open">Open</SelectItem>
            <SelectItem value="acknowledged">Acknowledged</SelectItem>
            <SelectItem value="resolved">Resolved</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div>
        <DataTable
          columns={columns}
          data={incidents}
          emptyMessage="No incidents recorded."
          getRowId={(incident) => incident.id ?? ""}
        />
      </div>
      <ListPagination
        count={count}
        limit={INCIDENT_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
