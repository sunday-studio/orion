import { IncidentSummary } from "./incident-summary";
import { DataTable } from "@/components/data-table";
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
    cell: ({ row }) => row.original.severity ?? "unknown",
  },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => row.original.status ?? "unknown",
  },
  {
    accessorKey: "agent_name",
    header: "Agent",
    cell: ({ row }) => row.original.agent_name ?? "Unknown agent",
  },
  {
    accessorKey: "monitor_name",
    header: "Monitor",
    cell: ({ row }) => row.original.monitor_name ?? "Unknown monitor",
  },

  {
    accessorKey: "notification_status",
    header: "Notification",
    cell: ({ row }) => row.original.notification_status ?? "—",
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

  const setStatus = (nextStatus: (typeof incidentStatuses)[number]) => {
    void setIncidentQuery({ status: nextStatus, page: 1 });
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
