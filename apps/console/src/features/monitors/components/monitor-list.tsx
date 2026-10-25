import { keepPreviousData } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { parseAsString, useQueryStates } from "nuqs";
import { useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  Checkbox,
  EmptyState,
  GridRow,
  GridTable,
  Input,
  ListPagination,
  Segmented,
  Select,
  Skeleton,
  StatusBadge,
  usePageSize,
} from "@/design-system";
import { cn } from "@/utils/cn";
import {
  type CoreWorkerDiagnostics,
  CoreWorkerWarning,
  shouldWarnForCoreWorker,
} from "@/features/monitors/components/core-worker-diagnostics";
import {
  type GetMonitorsParams,
  useGetMonitorSummary,
  useGetMonitors,
  useGetNodes,
} from "@/orion-sdk";
import {
  type MonitorRow,
  rowEvery,
  rowHealth,
  statusLabel,
  targetLabel,
  typeFamily,
} from "../monitor-format";
import { CheckStrip } from "./check-strip";
import { MonitorBulkBar } from "./monitor-bulk-bar";

const COLUMNS = "18px minmax(0,1.1fr) minmax(0,1.5fr) 124px 44px 100px";

const HEALTH_SEGMENTS = ["all", "down", "degraded", "up"] as const;

export const MonitorList = ({
  workerDiagnostics,
}: {
  workerDiagnostics?: CoreWorkerDiagnostics;
}) => {
  const navigate = useNavigate();
  const [{ q, health, type, server, page }, setQuery] = useQueryStates({
    q: parseAsString.withDefault(""),
    health: parseAsString.withDefault("all"),
    type: parseAsString.withDefault(""),
    server: parseAsString.withDefault(""),
    page: parseAsString.withDefault("1"),
  });
  const { limit, setLimit } = usePageSize(20);
  const pageNumber = Math.max(Number(page) || 1, 1);
  const offset = (pageNumber - 1) * limit;
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());

  const params: GetMonitorsParams = {
    limit,
    offset,
    q: q.trim() || undefined,
    status: health === "all" ? undefined : health,
    type: type || undefined,
    owner_kind: server === "core" ? "core" : undefined,
    node_id: server && server !== "core" ? server : undefined,
    sort: "updated_at",
    order: "desc",
  };
  const monitorsQuery = useGetMonitors(params, { query: { placeholderData: keepPreviousData } });
  const summary = useGetMonitorSummary();
  const nodes = useGetNodes({ limit: 200 });
  const monitors = useMemo(
    () => (monitorsQuery.data?.monitors ?? []) as MonitorRow[],
    [monitorsQuery.data],
  );
  // `count` is the filtered total; `total` ignores the status filter.
  const total = monitorsQuery.data?.count ?? monitors.length;
  const filtersActive = Boolean(q.trim() || health !== "all" || type || server);

  const typeOptions = useMemo(() => {
    const seen = new Set<string>();
    for (const monitor of monitors) if (monitor.type) seen.add(monitor.type);
    if (type) seen.add(type);
    return [...seen].sort().map((value) => ({ value, label: typeFamily(value) }));
  }, [monitors, type]);
  const serverOptions = [
    { value: "core", label: "Core" },
    ...(nodes.data?.nodes ?? []).map((node) => ({
      value: node.id ?? "",
      label: node.name ?? node.id ?? "Server",
    })),
  ];

  const counts = {
    all: summary.data?.total,
    down: summary.data?.down,
    degraded: summary.data?.degraded,
    up: summary.data?.up,
  };
  const toggle = (id: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const allSelected = monitors.length > 0 && monitors.every((m) => m.id && selected.has(m.id));
  const reset = () => void setQuery({ q: "", health: "all", type: "", server: "", page: "1" });

  const showWarning =
    monitors.some((m) => m.owner_kind === "core" || m.source === "core") &&
    shouldWarnForCoreWorker(workerDiagnostics);

  return (
    <div className="mt-3 flex flex-col gap-3">
      {showWarning && <CoreWorkerWarning worker={workerDiagnostics} />}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-[260px]">
          <Search
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint"
          />
          <Input
            aria-label="Search monitors"
            placeholder="Search monitors"
            className="pl-9"
            value={q}
            onChange={(event) => void setQuery({ q: event.target.value, page: "1" })}
          />
        </div>
        <Segmented
          role="group"
          aria-label="Monitor health"
          options={HEALTH_SEGMENTS.map((value) => ({
            value,
            label: `${value.charAt(0).toUpperCase()}${value.slice(1)}${
              counts[value] === undefined ? "" : ` ${counts[value]}`
            }`,
          }))}
          value={health}
          onValueChange={(next) => void setQuery({ health: next, page: "1" })}
        />
        <span className="flex-1" />
        <Select
          aria-label="Type"
          label="Type"
          className="min-w-[120px]"
          options={[{ value: "", label: "Any" }, ...typeOptions]}
          value={type}
          onValueChange={(next) => void setQuery({ type: next, page: "1" })}
        />
        <Select
          aria-label="Server"
          label="Server"
          className="min-w-[120px]"
          options={[{ value: "", label: "Any" }, ...serverOptions]}
          value={server}
          onValueChange={(next) => void setQuery({ server: next, page: "1" })}
        />
      </div>
      {monitorsQuery.isPending ? (
        <Skeleton className="h-40 w-full rounded-xl" />
      ) : monitorsQuery.isError ? (
        <EmptyState
          tone="error"
          title="Unable to load monitors"
          action={
            <Button size="sm" variant="secondary" onClick={() => void monitorsQuery.refetch()}>
              Retry
            </Button>
          }
        />
      ) : monitors.length === 0 ? (
        <EmptyState
          title="No monitors match"
          description="Clear the search or filters to see every monitor."
          action={
            filtersActive ? (
              <Button size="sm" variant="secondary" onClick={reset}>
                Clear filters
              </Button>
            ) : undefined
          }
        />
      ) : (
        <div
          className={cn(
            "ds-settled overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
            monitorsQuery.isPlaceholderData && "ds-fetching",
          )}
        >
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>
                <Checkbox
                  aria-label="Select all monitors"
                  checked={allSelected}
                  onCheckedChange={(checked) =>
                    setSelected(checked ? new Set(monitors.map((m) => m.id ?? "")) : new Set())
                  }
                />
              </span>
              <span>Name</span>
              <span>Target</span>
              <span>Status</span>
              <span>Every</span>
              <span>Last 24 checks</span>
            </GridRow>
            {monitors.map((monitor) => {
              const id = monitor.id ?? "";
              const isSelected = selected.has(id);
              return (
                <GridRow
                  key={id}
                  flipId={id}
                  columns={COLUMNS}
                  className={isSelected ? "bg-[var(--color-selected-row)]" : "cursor-pointer"}
                  onClick={() => navigate(`/monitors/${id}`)}
                >
                  <span onClick={(event) => event.stopPropagation()}>
                    <Checkbox
                      aria-label={`Select ${monitor.name}`}
                      checked={isSelected}
                      onCheckedChange={() => toggle(id)}
                    />
                  </span>
                  <Link
                    to={`/monitors/${id}`}
                    className="truncate font-medium outline-none hover:underline focus-visible:underline"
                    onClick={(event) => event.stopPropagation()}
                  >
                    {monitor.name ?? id}
                  </Link>
                  <span className="text-faint">{targetLabel(monitor)}</span>
                  <span>
                    <StatusBadge status={rowHealth(monitor)}>{statusLabel(monitor)}</StatusBadge>
                  </span>
                  <span className="text-faint">{rowEvery(monitor)}</span>
                  <CheckStrip checks={monitor.recent_checks} />
                </GridRow>
              );
            })}
          </GridTable>
          <ListPagination
            count={total}
            limit={limit}
            offset={offset}
            onOffsetChange={(next) => void setQuery({ page: String(Math.floor(next / limit) + 1) })}
            onLimitChange={setLimit}
          />
        </div>
      )}
      {selected.size > 0 && (
        <MonitorBulkBar
          ids={[...selected]}
          nodeIds={Object.fromEntries(
            monitors
              .filter((m) => m.id && m.node_id && selected.has(m.id))
              .map((m) => [m.id ?? "", m.node_id ?? ""]),
          )}
          onClear={() => setSelected(new Set())}
          onSucceeded={(done) =>
            setSelected((current) => new Set([...current].filter((id) => !done.includes(id))))
          }
        />
      )}
    </div>
  );
};
