import { DataTable, usePageSize } from "@/design-system";
import { ReportInspectionDrawer } from "@/features/report-inspection/components/report-inspection-drawer";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { type ApiAgentReportResponse, useGetAgentReports } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, useQueryStates } from "nuqs";
import { useState } from "react";
import { formatDuration, formatPercent } from "../agent-detail.domain";

const agentLogColumns: ColumnDef<ApiAgentReportResponse>[] = [
  {
    accessorKey: "created_at",
    header: "Time",
    cell: ({ row }) =>
      formatDate(row.original.created_at ?? row.original.timestamp, DATE_TIME_FORMAT),
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
  const { limit: pageSize, setLimit: setPageSize } = usePageSize(20, {
    key: "reportsSize",
    pageKey: "reportsPage",
  });
  const offset = (Math.max(reportsPage, 1) - 1) * pageSize;
  const reportsQuery = useGetAgentReports(agentId, { limit: pageSize, offset });
  const reports = reportsQuery.data?.reports ?? [];
  const count = reportsQuery.data?.count ?? reports.length;
  const [selectedReport, setSelectedReport] = useState<ApiAgentReportResponse>();
  const setOffset = (nextOffset: number) => {
    void setReportsQuery({ reportsPage: Math.floor(nextOffset / pageSize) + 1 });
  };

  return (
    <div className="space-y-3">
      {reportsQuery.error && <div className="text-sm">Unable to load reports.</div>}
      {!reportsQuery.error && (
        <DataTable
          pagination={{
            onLimitChange: setPageSize,
            count: count,
            limit: pageSize,
            offset: offset,
            onOffsetChange: setOffset,
          }}
          columns={agentLogColumns}
          data={reports}
          emptyMessage="No reports recorded"
          getRowId={(report, index) => report.id ?? `${report.agent_id ?? "server"}-${index}`}
          isLoading={reportsQuery.isLoading}
          onRowClick={setSelectedReport}
        />
      )}

      <ReportInspectionDrawer
        kind="agent"
        report={selectedReport}
        onOpenChange={(open) => {
          if (!open) setSelectedReport(undefined);
        }}
      />
    </div>
  );
};
