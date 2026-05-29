import { DataTable } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { type ApiServiceLogEntryResponse, useGetAgentServiceLogs } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { Search } from "lucide-react";
import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";

const AGENT_SERVICE_LOG_LIMIT = 20;

const levelOptions = [
  { value: "all", label: "All levels" },
  { value: "DEBUG", label: "Debug" },
  { value: "INFO", label: "Info" },
  { value: "WARN", label: "Warn" },
  { value: "ERROR", label: "Error" },
] as const;

const serviceLogColumns: ColumnDef<ApiServiceLogEntryResponse>[] = [
  {
    accessorKey: "occurred_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.occurred_at, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "level",
    header: "Level",
    cell: ({ row }) => row.original.level ?? "INFO",
  },
  {
    accessorKey: "component",
    header: "Component",
    cell: ({ row }) => row.original.component || "server",
  },
  {
    accessorKey: "message",
    header: "Message",
    cell: ({ row }) => (
      <div className="max-w-[30rem] truncate text-neutral-600">{row.original.message ?? "—"}</div>
    ),
  },
  {
    accessorKey: "monitor_name",
    header: "Monitor",
    cell: ({ row }) => row.original.monitor_name || "—",
  },
];

type AgentServiceLogsTabProps = {
  agentId: string;
};

export const AgentServiceLogsTab = ({ agentId }: AgentServiceLogsTabProps) => {
  const [{ serviceLogsLevel, serviceLogsPage, serviceLogsQ }, setServiceLogQuery] = useQueryStates({
    serviceLogsLevel: parseAsString.withDefault("all"),
    serviceLogsPage: parseAsInteger.withDefault(1),
    serviceLogsQ: parseAsString.withDefault(""),
  });
  const offset = (Math.max(serviceLogsPage, 1) - 1) * AGENT_SERVICE_LOG_LIMIT;
  const logsQuery = useGetAgentServiceLogs(agentId, {
    limit: AGENT_SERVICE_LOG_LIMIT,
    offset,
    level: serviceLogsLevel === "all" ? undefined : serviceLogsLevel,
    q: serviceLogsQ.trim() || undefined,
  });
  const logs = logsQuery.data?.logs ?? [];
  const count = logsQuery.data?.count ?? logs.length;
  const levelLabel =
    levelOptions.find((option) => option.value === serviceLogsLevel)?.label ?? serviceLogsLevel;
  const hasFilters = Boolean(serviceLogsQ.trim()) || serviceLogsLevel !== "all";

  const setOffset = (nextOffset: number) => {
    void setServiceLogQuery({
      serviceLogsPage: Math.floor(nextOffset / AGENT_SERVICE_LOG_LIMIT) + 1,
    });
  };

  const setSearch = (nextSearch: string) => {
    void setServiceLogQuery({ serviceLogsQ: nextSearch, serviceLogsPage: 1 });
  };

  const setLevel = (nextLevel: string) => {
    void setServiceLogQuery({ serviceLogsLevel: nextLevel, serviceLogsPage: 1 });
  };

  const clearFilters = () => {
    void setServiceLogQuery({ serviceLogsQ: "", serviceLogsLevel: "all", serviceLogsPage: 1 });
  };

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative w-full max-w-sm">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-400" />
          <Input
            value={serviceLogsQ}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search service logs"
            className="pl-9"
          />
        </div>
        <Select value={serviceLogsLevel} onValueChange={setLevel}>
          <SelectTrigger className="min-w-44 text-xs">
            <span data-slot="select-value">Level: {levelLabel}</span>
          </SelectTrigger>
          <SelectContent>
            {levelOptions.map((option) => (
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
      {logsQuery.error && <div className="text-sm">Unable to load service logs.</div>}
      {!logsQuery.error && (
        <DataTable
          columns={serviceLogColumns}
          data={logs}
          emptyMessage={
            hasFilters ? "No service logs match the current filters." : "No service logs recorded."
          }
          getRowId={(log, index) => log.id ?? `${log.agent_id ?? "server"}-${index}`}
          isLoading={logsQuery.isLoading}
          loadingMessage="Loading service logs..."
        />
      )}
      {count > 0 && (
        <ListPagination
          count={count}
          limit={AGENT_SERVICE_LOG_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
