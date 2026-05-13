import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { ApiMonitorResponse } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { Link } from "react-router-dom";
import { monitorHealth } from "./agent-detail-utils";

type AgentMonitorsTabProps = {
  monitors: ApiMonitorResponse[];
  isLoading: boolean;
  hasError: boolean;
};

export const AgentMonitorsTab = ({ monitors, isLoading, hasError }: AgentMonitorsTabProps) => {
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
            {monitors.map((monitor) => (
              <TableRow key={monitor.id}>
                <TableCell className="font-medium">
                  <Link to={`/monitors/${monitor.id}`} className="hover:text-neutral-600">
                    {monitor.name ?? monitor.id}
                  </Link>
                </TableCell>
                <TableCell>{monitorHealth(monitor)}</TableCell>
                <TableCell>{monitor.type ?? "unknown"}</TableCell>
                <TableCell>
                  {formatDate(monitor.last_successful_report_at, DATE_TIME_FORMAT)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
};
