import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
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
const monitorOwnerFilters = ["all", "agent", "core"] as const;
const monitorSourceFilters = ["all", "agent", "core"] as const;
const monitorTypeFilters = [
  "all",
  "http-healthcheck",
  "website",
  "tcp",
  "command",
  "pm2",
  "resource-threshold",
  "docker-container",
  "systemd-service",
  "internal-service",
  "http",
  "http_keyword",
  "expected_status",
  "dns",
  "tls",
  "udp",
  "api_request",
  "domain_expiration",
  "ping",
  "mail",
  "smtp",
  "imap",
  "pop",
  "synthetic",
  "playwright",
] as const;

const monitorTypeOptions: Array<{ value: (typeof monitorTypeFilters)[number]; label: string }> = [
  { value: "all", label: "All types" },
  { value: "http-healthcheck", label: "HTTP healthcheck" },
  { value: "website", label: "Website" },
  { value: "tcp", label: "TCP" },
  { value: "command", label: "Command" },
  { value: "pm2", label: "PM2" },
  { value: "resource-threshold", label: "Resource threshold" },
  { value: "docker-container", label: "Docker" },
  { value: "systemd-service", label: "Systemd" },
  { value: "internal-service", label: "Internal service" },
  { value: "http", label: "HTTP" },
  { value: "http_keyword", label: "HTTP keyword" },
  { value: "expected_status", label: "Expected status" },
  { value: "dns", label: "DNS" },
  { value: "tls", label: "TLS" },
  { value: "udp", label: "UDP" },
  { value: "api_request", label: "API request" },
  { value: "domain_expiration", label: "Domain expiration" },
  { value: "ping", label: "Ping" },
  { value: "mail", label: "Mail" },
  { value: "smtp", label: "SMTP" },
  { value: "imap", label: "IMAP" },
  { value: "pop", label: "POP" },
  { value: "synthetic", label: "Synthetic" },
  { value: "playwright", label: "Playwright" },
];

const monitorStatusOptions: Array<{
  value: (typeof monitorStatusFilters)[number];
  label: string;
}> = [
  { value: "all", label: "All statuses" },
  { value: "up", label: "Up" },
  { value: "down", label: "Down" },
  { value: "degraded", label: "Degraded" },
  { value: "unknown", label: "Unknown" },
  { value: "stale", label: "Stale" },
];

const monitorOwnerOptions: Array<{
  value: (typeof monitorOwnerFilters)[number];
  label: string;
}> = [
  { value: "all", label: "All owners" },
  { value: "agent", label: "Server" },
  { value: "core", label: "Core" },
];

const monitorSourceOptions: Array<{
  value: (typeof monitorSourceFilters)[number];
  label: string;
}> = [
  { value: "all", label: "All sources" },
  { value: "agent", label: "Server" },
  { value: "core", label: "Core" },
];

const isStaleMonitor = (monitor: ApiMonitorResponse) => {
  return monitor.health === "stale" || monitor.computed_health === "stale";
};

const monitorHealth = (monitor: ApiMonitorResponse) => {
  if (isStaleMonitor(monitor)) return "stale";
  return monitor.health ?? monitor.computed_health ?? "unknown";
};

const ownerLabel = (monitor: ApiMonitorResponse) => {
  if (monitor.owner_kind === "core" || monitor.source === "core") return "Core";
  return "Server";
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
    accessorKey: "owner_name",
    header: "Owner",
    cell: ({ row }) => {
      const monitor = row.original;
      const kind = ownerLabel(monitor);
      const name = monitor.owner_name ?? monitor.agent_name ?? monitor.agent_id ?? "Unknown owner";
      const labelClass =
        kind === "Core"
          ? "border-sky-200 bg-sky-50 text-sky-700"
          : "border-emerald-200 bg-emerald-50 text-emerald-700";
      const owner = (
        <div className="flex min-w-44 flex-wrap items-center gap-2">
          <span className={`rounded border px-2 py-0.5 text-xs font-medium ${labelClass}`}>
            {kind}
          </span>
          <span className="truncate">{name}</span>
        </div>
      );

      if (monitor.owner_kind === "core" || monitor.source === "core" || !monitor.agent_id) {
        return owner;
      }

      return (
        <DataTableLink to={`/servers/${monitor.agent_id}?tab=monitors`}>{owner}</DataTableLink>
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
  const [{ search, status, type, owner, ownerName, source, incidents, page }, setMonitorQuery] =
    useQueryStates({
      search: parseAsString.withDefault(""),
      status: parseAsStringLiteral(monitorStatusFilters).withDefault("all"),
      type: parseAsStringLiteral(monitorTypeFilters).withDefault("all"),
      owner: parseAsStringLiteral(monitorOwnerFilters).withDefault("all"),
      ownerName: parseAsString.withDefault(""),
      source: parseAsStringLiteral(monitorSourceFilters).withDefault("all"),
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
    owner_kind: owner === "all" ? undefined : owner,
    owner_name: ownerName.trim() || undefined,
    source: source === "all" ? undefined : source,
    has_incidents: incidents || undefined,
    sort: "updated_at",
    order: "desc",
  };

  const monitorsResponse = useGetMonitors(params);
  const summaryResponse = useGetMonitorSummary();
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const selectedSummaryFilter: MonitorSummaryFilter = incidents ? "incidents" : status;
  const hasFilters =
    Boolean(search.trim()) ||
    status !== "all" ||
    type !== "all" ||
    owner !== "all" ||
    Boolean(ownerName.trim()) ||
    source !== "all" ||
    incidents;
  const statusLabel =
    monitorStatusOptions.find((option) => option.value === status)?.label ?? status;
  const typeLabel = monitorTypeOptions.find((option) => option.value === type)?.label ?? type;
  const ownerFilterLabel =
    monitorOwnerOptions.find((option) => option.value === owner)?.label ?? owner;
  const sourceLabel =
    monitorSourceOptions.find((option) => option.value === source)?.label ?? source;

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

  const setOwner = (nextOwner: string) => {
    if (!monitorOwnerFilters.includes(nextOwner as (typeof monitorOwnerFilters)[number])) return;
    void setMonitorQuery({ owner: nextOwner as (typeof monitorOwnerFilters)[number], page: 1 });
  };

  const setOwnerName = (nextOwnerName: string) => {
    void setMonitorQuery({ ownerName: nextOwnerName, page: 1 });
  };

  const setSource = (nextSource: string) => {
    if (!monitorSourceFilters.includes(nextSource as (typeof monitorSourceFilters)[number])) return;
    void setMonitorQuery({
      source: nextSource as (typeof monitorSourceFilters)[number],
      page: 1,
    });
  };

  const clearFilters = () => {
    void setMonitorQuery({
      search: "",
      status: "all",
      type: "all",
      owner: "all",
      ownerName: "",
      source: "all",
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
      {summaryResponse.error && (
        <EmptyState
          className="min-h-32"
          title="Unable to load monitor summary"
          description="The monitor list can still load, but the summary filters may be stale."
          tone="error"
          action={
            <Button size="sm" variant="outline" onClick={() => void summaryResponse.refetch()}>
              Retry
            </Button>
          }
        />
      )}
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
          <SelectTrigger className="min-w-44 text-xs">
            <span data-slot="select-value">Status: {statusLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {monitorStatusOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={type} onValueChange={setType}>
          <SelectTrigger className="min-w-48 text-xs">
            <span data-slot="select-value">Type: {typeLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {monitorTypeOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={owner} onValueChange={setOwner}>
          <SelectTrigger className="min-w-44 text-xs">
            <span data-slot="select-value">Owner: {ownerFilterLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {monitorOwnerOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select value={source} onValueChange={setSource}>
          <SelectTrigger className="min-w-44 text-xs">
            <span data-slot="select-value">Source: {sourceLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {monitorSourceOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Input
          value={ownerName}
          onChange={(event) => setOwnerName(event.target.value)}
          placeholder="Owner name"
          className="w-full max-w-48"
        />
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Clear
          </Button>
        )}
      </div>
      {monitorsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading monitors...</div>
      )}
      {monitorsResponse.error && (
        <EmptyState
          title="Unable to load monitors"
          description="Retry after Core is reachable."
          tone="error"
          action={
            <Button size="sm" variant="outline" onClick={() => void monitorsResponse.refetch()}>
              Retry
            </Button>
          }
        />
      )}
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
      {count > 0 && (
        <ListPagination
          count={count}
          limit={MONITOR_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
