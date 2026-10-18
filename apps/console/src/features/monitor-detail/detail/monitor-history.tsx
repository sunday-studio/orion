import { useState } from "react";
import {
  Dot,
  EmptyState,
  GridRow,
  GridTable,
  ListPagination,
  Skeleton,
  usePageSize,
} from "@/design-system";
import { type ApiMonitorReportResponse, useGetMonitorHistory } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { MonitorCheckDrawer } from "./monitor-check-drawer";

const COLUMNS = "12px 150px minmax(0,1fr)";

export const MonitorHistory = ({
  monitorId,
  monitorName,
}: {
  monitorId: string;
  monitorName: string;
}) => {
  const { limit, setLimit } = usePageSize(20, { key: "historySize", pageKey: "historyPage" });
  const [page, setPage] = useState(1);
  const offset = (page - 1) * limit;
  const query = useGetMonitorHistory(monitorId, { limit, offset });
  const reports = query.data?.reports ?? [];
  const total = query.data?.count ?? reports.length;
  const [selected, setSelected] = useState<number>();
  const report: ApiMonitorReportResponse | undefined =
    selected === undefined ? undefined : reports[selected];

  if (query.isPending) return <Skeleton className="h-40 w-full rounded-xl" />;
  if (query.isError) return <EmptyState tone="error" title="Unable to load check history" />;
  if (reports.length === 0)
    return <EmptyState title="No checks yet" description="Checks appear here as they run." />;

  return (
    <>
      <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <GridTable>
          {reports.map((item, index) => {
            const passed = item.passed ?? item.health === "up";
            return (
              <GridRow
                key={item.id ?? index}
                columns={COLUMNS}
                className="cursor-pointer"
                onClick={() => setSelected(index)}
              >
                <Dot color={passed ? "#2a9f69" : "#e5484d"} />
                <b className="font-mono text-[13px] font-medium">
                  {formatDate(item.collected_at ?? item.created_at, "MMM d, HH:mm:ss")}
                </b>
                <span className="text-faint">{item.summary ?? "-"}</span>
              </GridRow>
            );
          })}
        </GridTable>
        <ListPagination
          count={total}
          limit={limit}
          offset={offset}
          onOffsetChange={(next) => setPage(Math.floor(next / limit) + 1)}
          onLimitChange={(next) => {
            setLimit(next);
            setPage(1);
          }}
        />
      </div>
      <MonitorCheckDrawer
        monitorName={monitorName}
        report={report}
        hasPrev={selected !== undefined && selected > 0}
        hasNext={selected !== undefined && selected < reports.length - 1}
        onPrev={() => setSelected((current) => (current ?? 1) - 1)}
        onNext={() => setSelected((current) => (current ?? -1) + 1)}
        onClose={() => setSelected(undefined)}
      />
    </>
  );
};
