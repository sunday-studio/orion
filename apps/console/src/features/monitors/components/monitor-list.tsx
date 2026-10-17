import { DataTableLink } from "@/components/shared/data-table-link";
import { ListPagination } from "@/components/shared/list-pagination";
import { DataTable, type DataTableFilterField, FilterBar, StatusBadge } from "@/design-system";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import {
  type CoreWorkerDiagnostics,
  CoreWorkerWarning,
  shouldWarnForCoreWorker,
} from "@/features/monitors/components/core-worker-diagnostics";
import {
  type ApiAgentResponse,
  type ApiMonitorResponse,
  type GetMonitorsParams,
  useGetAgents,
  useGetMonitorSummary,
  useGetMonitors,
} from "@/orion-sdk";
import { type ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
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

const isCoreOwnedMonitor = (monitor: ApiMonitorResponse) =>
  monitor.owner_kind === "core" || monitor.source === "core";

const serverFilterValue = (agentID: string) => `server:${agentID}`;

const serverLabel = (agent: ApiAgentResponse) => agent.name ?? agent.id ?? "Unknown server";

const monitorColumns = (
  workerDiagnostics?: CoreWorkerDiagnostics,
): ColumnDef<ApiMonitorResponse>[] => [
  {
    accessorKey: "name",
    header: "Monitor",
    cell: ({ row }) => {
      const monitor = row.original;
      return (
        <div className="min-w-56 space-y-1">
          <DataTableLink to={`/monitors/${monitor.id}`} truncate>
            {monitor.name ?? monitor.id ?? "Unknown monitor"}
          </DataTableLink>
          {isCoreOwnedMonitor(monitor) && shouldWarnForCoreWorker(workerDiagnostics) && (
            <div className="text-xs font-medium text-amber-800">Worker attention needed</div>
          )}
        </div>
      );
    },
  },
  {
    id: "health",
    header: "Health",
    cell: ({ row }) => <StatusBadge status={monitorHealth(row.original)} />,
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "owner_name",
    header: "Target",
    cell: ({ row }) => {
      const monitor = row.original;
      const kind = ownerLabel(monitor);
      const labelClass =
        kind === "Core"
          ? "border-sky-200 bg-sky-50 text-sky-700"
          : "border-emerald-200 bg-emerald-50 text-emerald-700";
      const owner = (
        <span className={`rounded border px-2 py-0.5 text-xs font-medium ${labelClass}`}>
          {kind}
        </span>
      );

      if (isCoreOwnedMonitor(monitor) || !monitor.agent_id) {
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

type MonitorListProps = {
  workerDiagnostics?: CoreWorkerDiagnostics;
};

export const MonitorList = ({ workerDiagnostics }: MonitorListProps) => {
  const [{ search, status, type, target, incidents, page }, setMonitorQuery] = useQueryStates({
    search: parseAsString.withDefault(""),
    status: parseAsStringLiteral(monitorStatusFilters).withDefault("all"),
    type: parseAsStringLiteral(monitorTypeFilters).withDefault("all"),
    target: parseAsString.withDefault("all"),
    incidents: parseAsBoolean.withDefault(false),
    page: parseAsInteger.withDefault(1),
  });
  const currentPage = Math.max(page, 1);
  const offset = (currentPage - 1) * MONITOR_LIMIT;
  const selectedServerID = target.startsWith("server:") ? target.slice("server:".length) : "";

  const params: GetMonitorsParams = {
    limit: MONITOR_LIMIT,
    offset,
    search: search.trim() || undefined,
    health: status === "all" ? undefined : status,
    type: type === "all" ? undefined : type,
    owner_kind: target === "core" ? "core" : selectedServerID ? "agent" : undefined,
    owner_name: selectedServerID || undefined,
    has_incidents: incidents || undefined,
    sort: "updated_at",
    order: "desc",
  };

  const agentsResponse = useGetAgents({ limit: 200 });
  const monitorsResponse = useGetMonitors(params);
  const summaryResponse = useGetMonitorSummary();
  const serverOptions = (agentsResponse.data?.agents ?? [])
    .filter((agent): agent is ApiAgentResponse & { id: string } => Boolean(agent.id))
    .map((agent) => ({ value: serverFilterValue(agent.id), label: serverLabel(agent) }))
    .sort((first, second) => first.label.localeCompare(second.label));
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const hasCoreMonitors = monitors.some(isCoreOwnedMonitor);
  const selectedSummaryFilter: MonitorSummaryFilter = incidents ? "incidents" : status;
  const hasFilters =
    Boolean(search.trim()) || status !== "all" || type !== "all" || target !== "all" || incidents;
  const targetOptions = useMemo(
    () => [
      { value: "all", label: "All targets" },
      { value: "core", label: "Core" },
      ...serverOptions,
    ],
    // serverOptions is rebuilt each render from agent data; key on its content.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [JSON.stringify(serverOptions)],
  );
  const fields = useMemo<readonly DataTableFilterField[]>(
    () => [
      { type: "search", key: "search", placeholder: "Search monitors" },
      { type: "select", key: "status", label: "Status", options: monitorStatusOptions },
      { type: "select", key: "type", label: "Type", options: monitorTypeOptions },
      { type: "select", key: "target", label: "Target", options: targetOptions },
    ],
    [targetOptions],
  );
  const filterHandlers: Record<string, (value: string) => void> = {
    search: (value) => setSearch(value),
    status: (value) => setStatus(value),
    type: (value) => setType(value),
    target: (value) => setTarget(value),
  };

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

  const setTarget = (nextTarget: string) => {
    void setMonitorQuery({ target: nextTarget, page: 1 });
  };

  const clearFilters = () => {
    void setMonitorQuery({
      search: "",
      status: "all",
      type: "all",
      target: "all",
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
      {hasCoreMonitors && shouldWarnForCoreWorker(workerDiagnostics) && (
        <CoreWorkerWarning worker={workerDiagnostics} className="mt-4" />
      )}
      <div className="mt-6">
        <FilterBar
          fields={fields}
          values={{ search, status, type, target }}
          onChange={(key, value) => filterHandlers[key]?.(value)}
          onClear={clearFilters}
        />
      </div>
      <div className="mt-2">
        <DataTable
          columns={monitorColumns(workerDiagnostics)}
          data={monitors}
          emptyMessage={
            hasFilters ? "No monitors match the current filters." : "No monitors registered."
          }
          getRowId={(monitor) => monitor.id ?? ""}
          isLoading={monitorsResponse.isLoading}
          error={
            monitorsResponse.error
              ? {
                  message: "Unable to load monitors",
                  onRetry: () => void monitorsResponse.refetch(),
                }
              : undefined
          }
          onResetFilters={hasFilters ? clearFilters : undefined}
        />
      </div>
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
