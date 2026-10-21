import { cn } from "@/utils/cn";
import { keepPreviousData } from "@tanstack/react-query";
import { parseAsInteger, parseAsString, useQueryState } from "nuqs";
import { useEffect } from "react";
import { Link } from "react-router-dom";
import {
  AddFilterButton,
  CheckFilterPanel,
  EmptyState,
  FilterToken,
  FilterTokenRow,
  GridFooter,
  GridRow,
  GridTable,
  ListPagination,
  PopoverClose,
  PopoverContent,
  Skeleton,
  usePageSize,
} from "@/design-system";
import { useGetNodeMonitors } from "@/orion-sdk";
import { MONITOR_COLUMNS, MonitorCells } from "./monitor-rows";

const STATUS_OPTIONS = [
  { value: "down", label: "Down" },
  { value: "degraded", label: "Degraded" },
  { value: "stale", label: "Stale" },
  { value: "unknown", label: "Unknown" },
  { value: "up", label: "Up" },
] as const;

const DEFAULT = "down,degraded,stale,unknown";

const labelFor = (values: readonly string[]) =>
  values.map((value) => STATUS_OPTIONS.find((o) => o.value === value)?.label ?? value).join(", ") ||
  "Any";

export const NodeMonitorsTab = ({ nodeId }: { nodeId: string }) => {
  const [statusParam, setStatusParam] = useQueryState(
    "mstatus",
    parseAsString.withDefault(DEFAULT),
  );
  const selected = statusParam.split(",").filter(Boolean);
  const { limit, setLimit } = usePageSize(20, { key: "monitorsSize", pageKey: "monitorsPage" });
  const [page, setPage] = useQueryState("monitorsPage", parseAsInteger.withDefault(1));
  const setStatus = (value: string) => {
    void setStatusParam(value);
    void setPage(1);
  };
  const offset = (Math.max(page, 1) - 1) * limit;
  const query = useGetNodeMonitors(
    nodeId,
    { status: selected.length > 0 ? selected.join(",") : undefined, limit, offset },
    { query: { placeholderData: keepPreviousData } },
  );
  const shown = query.data?.monitors ?? [];
  const total = query.data?.count ?? shown.length;
  const hidden = query.data?.hidden_count ?? 0;
  const pagination = {
    count: total,
    limit,
    offset,
    onOffsetChange: (next: number) => void setPage(Math.floor(next / limit) + 1),
    onLimitChange: setLimit,
  };
  const pageItems = shown;
  const isDefault = statusParam === DEFAULT;

  // A stale ?monitorsPage past the end would show an empty tab with no way back.
  useEffect(() => {
    if (total > 0 && offset >= total) void setPage(Math.ceil(total / limit));
  }, [limit, offset, setPage, total]);

  return (
    <div className="flex flex-col gap-3">
      <FilterTokenRow onClearAll={() => void setStatus(DEFAULT)}>
        {selected.length > 0 && (
          <FilterToken
            field="Status"
            operator="is one of"
            value={labelFor(selected)}
            onRemove={() => void setStatus(STATUS_OPTIONS.map((o) => o.value).join(","))}
          >
            <CheckFilterPanel
              title="Status"
              options={STATUS_OPTIONS}
              selected={selected}
              onChange={(next) => void setStatus(next.join(","))}
              onDone={() => undefined}
            />
          </FilterToken>
        )}
        <AddFilterButton>
          <PopoverContent className="w-[200px] p-1.5">
            <PopoverClose
              onClick={() => void setStatus(selected.length ? statusParam : DEFAULT)}
              className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
            >
              Status
            </PopoverClose>
          </PopoverContent>
        </AddFilterButton>
      </FilterTokenRow>
      {query.isPending ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : query.isError ? (
        <EmptyState tone="error" title="Unable to load monitors" />
      ) : shown.length === 0 ? (
        <EmptyState
          title={isDefault ? "No unhealthy monitors" : "No monitors match"}
          description={
            hidden > 0
              ? `${hidden} monitors are hidden by the status filter.`
              : "No monitors are registered on this server."
          }
        />
      ) : (
        <div
          className={cn(
            "ds-settled overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
            query.isPlaceholderData && "ds-fetching",
          )}
        >
          <GridTable>
            <GridRow columns={MONITOR_COLUMNS} header>
              <span>Monitor</span>
              <span>Type</span>
              <span>Status</span>
              <span>Detail</span>
            </GridRow>
            {pageItems.map((monitor) => (
              <GridRow key={monitor.id} flipId={monitor.id} columns={MONITOR_COLUMNS}>
                <MonitorCells monitor={monitor} />
              </GridRow>
            ))}
            {hidden > 0 && (
              <GridFooter>
                {hidden} {hidden === 1 ? "monitor is" : "monitors are"} hidden.{" "}
                <button
                  type="button"
                  className="underline"
                  onClick={() => void setStatus(STATUS_OPTIONS.map((o) => o.value).join(","))}
                >
                  Show them
                </button>{" "}
                ·{" "}
                <Link className="underline" to={`/monitors?target=server:${nodeId}`}>
                  Open in Monitors
                </Link>
              </GridFooter>
            )}
          </GridTable>
          <ListPagination {...pagination} />
        </div>
      )}
    </div>
  );
};
