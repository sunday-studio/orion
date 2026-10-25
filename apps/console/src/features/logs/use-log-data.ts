import { keepPreviousData } from "@tanstack/react-query";
import { useState } from "react";
import {
  type ApiLogEntry,
  useGetInternalLogFacets,
  useGetInternalLogHistogram,
  useGetInternalLogPatterns,
  useGetLogFacets,
  useGetLogHistogram,
  useGetLogPatterns,
  useGetStatusLogFacets,
  useGetStatusLogHistogram,
  useGetStatusLogPatterns,
  useListStatusLogs,
  useSearchInternalLogs,
  useSearchLogs,
} from "@/orion-sdk";
import { type LogRow, serviceRow, statusRow } from "./log-model";
import { type LogKind, type LogQueryState, filterParams } from "./logs-state";

const keep = { query: { placeholderData: keepPreviousData } };

export type FacetItem = { value: string; label: string; count: number; disabled?: boolean };
export type FacetGroup = { key: string; title: string; items: FacetItem[]; more?: number };

export const isSearchTimeout = (error: unknown) =>
  (error as { status?: number } | null)?.status === 504;

/** A stack of cursors: the last one is the current page, popping it goes to newer lines. */
const useCursorStack = (resetKey: string) => {
  const [stack, setStack] = useState<{ key: string; cursors: string[] }>({
    key: resetKey,
    cursors: [],
  });
  const cursors = stack.key === resetKey ? stack.cursors : [];
  return {
    cursor: cursors.at(-1),
    page: cursors.length,
    older: (next: string) => setStack({ key: resetKey, cursors: [...cursors, next] }),
    newer: () => setStack({ key: resetKey, cursors: cursors.slice(0, -1) }),
  };
};

const num = (record?: Record<string, number>) => record ?? {};

export const useLogList = (kind: LogKind, state: LogQueryState, limit: number, enabled = true) => {
  const params = filterParams(state, kind);
  const paging = useCursorStack(`${kind}:${JSON.stringify(params)}:${limit}`);
  const opts = { query: { ...keep.query, enabled } };
  const service = useSearchLogs(
    { ...params, limit, cursor: paging.cursor },
    { query: { ...opts.query, enabled: enabled && kind === "service" } },
  );
  const internal = useSearchInternalLogs(
    { ...params, limit, cursor: paging.cursor },
    { query: { ...opts.query, enabled: enabled && kind === "internal" } },
  );
  const status = useListStatusLogs(
    { ...params, limit, cursor: paging.cursor },
    { query: { ...opts.query, enabled: enabled && kind === "status" } },
  );
  const active = kind === "status" ? status : kind === "internal" ? internal : service;
  const rows: LogRow[] =
    kind === "status"
      ? (status.data?.entries ?? []).map(statusRow)
      : ((kind === "internal" ? internal.data : service.data)?.entries ?? []).map(
          (entry: ApiLogEntry) => serviceRow(entry),
        );
  const next =
    kind === "status"
      ? status.data?.next_cursor
      : active.data && "next_cursor" in active.data
        ? (active.data as { next_cursor?: string }).next_cursor
        : undefined;
  return {
    rows,
    total: kind === "status" ? status.data?.total : undefined,
    hasOlder: Boolean(next),
    hasNewer: paging.page > 0,
    page: paging.page,
    older: () => next && paging.older(next),
    newer: paging.newer,
    isPending: active.isPending,
    isError: active.isError,
    error: active.error,
    isPlaceholderData: active.isPlaceholderData,
    refetch: () => void active.refetch(),
  };
};

const useRawFacets = (
  kind: LogKind,
  state: LogQueryState,
  field?: string,
  top?: number,
  ready = true,
) => {
  const params = filterParams(state, kind);
  const service = useGetLogFacets(
    { ...params, field, events: top },
    { query: { ...keep.query, enabled: ready && kind === "service" } },
  );
  const internal = useGetInternalLogFacets(params, {
    query: { ...keep.query, enabled: ready && kind === "internal" },
  });
  const status = useGetStatusLogFacets(
    { ...params, top },
    {
      query: { ...keep.query, enabled: kind === "status" },
    },
  );
  const toItems = (items?: { value?: string; label?: string; count?: number }[]): FacetItem[] =>
    (items ?? []).map((item) => ({
      value: item.value ?? "",
      label: item.label ?? item.value ?? "",
      count: item.count ?? 0,
    }));
  const fromCounts = (
    record: Record<string, number>,
    order: string[],
    label: (v: string) => string,
  ) =>
    order
      .filter((key) => key in record)
      .map((key) => ({ value: key, label: label(key), count: record[key] }));

  if (kind === "status") {
    const data = status.data;
    const groups: FacetGroup[] = [
      {
        key: "severity",
        title: "Level",
        items: fromCounts(num(data?.levels), ["error", "warn", "info"], (v) =>
          v === "warn" ? "Warning" : v[0].toUpperCase() + v.slice(1),
        ),
      },
      {
        key: "type",
        title: "Type",
        items: Object.entries(num(data?.types)).map(([value, count]) => ({
          value,
          label: value,
          count,
        })),
      },
      {
        key: "server",
        title: "Server",
        items: toItems(data?.servers),
        more: Math.max((data?.servers_distinct ?? 0) - (data?.servers?.length ?? 0), 0),
      },
      {
        key: "monitor",
        title: "Monitor",
        items: toItems(data?.monitors),
        more: Math.max((data?.monitors_distinct ?? 0) - (data?.monitors?.length ?? 0), 0),
      },
    ];
    return { groups, total: data?.total, fields: [], values: [], isPending: status.isPending };
  }
  const data = (kind === "internal" ? internal : service).data;
  const severity = fromCounts(num(data?.severity), ["error", "warn", "info", "debug"], (v) =>
    kind === "internal" && v === "warn" ? "Warning" : v[0].toUpperCase() + v.slice(1),
  );
  const groups: FacetGroup[] =
    kind === "internal"
      ? [
          { key: "severity", title: "Level", items: severity },
          { key: "component", title: "Component", items: toItems(data?.values ?? data?.sources) },
        ]
      : [
          { key: "severity", title: "Severity", items: severity },
          { key: "source", title: "Source", items: toItems(data?.sources) },
          { key: "env", title: "Environment", items: toItems(data?.environments) },
          {
            key: "event",
            title: "Event",
            items: toItems(data?.events),
            more: Math.max((data?.events_distinct ?? 0) - (data?.events?.length ?? 0), 0),
          },
        ];
  return {
    groups,
    total: data?.total,
    fields: data?.fields ?? [],
    values: toItems(data?.values),
    isPending: (kind === "internal" ? internal : service).isPending,
  };
};

export const useLogHistogram = (kind: LogKind, state: LogQueryState, ready = true) => {
  const params = filterParams(state, kind);
  const service = useGetLogHistogram(params, {
    query: { ...keep.query, enabled: ready && kind === "service" },
  });
  const status = useGetStatusLogHistogram(params, {
    query: { ...keep.query, enabled: kind === "status" },
  });
  const internal = useGetInternalLogHistogram(params, {
    query: { ...keep.query, enabled: kind === "internal" },
  });
  const active = kind === "status" ? status : kind === "internal" ? internal : service;
  return { data: active.data, isPending: active.isPending, isError: active.isError };
};

export const useLogPatterns = (kind: LogKind, state: LogQueryState, enabled: boolean) => {
  const params = filterParams(state, kind);
  const service = useGetLogPatterns(params, {
    query: { enabled: enabled && kind === "service" },
  });
  const status = useGetStatusLogPatterns(params, {
    query: { enabled: enabled && kind === "status" },
  });
  const internal = useGetInternalLogPatterns(params, {
    query: { enabled: enabled && kind === "internal" },
  });
  const active = kind === "status" ? status : kind === "internal" ? internal : service;
  return {
    patterns: active.data?.patterns ?? [],
    isPending: active.isPending,
    isError: active.isError,
    error: active.error,
    refetch: () => void active.refetch(),
  };
};

const GROUP_STATE: Record<string, keyof LogQueryState> = {
  severity: "severity",
  env: "env",
  source: "source",
  event: "event",
  type: "type",
  server: "server",
  monitor: "monitor",
  component: "component",
};

/**
 * Facets for the sidebar. Every option stays listed (from the search without facet filters);
 * options that no result matches under the other selected filters are disabled instead of
 * disappearing, so picking one never removes its siblings.
 */
export const useLogFacets = (
  kind: LogKind,
  state: LogQueryState,
  field?: string,
  top?: number,
  ready = true,
) => {
  const filtered = useRawFacets(kind, state, field, top, ready);
  const baseState = {
    ...state,
    source: "",
    severity: "",
    event: "",
    type: "",
    server: "",
    monitor: "",
    component: "",
    attr: "",
    env: "",
  };
  const base = useRawFacets(kind, baseState, undefined, top, ready);
  const groups = base.groups.map((group) => {
    const own = String(state[GROUP_STATE[group.key]] ?? "");
    const live = new Map(
      (filtered.groups.find((g) => g.key === group.key)?.items ?? []).map((item) => [
        item.value,
        item.count,
      ]),
    );
    return {
      ...group,
      items: group.items.map((item) => {
        const count = own ? item.count : (live.get(item.value) ?? 0);
        return { ...item, count, disabled: !own && count === 0 };
      }),
    };
  });
  return {
    ...filtered,
    groups: base.isPending ? filtered.groups : groups,
  };
};
