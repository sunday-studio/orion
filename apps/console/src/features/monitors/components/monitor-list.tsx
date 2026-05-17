import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
const monitorStatusFilters = ["all", "up", "down", "degraded", "unknown", "stale"] as const;
const monitorTypeFilters = [
  "all",
  "http-healthcheck",
  "website",
  "tcp",
  "command",
  "pm2",
  "resource-threshold",
  "docker",
  "systemd",
  "internal-service",
  "http",
] as const;

const monitorTypeOptions: Array<{ value: (typeof monitorTypeFilters)[number]; label: string }> = [
  { value: "all", label: "All types" },
  { value: "http-healthcheck", label: "HTTP healthcheck" },
  { value: "website", label: "Website" },
  { value: "tcp", label: "TCP" },
  { value: "command", label: "Command" },
  { value: "pm2", label: "PM2" },
  { value: "resource-threshold", label: "Resource threshold" },
  { value: "docker", label: "Docker" },
  { value: "systemd", label: "Systemd" },
  { value: "internal-service", label: "Internal service" },
  { value: "http", label: "HTTP" },
];

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
  const [{ search, status, type, incidents, page }, setMonitorQuery] = useQueryStates({
    search: parseAsString.withDefault(""),
    status: parseAsStringLiteral(monitorStatusFilters).withDefault("all"),
    type: parseAsStringLiteral(monitorTypeFilters).withDefault("all"),
    incidents: parseAsBoolean.withDefault(false),
    page: parseAsInteger.withDefault(1),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * MONITOR_LIMIT;

  const params: GetMonitorsParams = {
    limit: MONITOR_LIMIT,
    offset,
    search: search.trim() || undefined,
    health: status === "all" ? undefined : status,
    type: type === "all" ? undefined : type,
    has_incidents: incidents || undefined,
    sort: "updated_at",
    order: "desc",
  };

  const monitorsResponse = useGetMonitors(params);
  const summaryResponse = useGetMonitorSummary();
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const selectedSummaryFilter: MonitorSummaryFilter = incidents ? "incidents" : status;
  const hasFilters = Boolean(search.trim()) || status !== "all" || type !== "all" || incidents;

  const setOffset = (nextOffset: number) => {
    void setMonitorQuery({ page: Math.floor(nextOffset / MONITOR_LIMIT) + 1 });
  };

  const setSummaryFilter = (filter: MonitorSummaryFilter) => {
    void setMonitorQuery({
      status: filter === "incidents" ? "all" : filter,
      incidents: filter === "incidents",
      page: 1,
    });
  };

  const setSearch = (nextSearch: string) => {
    void setMonitorQuery({ search: nextSearch, page: 1 });
  };

  const setStatus = (nextStatus: string) => {
    if (!monitorStatusFilters.includes(nextStatus as (typeof monitorStatusFilters)[number])) return;
    void setMonitorQuery({
      status: nextStatus as (typeof monitorStatusFilters)[number],
      incidents: false,
      page: 1,
    });
  };

  const setType = (nextType: string) => {
    if (!monitorTypeFilters.includes(nextType as (typeof monitorTypeFilters)[number])) return;
    void setMonitorQuery({ type: nextType as (typeof monitorTypeFilters)[number], page: 1 });
  };

  const clearFilters = () => {
    void setMonitorQuery({
      search: "",
      status: "all",
      type: "all",
      incidents: false,
      page: 1,
    });
  };

  return (
    <div>
      <MonitorSummary
        summary={summaryResponse.data}
        selectedFilter={selectedSummaryFilter}
        onFilterChange={setSummaryFilter}
      />
      {summaryResponse.error && <div className="py-3 text-sm">Unable to load monitor summary.</div>}
      <div className="mt-6 flex flex-wrap items-center gap-2">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-400" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search monitors"
            className="pl-9"
          />
        </div>
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="min-w-30 text-xs">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="up">Up</SelectItem>
            <SelectItem value="down">Down</SelectItem>
            <SelectItem value="degraded">Degraded</SelectItem>
            <SelectItem value="unknown">Unknown</SelectItem>
            <SelectItem value="stale">Stale</SelectItem>
          </SelectContent>
        </Select>

        <Select value={type} onValueChange={setType}>
          <SelectTrigger className="min-w-30 text-xs">
            <SelectValue placeholder="All types" />
          </SelectTrigger>
          <SelectContent>
            {monitorTypeOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Clear
          </Button>
        )}
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
        <div className="mt-2">
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
