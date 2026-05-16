import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import { DataTableLink } from "@/components/data-table-link";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { type ApiIncidentResponse, useGetAgentMonitors } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import { useState } from "react";
import { monitorHealth, monitorPriority } from "./agent-detail-utils";

const AGENT_MONITOR_LIMIT = 20;

type AgentMonitorsTabProps = {
  agentId: string;
  highlightedIncident?: ApiIncidentResponse;
};

export const AgentMonitorsTab = ({ agentId, highlightedIncident }: AgentMonitorsTabProps) => {
  const [page, setPage] = useState(1);
  const offset = (Math.max(page, 1) - 1) * AGENT_MONITOR_LIMIT;
  const monitorsResponse = useGetAgentMonitors(agentId, { limit: AGENT_MONITOR_LIMIT, offset });
  const monitors = [...(monitorsResponse.data?.monitors ?? [])].sort(
    (left, right) => monitorPriority(left) - monitorPriority(right),
  );
  const count = monitorsResponse.data?.count ?? monitors.length;
  const setOffset = (nextOffset: number) => {
    setPage(Math.floor(nextOffset / AGENT_MONITOR_LIMIT) + 1);
  };

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Monitors</h2>
        <p className="text-sm text-neutral-600">Checks registered by this agent.</p>
      </div>
      {monitorsResponse.isLoading && (
        <div className="text-sm text-neutral-600">Loading monitors...</div>
      )}
      {monitorsResponse.error && <div className="text-sm">Unable to load monitors.</div>}
      {!monitorsResponse.isLoading && !monitorsResponse.error && monitors.length === 0 && (
        <EmptyState title="No monitors registered" />
      )}
      {monitors.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Health</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Last success</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {monitors.map((monitor) => {
              const isHighlightedMonitor = highlightedIncident?.monitor_id === monitor.id;
              const monitorPath =
                isHighlightedMonitor && highlightedIncident
                  ? `/monitors/${monitor.id}?incident=${encodeURIComponent(highlightedIncident.id ?? "")}`
                  : `/monitors/${monitor.id}`;

              return (
                <TableRow key={monitor.id} className={cn(isHighlightedMonitor && "bg-amber-50")}>
                  <TableCell className="font-medium">
                    <DataTableLink to={monitorPath}>{monitor.name ?? monitor.id}</DataTableLink>
                  </TableCell>
                  <TableCell>{monitorHealth(monitor)}</TableCell>
                  <TableCell>{monitor.type ?? "unknown"}</TableCell>
                  <TableCell>
                    {formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
                  </TableCell>
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      )}
      <ListPagination
        count={count}
        limit={AGENT_MONITOR_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
