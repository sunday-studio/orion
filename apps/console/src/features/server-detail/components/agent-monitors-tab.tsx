import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ApiIncidentResponse, ApiMonitorResponse } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { cn } from "@/lib/utils";
import { Link } from "react-router-dom";
import { monitorHealth } from "./agent-detail-utils";

type AgentMonitorsTabProps = {
  monitors: ApiMonitorResponse[];
  isLoading: boolean;
  hasError: boolean;
  highlightedIncident?: ApiIncidentResponse;
};

export const AgentMonitorsTab = ({
  monitors,
  isLoading,
  hasError,
  highlightedIncident,
}: AgentMonitorsTabProps) => {
  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Monitors</h2>
        <p className="text-sm text-neutral-600">Checks registered by this agent.</p>
      </div>
      {isLoading && <div className="text-sm text-neutral-600">Loading monitors...</div>}
      {hasError && <div className="text-sm">Unable to load monitors.</div>}
      {!isLoading && !hasError && monitors.length === 0 && (
        <div className="text-sm text-neutral-600">No monitors registered.</div>
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
                    <Link to={monitorPath} className="hover:text-neutral-600">
                      {monitor.name ?? monitor.id}
                    </Link>
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
    </div>
  );
};
