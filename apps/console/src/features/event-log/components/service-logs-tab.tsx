import type { ColumnDef } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  DataTable,
  type DataTableFilterField,
  FilterBar,
  StatusBadge,
  filterParam,
  useDataTableUrlState,
} from "@/design-system";
import { type ApiLogEntry, type SearchLogsParams, useListApps, useSearchLogs } from "@/orion-sdk";
import { DATE_TIME_FORMAT, formatDate } from "@/utils/date";
import { LogHistogram } from "./log-histogram";
import { LogEntryDrawer, logTitle } from "./log-entry-drawer";

const PAGE_SIZE = 50;

/** Default window is one hour because the histogram is slow over wide windows (D-025). */
const windowOptions = [
  { value: "-15m", label: "Last 15 minutes" },
  { value: "-1h", label: "Last hour" },
  { value: "-6h", label: "Last 6 hours" },
  { value: "-24h", label: "Last 24 hours" },
] as const;

const severityOptions = [
  { value: "all", label: "All severities" },
  { value: ">=warn", label: "Warn and above" },
  { value: "error,fatal", label: "Error and fatal" },
  { value: "info", label: "Info" },
  { value: "debug,trace", label: "Debug and trace" },
] as const;

const sourceOptions = [
  { value: "app", label: "Apps" },
  { value: "agent", label: "Servers" },
] as const;

const columns: ColumnDef<ApiLogEntry>[] = [
  {
    id: "time",
    header: "Time",
    cell: ({ row }) => formatDate(row.original.timestamp, DATE_TIME_FORMAT),
  },
  {
    id: "severity",
    header: "Severity",
    cell: ({ row }) => (
      <StatusBadge status={row.original.severity === "warn" ? "degraded" : row.original.severity}>
        {row.original.severity}
      </StatusBadge>
    ),
  },
  {
    id: "source",
    header: "Source",
    cell: ({ row }) =>
      row.original.source === "agent" ? "Server" : (row.original.app_name ?? "App"),
  },
  {
    id: "message",
    header: "Message",
    cell: ({ row }) => <div className="max-w-[32rem] truncate">{logTitle(row.original)}</div>,
  },
];

/** Module-level so the URL-state hook gets a stable schema; option lists are filled in below. */
const urlFields: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search logs" },
  { type: "select", key: "source", label: "Source", options: sourceOptions, allValue: "app" },
  { type: "select", key: "app", label: "App", options: [{ value: "all", label: "All apps" }] },
  { type: "select", key: "severity", label: "Severity", options: severityOptions },
  { type: "select", key: "window", label: "Time", options: windowOptions, allValue: "-1h" },
];

export const ServiceLogsTab = () => {
  const appsQuery = useListApps();
  const apps = useMemo(() => appsQuery.data?.apps ?? [], [appsQuery.data]);
  const table = useDataTableUrlState({ fields: urlFields, limit: PAGE_SIZE, prefix: "app" });
  const [selected, setSelected] = useState<ApiLogEntry>();

  const fields = useMemo<readonly DataTableFilterField[]>(
    () =>
      urlFields.map((field) =>
        field.key === "app" && field.type === "select"
          ? {
              ...field,
              options: [
                { value: "all", label: "All apps" },
                ...apps.map((app) => ({ value: app.id ?? "", label: app.name ?? "App" })),
              ],
            }
          : field,
      ),
    [apps],
  );
  const source = filterParam(urlFields[1], table.values.source) ?? "app";
  const filters = {
    source,
    q: filterParam(urlFields[0], table.values.q),
    app_id: source === "app" ? filterParam(urlFields[2], table.values.app) : undefined,
    severity: filterParam(urlFields[3], table.values.severity),
    from: table.values.window || "-1h",
  };
  // Cursors belong to one filter set; any change to the filters (including Clear and history
  // navigation) starts again from the first page.
  const filterKey = JSON.stringify(filters);
  const [paging, setPaging] = useState<{ key: string; cursors: string[] }>({
    key: filterKey,
    cursors: [],
  });
  const cursors = paging.key === filterKey ? paging.cursors : [];
  const setCursors = (next: string[]) => setPaging({ key: filterKey, cursors: next });
  const query = useSearchLogs({
    ...filters,
    limit: PAGE_SIZE,
    cursor: cursors[cursors.length - 1],
  } satisfies SearchLogsParams);
  const entries = query.data?.entries ?? [];
  const nextCursor = query.data?.next_cursor;
  const hasApps = apps.length > 0 || !appsQuery.isSuccess;

  const setFilter = table.setFilter;

  if (
    !hasApps &&
    source === "app" &&
    !table.hasFilters &&
    entries.length === 0 &&
    !query.isLoading
  ) {
    return (
      <div className="space-y-3">
        <FilterBar
          fields={fields}
          values={table.values}
          onChange={setFilter}
          onClear={table.clearFilters}
        />
        <div className="flex min-h-40 flex-col items-center justify-center gap-3 border border-dashed border-hairline text-center">
          <p className="text-sm font-medium">No apps connected yet</p>
          <p className="text-xs text-quiet">Connect an app to see its product logs here.</p>
          <Link className="text-[13px] text-brand hover:underline" to="/settings/apps">
            Connect your first app
          </Link>
        </div>
      </div>
    );
  }

  return (
    <section className="space-y-4">
      <FilterBar
        fields={fields}
        values={table.values}
        onChange={setFilter}
        onClear={table.clearFilters}
      />
      <LogHistogram params={filters} />
      <DataTable
        columns={columns}
        data={entries}
        getRowId={(entry, index) => entry.id ?? `log-${index}`}
        isLoading={query.isLoading}
        error={
          query.isError
            ? { message: "Unable to load logs", onRetry: () => void query.refetch() }
            : undefined
        }
        emptyMessage={
          table.hasFilters ? "No logs match the current filters." : "No logs in this time window."
        }
        onResetFilters={table.hasFilters ? table.clearFilters : undefined}
        onRowClick={setSelected}
      />
      {query.data?.truncated && (
        <p className="text-xs text-quiet">
          The window matched more rows than Core will scan. Narrow the time range or add filters.
        </p>
      )}
      <div className="flex justify-end gap-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={cursors.length === 0}
          onClick={() => setCursors(cursors.slice(0, -1))}
        >
          Newer
        </Button>
        <Button
          size="sm"
          variant="secondary"
          disabled={!nextCursor}
          onClick={() => nextCursor && setCursors([...cursors, nextCursor])}
        >
          Older
        </Button>
      </div>
      <LogEntryDrawer
        entry={selected}
        onOpenChange={(open) => {
          if (!open) setSelected(undefined);
        }}
      />
    </section>
  );
};
