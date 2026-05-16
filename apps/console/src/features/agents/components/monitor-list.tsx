import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { DataTable } from "@/components/data-table";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { StatusBadge, toStatus } from "@/components/status-badges";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useState } from "react";

const MONITOR_LIMIT = 10;

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
      return <StatusBadge value={toStatus(health)} />;
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
  const offset = (Math.max(page, 1) - 1) * MONITOR_LIMIT;
  const monitorsResponse = useGetAgentMonitors(agentId, { limit: MONITOR_LIMIT, offset });
  const monitors = monitorsResponse.data?.monitors ?? [];
  const count = monitorsResponse.data?.count ?? monitors.length;
  const setOffset = (nextOffset: number) => {
    setPage(Math.floor(nextOffset / MONITOR_LIMIT) + 1);
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
        limit={MONITOR_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
        className="rounded-b-lg! bg-neutral-100/80 border-t border-neutral-300/60 py-1"
      />
    </div>
  );
};
