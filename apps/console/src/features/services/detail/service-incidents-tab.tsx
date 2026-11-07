import type { ColumnDef } from "@tanstack/react-table";
import { useNavigate } from "react-router-dom";
import { DataTable, StatusBadge } from "@/design-system";
import { incidentLabel, statusLabel } from "@/features/incidents/incident-format";
import { type ApiIncidentResponse, useGetIncidents } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { incidentsForMonitors } from "../services.domain";

const columns: ColumnDef<ApiIncidentResponse>[] = [
  { id: "id", header: "Incident", cell: ({ row }) => incidentLabel(row.original) },
  { id: "title", header: "Title", cell: ({ row }) => row.original.title ?? row.original.summary },
  { id: "monitor", header: "Monitor", cell: ({ row }) => row.original.monitor_name ?? "-" },
  {
    id: "status",
    header: "Status",
    cell: ({ row }) => (
      <StatusBadge status={row.original.status}>{statusLabel(row.original.status)}</StatusBadge>
    ),
  },
  {
    id: "opened",
    header: "Opened",
    cell: ({ row }) => formatDate(row.original.opened_at, DATE_TIME_FORMAT),
  },
];

/** Incidents on the service's monitors. Core has no service filter yet, so this narrows the latest 200 here. */
export const ServiceIncidentsTab = ({ monitorIds }: { monitorIds: readonly string[] }) => {
  const navigate = useNavigate();
  const query = useGetIncidents({ limit: 200 });
  const rows = incidentsForMonitors(query.data?.incidents ?? [], monitorIds);
  return (
    <DataTable
      columns={columns}
      data={rows}
      isLoading={query.isPending}
      error={
        query.isError
          ? { message: "Unable to load incidents", onRetry: () => void query.refetch() }
          : undefined
      }
      getRowId={(incident, index) => incident.id ?? `incident-${index}`}
      emptyMessage={
        monitorIds.length === 0
          ? "Link a monitor to see its incidents here."
          : "No recent incidents on this service's monitors."
      }
      onRowClick={(incident) => navigate(`/incidents/${incident.id}`)}
    />
  );
};
