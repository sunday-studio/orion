import { InfiniteScrollSentinel } from "@/components/infinite-scroll-sentinel";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { getAgentReports, type GetAgentReports200 } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { formatDuration, formatPercent } from "./agent-detail-utils";

const AGENT_LOG_LIMIT = 20;

type AgentLogsTabProps = {
  agentId: string;
};

const getNextOffset = (lastPage: GetAgentReports200) => {
  const offset = lastPage.offset ?? 0;
  const limit = lastPage.limit ?? AGENT_LOG_LIMIT;
  const count = lastPage.count ?? 0;
  const nextOffset = offset + limit;
  return nextOffset < count ? nextOffset : undefined;
};

export const AgentLogsTab = ({ agentId }: AgentLogsTabProps) => {
  const reportsQuery = useInfiniteQuery({
    queryKey: ["agent-reports", agentId, AGENT_LOG_LIMIT],
    queryFn: ({ pageParam, signal }) =>
      getAgentReports(agentId, { limit: AGENT_LOG_LIMIT, offset: pageParam }, { signal }),
    initialPageParam: 0,
    getNextPageParam: getNextOffset,
    enabled: agentId !== "",
  });
  const { fetchNextPage } = reportsQuery;
  const reports = reportsQuery.data?.pages.flatMap((page) => page.reports ?? []) ?? [];
  const loadMore = useCallback(() => {
    void fetchNextPage();
  }, [fetchNextPage]);

  return (
    <div className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">Agent Reports</h2>
        <p className="text-sm text-neutral-600">Reports received from this agent.</p>
      </div>
      {reportsQuery.isLoading && <div className="text-sm text-neutral-600">Loading reports...</div>}
      {reportsQuery.error && <div className="text-sm">Unable to load reports.</div>}
      {!reportsQuery.isLoading && !reportsQuery.error && reports.length === 0 && (
        <div className="text-sm text-neutral-600">No reports recorded.</div>
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
      <InfiniteScrollSentinel
        hasNextPage={Boolean(reportsQuery.hasNextPage)}
        isFetchingNextPage={reportsQuery.isFetchingNextPage}
        onLoadMore={loadMore}
      />
    </div>
  );
};
