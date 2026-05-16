import { type ApiMonitorResponse, useGetAgentMonitors } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { DataTableLink } from "@/components/data-table-link";
import { ListPagination } from "@/components/list-pagination";
import { StatusBadge, toStatus } from "@/components/status-badges";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useState } from "react";

const MONITOR_LIMIT = 10;

const MonitorRow = ({ monitor }: { monitor: ApiMonitorResponse }) => {
  const health = monitor.health ?? monitor.computed_health ?? "unknown";

  return (
    <TableRow>
      <TableCell className="font-medium">
        <DataTableLink to={`/monitors/${monitor.id}`} truncate>
          {monitor.name ?? monitor.id}
        </DataTableLink>
      </TableCell>
      <TableCell>
        <StatusBadge value={toStatus(health)} />
      </TableCell>
      <TableCell>{monitor.type ?? "unknown"}</TableCell>
      <TableCell>{formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}</TableCell>
    </TableRow>
  );
};

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
    <div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Monitor</TableHead>
            <TableHead>Health</TableHead>
            <TableHead>Type</TableHead>
            <TableHead>Last success</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {monitors.map((monitor: ApiMonitorResponse) => (
            <MonitorRow key={monitor.id} monitor={monitor} />
          ))}
        </TableBody>
      </Table>
      <ListPagination
        count={count}
        limit={MONITOR_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
