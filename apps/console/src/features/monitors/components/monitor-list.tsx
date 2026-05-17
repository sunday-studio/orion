import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { Input } from "@/components/ui/input";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ApiMonitorResponse,
  type GetMonitorsParams,
  useGetMonitorSummary,
  useGetMonitors,
} from "@/orion-sdk";
import { type ColumnDef } from "@tanstack/react-table";
import { Search } from "lucide-react";
import {
  parseAsBoolean,
  parseAsInteger,
  parseAsString,
  parseAsStringLiteral,
  useQueryStates,
} from "nuqs";
import { MonitorSummary, type MonitorSummaryFilter } from "./monitor-summary";

const MONITOR_LIMIT = 20;
const monitorHealthFilters = ["all", "up", "down", "degraded", "unknown", "stale"] as const;

const isStaleMonitor = (monitor: ApiMonitorResponse) => {
  return monitor.health === "stale" || monitor.computed_health === "stale";
};

const monitorHealth = (monitor: ApiMonitorResponse) => {
  if (isStaleMonitor(monitor)) return "stale";
  return monitor.health ?? monitor.computed_health ?? "unknown";
};

const columns: ColumnDef<ApiMonitorResponse>[] = [
  {
    accessorKey: "name",
    header: "Monitor",
    cell: ({ row }) => {
      const monitor = row.original;
      return (
        <div className="min-w-56">
          <DataTableLink to={`/monitors/${monitor.id}`} truncate>
            {monitor.name ?? monitor.id ?? "Unknown monitor"}
          </DataTableLink>
        </div>
      );
    },
  },
  {
    id: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge value={toStatus(monitorHealth(row.original))} />,
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "agent_name",
    header: "Agent",
    cell: ({ row }) => {
      const monitor = row.original;
      if (!monitor.agent_id) return monitor.agent_name ?? "Unknown agent";

      return (
        <DataTableLink to={`/agents/${monitor.agent_id}?tab=monitors`}>
          {monitor.agent_name ?? monitor.agent_id}
        </DataTableLink>
      );
    },
  },
  {
    accessorKey: "active_incident_id",
    header: "Incident",
    cell: ({ row }) => {
      const incidentID = row.original.active_incident_id;
      if (!incidentID)
        return row.original.incident_state && row.original.incident_state !== "unknown"
          ? row.original.incident_state
          : "—";

      return <DataTableLink to={`/incidents/${incidentID}`}>active</DataTableLink>;
    },
  },
  {
    accessorKey: "last_successful_report_at",
    header: "Last success",
    cell: ({ row }) => formatDate(row.original.last_successful_report_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "lifecycle",
    header: "Lifecycle",
    cell: ({ row }) => row.original.lifecycle ?? "unknown",
  },
];

export const MonitorList = () => {
  const [{ search, health, incidents, page }, setMonitorQuery] = useQueryStates({
    search: parseAsString.withDefault(""),
    health: parseAsStringLiteral(monitorHealthFilters).withDefault("all"),
    incidents: parseAsBoolean.withDefault(false),
    page: parseAsInteger.withDefault(1),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * MONITOR_LIMIT;

  const params: GetMonitorsParams = {
    limit: MONITOR_LIMIT,
    offset,
    search: search.trim() || undefined,
    health: health === "all" ? undefined : health,
    has_incidents: incidents || undefined,
    sort: "updated_at",
    order: "desc",
  };

  const monitorsResponse = useGetMonitors(params);
  const summaryResponse = useGetMonitorSummary();
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const selectedSummaryFilter: MonitorSummaryFilter = incidents ? "incidents" : health;

  const setOffset = (nextOffset: number) => {
    void setMonitorQuery({ page: Math.floor(nextOffset / MONITOR_LIMIT) + 1 });
  };

  const setSummaryFilter = (filter: MonitorSummaryFilter) => {
    void setMonitorQuery({
      health: filter === "incidents" ? "all" : filter,
      incidents: filter === "incidents",
      page: 1,
    });
  };

  const setSearch = (nextSearch: string) => {
    void setMonitorQuery({ search: nextSearch, page: 1 });
  };

  return (
    <div>
      <MonitorSummary
        summary={summaryResponse.data}
        selectedFilter={selectedSummaryFilter}
        onFilterChange={setSummaryFilter}
      />
      {summaryResponse.error && <div className="py-3 text-sm">Unable to load monitor summary.</div>}
      <div className="relative my-4 max-w-sm">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-400" />
        <Input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search monitors or agents"
          className="pl-9"
        />
      </div>
      {monitorsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading monitors...</div>
      )}
      {monitorsResponse.error && <div className="py-3 text-sm">Unable to load monitors.</div>}
      {!monitorsResponse.isLoading && !monitorsResponse.error && monitors.length === 0 && (
        <EmptyState
          title="No monitors found"
          description="No monitors match the current filters."
        />
      )}
      {!monitorsResponse.isLoading && !monitorsResponse.error && monitors.length > 0 && (
        <div className="my-6">
          <DataTable
            columns={columns}
            data={monitors}
            emptyMessage="No monitors registered."
            getRowId={(monitor) => monitor.id ?? ""}
          />
        </div>
      )}
      <ListPagination
        count={count}
        limit={MONITOR_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
