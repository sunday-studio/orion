// Links between Logs, Traces and services. Pure, so the URLs can be tested. The query keys match
// the nuqs keys in use-trace-query.ts (Traces) and logs-state.ts (Logs).
import type { TraceFilters } from "./traces-state.ts";

export const TRACES_PATH = "/logs/traces";

export const traceHref = (traceId: string, span?: string) => {
  const base = `${TRACES_PATH}/${encodeURIComponent(traceId)}`;
  return span ? `${base}?span=${encodeURIComponent(span)}` : base;
};

/** The Traces list with some filters set. Empty ones stay out of the URL. */
export const tracesHref = (filters: Partial<TraceFilters>) => {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value) params.set(key, value);
  }
  const query = params.toString();
  return query ? `${TRACES_PATH}?${query}` : TRACES_PATH;
};

/** Traces whose spans or resource carry `key` = `value`. */
export const attributeFilterHref = (key: string, value: string, range?: string) =>
  tracesHref({ attr: `${key}:${value}`, range });

const HOUR_MS = 3_600_000;

/** The window to search for a trace's log lines: the trace plus an hour on each side. Null without a valid start. */
export const traceLogWindow = (
  startTime?: string,
  durationNs?: number,
): { from: string; to: string } | null => {
  const start = startTime ? new Date(startTime).getTime() : Number.NaN;
  if (!Number.isFinite(start)) return null;
  const length = durationNs && Number.isFinite(durationNs) && durationNs > 0 ? durationNs / 1e6 : 0;
  return {
    from: new Date(start - HOUR_MS).toISOString(),
    to: new Date(start + length + HOUR_MS).toISOString(),
  };
};

/**
 * Every log line of one trace in the Logs screen. The window leaves room on both sides, so exporters
 * whose clocks drift or flush late still land inside it.
 */
export const logsHrefForTrace = (traceId: string, startTime?: string, durationNs?: number) => {
  const params = new URLSearchParams({ trace: traceId });
  const window = traceLogWindow(startTime, durationNs);
  if (window) {
    params.set("range", "custom");
    params.set("from", window.from);
    params.set("to", window.to);
  } else {
    params.set("range", "7d");
  }
  return `/logs?${params.toString()}`;
};
