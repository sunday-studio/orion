import { useQueries } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { Link } from "react-router-dom";
import {
  DataTable,
  type DataTableFilterField,
  EmptyState,
  FilterBar,
  StatusBadge,
  useDataTableUrlState,
} from "@/design-system";
import { levelLabel, levelTone } from "@/features/logs/log-model";
import { type ApiLogEntry, getSearchLogsQueryOptions } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import {
  LEVEL_FILTERS,
  LOG_RANGES,
  logRangeFromParam,
  mergeLogs,
  openInLogsHref,
  severityParam,
} from "../logs-model";

const FIELDS: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search these logs" },
  { type: "select", key: "level", label: "Level", options: LEVEL_FILTERS },
  { type: "select", key: "range", label: "Time", options: LOG_RANGES, allValue: "2h" },
];

const columns: ColumnDef<ApiLogEntry>[] = [
  {
    id: "time",
    header: "Time",
    cell: ({ row }) => (
      <span className="font-mono text-xs text-quiet">
        {formatDate(row.original.timestamp, TIME_FORMAT + ":ss")}
      </span>
    ),
  },
  {
    id: "level",
    header: "Level",
    cell: ({ row }) => (
      <StatusBadge status={levelTone(row.original.severity)}>
        {levelLabel(row.original.severity)}
      </StatusBadge>
    ),
  },
  { id: "app", header: "App", cell: ({ row }) => row.original.app_name ?? "-" },
  {
    id: "message",
    header: "Message",
    cell: ({ row }) => (
      <span className="block max-w-[56ch] truncate">{row.original.event ?? row.original.body}</span>
    ),
  },
];

/** Logs from every app linked to the service, newest first. */
export const ServiceLogsTab = ({ appIds }: { appIds: readonly string[] }) => {
  const url = useDataTableUrlState({ fields: FIELDS, limit: 25, prefix: "log" });
  const range = logRangeFromParam(url.values.range);
  const q = url.values.q?.trim() || undefined;
  const results = useQueries({
    queries: appIds.map((id) =>
      getSearchLogsQueryOptions({
        app_id: id,
        from: `-${range}`,
        severity: severityParam(url.values.level ?? "all"),
        q,
        limit: 200,
      }),
    ),
  });
  const pending = results.some((result) => result.isPending);
  const failed = results.find((result) => result.isError);
  const entries = mergeLogs(results.map((result) => result.data?.entries ?? []));

  if (appIds.length === 0) {
    return (
      <EmptyState
        title="No app is linked to this service"
        description="Link an app to read its logs here and on incidents for this service's monitors."
      />
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterBar
          fields={FIELDS}
          values={url.values}
          onChange={url.setFilter}
          onClear={url.clearFilters}
        />
        <Link to={openInLogsHref(appIds, range)} className="text-[12.5px] underline">
          Open in Logs
        </Link>
      </div>
      <DataTable
        columns={columns}
        data={entries}
        isLoading={pending}
        error={
          failed
            ? {
                message: "Unable to load logs",
                onRetry: () => results.forEach((r) => void r.refetch()),
              }
            : undefined
        }
        getRowId={(entry, index) => entry.id ?? `log-${index}`}
        emptyMessage="No logs in this range."
        onResetFilters={url.hasFilters ? url.clearFilters : undefined}
        defaultPageSize={url.limit}
      />
    </div>
  );
};
