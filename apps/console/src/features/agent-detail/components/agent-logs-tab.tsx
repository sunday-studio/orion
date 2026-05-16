import { EmptyState } from "@/components/empty-state";
import { ListPagination } from "@/components/list-pagination";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useGetAgentReports } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useState } from "react";
import { formatDuration, formatPercent } from "./agent-detail-utils";

const AGENT_LOG_LIMIT = 20;

type AgentLogsTabProps = {
  agentId: string;
};

export const AgentLogsTab = ({ agentId }: AgentLogsTabProps) => {
  const [page, setPage] = useState(1);
  const offset = (Math.max(page, 1) - 1) * AGENT_LOG_LIMIT;
  const reportsQuery = useGetAgentReports(agentId, { limit: AGENT_LOG_LIMIT, offset });
  const reports = reportsQuery.data?.reports ?? [];
  const count = reportsQuery.data?.count ?? reports.length;
  const setOffset = (nextOffset: number) => {
    setPage(Math.floor(nextOffset / AGENT_LOG_LIMIT) + 1);
  };

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Agent Reports</h2>
        <p className="text-sm text-neutral-600">Reports received from this agent.</p>
      </div>
      {reportsQuery.isLoading && <div className="text-sm text-neutral-600">Loading reports...</div>}
      {reportsQuery.error && <div className="text-sm">Unable to load reports.</div>}
      {!reportsQuery.isLoading && !reportsQuery.error && reports.length === 0 && (
        <EmptyState title="No reports recorded" />
      )}
      {reports.length > 0 && (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Time</TableHead>
              <TableHead>CPU</TableHead>
              <TableHead>Memory</TableHead>
              <TableHead>Disk</TableHead>
              <TableHead>Uptime</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {reports.map((report, index) => (
              <TableRow key={report.id ?? index}>
                <TableCell className="font-medium">
                  {formatDate(report.created_at ?? report.timestamp, DATE_TIME_FORMAT)}
                </TableCell>
                <TableCell>{formatPercent(report.cpu?.usage_percent)}</TableCell>
                <TableCell>{formatPercent(report.memory?.used_percent)}</TableCell>
                <TableCell>{formatPercent(report.disk?.used_percent)}</TableCell>
                <TableCell>{formatDuration(report.uptime_seconds)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
      <ListPagination
        count={count}
        limit={AGENT_LOG_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
