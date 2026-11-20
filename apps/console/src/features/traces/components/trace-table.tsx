import type { ColumnDef } from "@tanstack/react-table";
import { useMemo } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Badge, DataTable } from "@/design-system";
import { focusRing } from "@/design-system/styles";
import { LogPager } from "@/features/logs/components/log-table";
import type { ApiTraceListItem } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { formatDate } from "@/utils/date";
import { formatDurationNs } from "../trace-format";
import { traceHref } from "../trace-links";
import { durationShare, slowestDuration } from "../traces-state";

export const traceRowId = (row: ApiTraceListItem) => `${row.app_id ?? ""}:${row.trace_id ?? ""}`;

const DurationCell = ({ row, slowest }: { row: ApiTraceListItem; slowest: number }) => {
  const share = durationShare(row.duration_ns, slowest);
  const failed = row.status === "error";
  return (
    <span className="flex items-center gap-2.5">
      <span aria-hidden className="h-1.5 w-20 shrink-0 overflow-hidden rounded-full bg-tray">
        <span
          className="block h-full rounded-full"
          style={{
            width: `${share}%`,
            background: failed ? "var(--color-dot-down)" : "var(--color-brand-chart)",
          }}
        />
      </span>
      <span className="tabular-nums">{formatDurationNs(row.duration_ns)}</span>
    </span>
  );
};

const columnsFor = (slowest: number): ColumnDef<ApiTraceListItem>[] => [
  {
    id: "name",
    header: "Root span",
    cell: ({ row }) => {
      const item = row.original;
      return (
        <span className="flex min-w-0 items-center gap-2">
          <Link
            to={traceHref(item.trace_id ?? "")}
            // The row opens the trace too: this link is for middle click and screen readers.
            onClick={(event) => event.stopPropagation()}
            className={cn(
              focusRing,
              "min-w-0 max-w-[40ch] truncate rounded-sm font-medium hover:underline",
            )}
          >
            {item.root_name || "(unnamed)"}
          </Link>
          {item.has_root === false && (
            <span className="shrink-0 text-xs text-faint" title="The root span has not arrived">
              partial
            </span>
          )}
        </span>
      );
    },
  },
  {
    id: "service",
    header: "Service",
    cell: ({ row }) => (
      <span className="block max-w-[24ch] truncate text-mid">
        {row.original.service_name || row.original.app_name || "-"}
      </span>
    ),
  },
  {
    id: "started",
    header: "Started",
    cell: ({ row }) => (
      <span className="text-quiet">{formatDate(row.original.start_time, "MMM d, HH:mm:ss")}</span>
    ),
  },
  {
    id: "duration",
    header: "Duration",
    cell: ({ row }) => <DurationCell row={row.original} slowest={slowest} />,
  },
  {
    id: "spans",
    header: "Spans",
    cell: ({ row }) => (row.original.span_count ?? 0).toLocaleString(),
  },
  {
    id: "errors",
    header: "Errors",
    cell: ({ row }) => {
      const count = row.original.error_count ?? 0;
      return count > 0 ? (
        <span className="font-medium text-(--tone-down-fg)">{count.toLocaleString()}</span>
      ) : (
        <span className="text-faint">0</span>
      );
    },
  },
  {
    id: "status",
    header: "Status",
    cell: ({ row }) =>
      row.original.status === "error" ? (
        <Badge tone="down">Error</Badge>
      ) : (
        <Badge tone="up">OK</Badge>
      ),
  },
];

type Paging = {
  from: number;
  to: number;
  total?: number;
  hasNewer: boolean;
  hasOlder: boolean;
  limit: string;
  onNewer: () => void;
  onOlder: () => void;
  onLimit: (limit: string) => void;
};

/** One row per trace. Rows are focusable and open the trace on Enter, as on the other tables. */
export const TraceTable = ({
  rows,
  fetching,
  paging,
}: {
  rows: ApiTraceListItem[];
  fetching: boolean;
  paging: Paging;
}) => {
  const navigate = useNavigate();
  const slowest = useMemo(() => slowestDuration(rows), [rows]);
  const columns = useMemo(() => columnsFor(slowest), [slowest]);
  return (
    <div
      className={cn(
        "ds-settled min-w-0 overflow-hidden rounded-xl border border-hairline bg-card shadow-card",
        fetching && "ds-fetching",
      )}
    >
      <div className="overflow-x-auto">
        <div className="min-w-[760px]">
          <DataTable
            columns={columns}
            data={rows}
            getRowId={traceRowId}
            paginate={false}
            onRowClick={(row) => navigate(traceHref(row.trace_id ?? ""))}
          />
        </div>
      </div>
      <LogPager {...paging} />
    </div>
  );
};
