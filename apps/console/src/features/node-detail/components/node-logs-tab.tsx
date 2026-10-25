import {
  AddFilterButton,
  DataTable,
  FilterToken,
  FilterTokenRow,
  PopoverClose,
  PopoverContent,
  usePageSize,
} from "@/design-system";
import { ReportInspectionDrawer } from "@/features/report-inspection/components/report-inspection-drawer";
import { formatDate } from "@/utils/date";
import { type ApiNodeReportResponse, useGetNodeReports } from "@/orion-sdk";
import type { ColumnDef } from "@tanstack/react-table";
import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { useState } from "react";

const whole = (value?: number) => (typeof value === "number" ? `${Math.round(value)}%` : "-");

const nodeLogColumns: ColumnDef<ApiNodeReportResponse>[] = [
  {
    id: "time",
    header: "Time",
    cell: ({ row }) => (
      <span className="font-mono text-faint">
        {formatDate(row.original.created_at ?? row.original.timestamp, "HH:mm:ss")}
      </span>
    ),
  },
  { id: "cpu", header: "CPU", cell: ({ row }) => whole(row.original.cpu?.usage_percent) },
  { id: "memory", header: "Memory", cell: ({ row }) => whole(row.original.memory?.used_percent) },
  { id: "disk", header: "Disk", cell: ({ row }) => whole(row.original.disk?.used_percent) },
  {
    id: "monitors",
    header: "Monitors",
    cell: ({ row }) => (
      <span className="text-faint">{row.original.config_summary?.monitor_count ?? "-"}</span>
    ),
  },
];

const SINCE_OPTIONS = [
  { value: "1h", label: "last hour" },
  { value: "24h", label: "last 24 hours" },
  { value: "7d", label: "last 7 days" },
] as const;

type NodeLogsTabProps = {
  nodeId: string;
};

export const NodeLogsTab = ({ nodeId }: NodeLogsTabProps) => {
  const [{ reportsPage, since: sinceParam }, setReportsQuery] = useQueryStates({
    reportsPage: parseAsInteger.withDefault(1),
    since: parseAsString.withDefault("1h"),
  });
  const since =
    sinceParam === "all" || SINCE_OPTIONS.some((option) => option.value === sinceParam)
      ? sinceParam
      : "1h";
  const { limit: pageSize, setLimit: setPageSize } = usePageSize(20, {
    key: "reportsSize",
    pageKey: "reportsPage",
  });
  const offset = (Math.max(reportsPage, 1) - 1) * pageSize;
  const reportsQuery = useGetNodeReports(nodeId, {
    limit: pageSize,
    offset,
    since: since === "all" ? undefined : since,
  });
  const reports = reportsQuery.data?.reports ?? [];
  const count = reportsQuery.data?.count ?? reports.length;
  const [selectedReport, setSelectedReport] = useState<ApiNodeReportResponse>();
  const setOffset = (nextOffset: number) => {
    void setReportsQuery({ reportsPage: Math.floor(nextOffset / pageSize) + 1 });
  };

  return (
    <div className="space-y-3">
      <FilterTokenRow onClearAll={() => void setReportsQuery({ since: "1h", reportsPage: 1 })}>
        {since !== "all" && (
          <FilterToken
            field="Time"
            operator="in"
            value={SINCE_OPTIONS.find((option) => option.value === since)?.label ?? since}
            onRemove={() => void setReportsQuery({ since: "all", reportsPage: 1 })}
          >
            <PopoverContent className="w-[200px] p-1.5">
              {SINCE_OPTIONS.map((option) => (
                <PopoverClose
                  key={option.value}
                  onClick={() => void setReportsQuery({ since: option.value, reportsPage: 1 })}
                  className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
                >
                  {option.label}
                </PopoverClose>
              ))}
            </PopoverContent>
          </FilterToken>
        )}
        {since === "all" && (
          <AddFilterButton>
            <PopoverContent className="w-[200px] p-1.5">
              <PopoverClose
                onClick={() => void setReportsQuery({ since: "1h", reportsPage: 1 })}
                className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
              >
                Time
              </PopoverClose>
            </PopoverContent>
          </AddFilterButton>
        )}
      </FilterTokenRow>
      {reportsQuery.error && <div className="text-sm">Unable to load reports.</div>}
      {!reportsQuery.error && (
        <DataTable
          pagination={{
            onLimitChange: setPageSize,
            count: count,
            limit: pageSize,
            offset: offset,
            onOffsetChange: setOffset,
          }}
          columns={nodeLogColumns}
          data={reports}
          emptyMessage="No reports recorded"
          getRowId={(report, index) => report.id ?? `${report.node_id ?? "server"}-${index}`}
          isLoading={reportsQuery.isLoading}
          onRowClick={setSelectedReport}
        />
      )}

      <ReportInspectionDrawer
        kind="node"
        report={selectedReport}
        onOpenChange={(open) => {
          if (!open) setSelectedReport(undefined);
        }}
      />
    </div>
  );
};
