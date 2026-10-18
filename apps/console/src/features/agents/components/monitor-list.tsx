import { DataTable, DataTableLink, ListPagination, StatusBadge } from "@/design-system";
import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { useState } from "react";

const MONITOR_LIMIT = 20;

const monitorColumns: ColumnDef<ApiMonitorResponse>[] = [
  {
    accessorKey: "name",
    header: "Monitor",
    cell: ({ row }) => (
      <DataTableLink to={`/monitors/${row.original.id}`} truncate>
        {row.original.name ?? row.original.id}
      </DataTableLink>
    ),
  },
  {
    id: "health",
    header: "Health",
    cell: ({ row }) => {
      const health = row.original.health ?? row.original.computed_health ?? "unknown";
      return <StatusBadge status={health} />;
    },
  },
  {
    accessorKey: "type",
    header: "Type",
    cell: ({ row }) => row.original.type ?? "unknown",
  },
  {
    accessorKey: "last_successful_report_at",
    header: "Last success",
    cell: ({ row }) => formatDate(row.original.last_successful_report_at, DATE_TIME_FORMAT),
  },
];

export const MonitorList = ({ agentId }: { agentId: string }) => {
  const [page, setPage] = useState(1);
  const [pageSize, setPageSizeState] = useState(MONITOR_LIMIT);
  const offset = (Math.max(page, 1) - 1) * pageSize;
  const monitorsResponse = useGetAgentMonitors(agentId, { limit: pageSize, offset });
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const setOffset = (nextOffset: number) => {
    setPage(Math.floor(nextOffset / pageSize) + 1);
  };
  const setPageSize = (next: number) => {
    setPageSizeState(next);
    setPage(1);
  };

  if (monitorsResponse.isLoading) {
    return <div className="px-3 py-3 text-sm text-neutral-600">Loading monitors...</div>;
  }

  if (monitorsResponse.error) {
    return <div className="px-3 py-3 text-sm">Unable to load monitors.</div>;
  }

  if (monitors.length === 0) {
    return <div className="px-3 py-3 text-sm text-neutral-600">No monitors registered.</div>;
  }

  return (
    <div className="overflow-hidden">
      <DataTable
        columns={monitorColumns}
        data={monitors}
        emptyMessage="No monitors registered."
        getRowId={(monitor) => monitor.id ?? ""}
      />
      <ListPagination
        count={count}
        limit={pageSize}
        offset={offset}
        onLimitChange={setPageSize}
        onOffsetChange={setOffset}
        className=" bg-neutral-100/80 border-t border-neutral-300/60 py-1"
      />
    </div>
  );
};
