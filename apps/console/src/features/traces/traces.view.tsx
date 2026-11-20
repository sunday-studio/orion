import { useCallback } from "react";
import { Link } from "react-router-dom";
import { Button, FilterToken, FilterTokenRow, PageHeader, Skeleton } from "@/design-system";
import { useGetService, useListApps } from "@/orion-sdk";
import { TraceToolbar } from "./components/trace-filters";
import { NoTraces, TraceLoadError } from "./components/trace-states";
import { TraceTable } from "./components/trace-table";
import {
  EMPTY_TRACE_FILTERS,
  type TraceFilters,
  filterTokens,
  traceRangeLabel,
} from "./traces-state";
import { pageLimit, toFilters, useTraceQuery } from "./use-trace-query";
import { useTraceFacets, useTraceList } from "./use-trace-data";

export const TracesPage = () => {
  const { state, set } = useTraceQuery();
  const filters = toFilters(state);
  const limit = pageLimit(state.limit);
  const list = useTraceList(filters, limit);
  const facets = useTraceFacets(filters);
  const apps = useListApps();
  const service = useGetService(state.serviceId, { query: { enabled: Boolean(state.serviceId) } });

  const appOptions = (apps.data?.apps ?? []).flatMap((app) =>
    app.id ? [{ value: app.id, label: app.name ?? app.id }] : [],
  );
  const patch = useCallback((next: Partial<TraceFilters>) => void set(next), [set]);
  const clear = () => void set({ ...EMPTY_TRACE_FILTERS });

  const tokens = filterTokens(filters, {
    app: (id) => appOptions.find((option) => option.value === id)?.label ?? id,
    serviceId: (id) => service.data?.service?.name ?? id,
  });
  const range = traceRangeLabel(state.range).toLowerCase();
  const first = list.page * limit + 1;
  const total = facets.data?.total;
  const announce = list.isPending
    ? "Loading traces"
    : list.isError
      ? "Unable to load traces"
      : `${list.rows.length} ${list.rows.length === 1 ? "trace" : "traces"} shown${
          total === undefined ? "" : `, ${total.toLocaleString()} match`
        }`;

  const body = () => {
    if (list.isPending) return <Skeleton className="h-60 w-full rounded-xl" />;
    if (list.isError) {
      return <TraceLoadError error={list.error} onRetry={list.refetch} />;
    }
    if (list.rows.length === 0) {
      return (
        <NoTraces
          range={range}
          filtered={tokens.length > 0}
          onWiden={
            state.range === "24h" ? undefined : () => patch({ range: "24h", from: "", to: "" })
          }
          onClear={clear}
        />
      );
    }
    return (
      <TraceTable
        rows={list.rows}
        fetching={list.isPlaceholderData}
        paging={{
          from: first,
          to: first + list.rows.length - 1,
          total,
          hasNewer: list.hasNewer,
          hasOlder: list.hasOlder,
          limit: String(limit),
          onNewer: list.newer,
          onOlder: list.older,
          onLimit: (next) => void set({ limit: next }),
        }}
      />
    );
  };

  return (
    <div className="w-full pb-10">
      <PageHeader
        className="pb-5"
        title="Traces"
        description="One row per trace your apps send over OpenTelemetry"
        actions={
          <Link to="/logs">
            <Button size="sm" variant="secondary">
              Open logs
            </Button>
          </Link>
        }
      />
      <p role="status" aria-live="polite" className="sr-only">
        {announce}
      </p>
      <div className="flex flex-col gap-4">
        <TraceToolbar filters={filters} facets={facets.data} apps={appOptions} onChange={patch} />
        {tokens.length > 0 && (
          <FilterTokenRow onClearAll={clear}>
            {tokens.map((token) => (
              <FilterToken
                key={token.key}
                field={token.field}
                operator="is"
                value={token.value}
                onRemove={() => patch({ [token.key]: "" })}
              />
            ))}
          </FilterTokenRow>
        )}
        {body()}
      </div>
    </div>
  );
};
