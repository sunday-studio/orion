import { DataTable } from "@/components/data-table";
import { ListPagination } from "@/components/list-pagination";
import { type ApiAgentReportResponse, useGetAgentReports } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, useQueryStates } from "nuqs";
import { formatDuration, formatPercent } from "./agent-detail-utils";

const AGENT_LOG_LIMIT = 20;

const agentLogColumns: ColumnDef<ApiAgentReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.created_at ?? row.original.timestamp, DATE_TIME_FORMAT),
  },
  {
    accessorKey: "cpu",
    header: "CPU",
    cell: ({ row }) => formatPercent(row.original.cpu?.usage_percent),
  },
  {
    accessorKey: "memory",
    header: "Memory",
    cell: ({ row }) => formatPercent(row.original.memory?.used_percent),
  },
  {
    accessorKey: "disk",
    header: "Disk",
    cell: ({ row }) => formatPercent(row.original.disk?.used_percent),
  },
  {
    accessorKey: "uptime_seconds",
    header: "Uptime",
    cell: ({ row }) => formatDuration(row.original.uptime_seconds),
  },
];

type AgentLogsTabProps = {
  agentId: string;
};

export const AgentLogsTab = ({ agentId }: AgentLogsTabProps) => {
  const [{ reportsPage }, setReportsQuery] = useQueryStates({
    reportsPage: parseAsInteger.withDefault(1),
  });
  const offset = (Math.max(reportsPage, 1) - 1) * AGENT_LOG_LIMIT;
  const reportsQuery = useGetAgentReports(agentId, { limit: AGENT_LOG_LIMIT, offset });
  const reports = reportsQuery.data?.reports ?? [];
  const count = reportsQuery.data?.count ?? reports.length;
  const setOffset = (nextOffset: number) => {
    void setReportsQuery({ reportsPage: Math.floor(nextOffset / AGENT_LOG_LIMIT) + 1 });
  };

  return (
    <div className="space-y-3">
      {reportsQuery.error && <div className="text-sm">Unable to load reports.</div>}
      {!reportsQuery.error && (
        <DataTable
          columns={agentLogColumns}
          data={reports}
          emptyMessage="No reports recorded"
          getRowId={(report, index) => report.id ?? `${report.agent_id ?? "agent"}-${index}`}
          isLoading={reportsQuery.isLoading}
          loadingMessage="Loading reports..."
        />
      )}
      {count > 0 && (
        <ListPagination
          count={count}
          limit={AGENT_LOG_LIMIT}
          offset={offset}
          onOffsetChange={setOffset}
        />
      )}
    </div>
  );
};
