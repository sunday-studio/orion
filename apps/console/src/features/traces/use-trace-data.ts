import { keepPreviousData } from "@tanstack/react-query";
import { useCursorStack } from "@/features/logs/use-log-data";
import {
  type ListTracesParams,
  useGetTrace,
  useGetTraceFacets,
  useListTraces,
  useSearchLogs,
} from "@/orion-sdk";
import { type TraceFilters, traceQueryParams } from "./traces-state";

const keep = { placeholderData: keepPreviousData };

/** An error from Core's API client carries the HTTP status. */
export const errorStatus = (error: unknown) => (error as { status?: number } | null)?.status;

/** One page of traces, with a cursor stack so "Newer" and "Older" both work. */
export const useTraceList = (filters: TraceFilters, limit: number) => {
  const params = traceQueryParams(filters);
  const paging = useCursorStack(`${JSON.stringify(params)}:${limit}`);
  const query = useListTraces({ ...params, limit, cursor: paging.cursor } as ListTracesParams, {
    query: keep,
  });
  const next = query.data?.next_cursor;
  return {
    rows: query.data?.traces ?? [],
    hasOlder: Boolean(next),
    hasNewer: paging.page > 0,
    page: paging.page,
    older: () => next && paging.older(next),
    newer: paging.newer,
    isPending: query.isPending,
    isError: query.isError,
    error: query.error,
    isPlaceholderData: query.isPlaceholderData,
    refetch: () => void query.refetch(),
  };
};

/** Counts for the filter bar: status split and the most common services and root names. */
export const useTraceFacets = (filters: TraceFilters) =>
  useGetTraceFacets(traceQueryParams(filters), { query: keep });

export const useTrace = (traceId: string) =>
  useGetTrace(traceId, { query: { enabled: Boolean(traceId), retry: false } });

/** Log lines that carry the trace id, over the window Logs would open. */
export const useTraceLogs = (traceId: string, from?: string, to?: string, enabled = true) =>
  useSearchLogs(
    { trace_id: traceId, from, to, limit: 100 },
    { query: { enabled: enabled && Boolean(traceId && from) } },
  );
