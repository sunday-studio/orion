// What the service page shows about traces. Pure, no React.
import type { ApiServiceTraceSummaryResponse, ApiTraceListItem } from "../../orion-sdk/index.ts";
import { formatDurationNs, formatPercent } from "../traces/trace-format.ts";

export type TraceSummaryView = {
  traces: string;
  errors: string;
  errorRate: string;
  p50: string;
  p95: string;
  p99: string;
  /** No trace in the range had its root span, so there is no latency to show. */
  noLatency: boolean;
  /** Percentiles come from the newest 50,000 traces only. */
  sampled: boolean;
  empty: boolean;
};

/** The summary as display strings. Absent numbers read "-", never zero. */
export const traceSummaryView = (summary?: ApiServiceTraceSummaryResponse): TraceSummaryView => {
  const count = summary?.trace_count ?? 0;
  const rooted = summary?.rooted_count ?? 0;
  const noLatency = rooted === 0;
  return {
    traces: count.toLocaleString(),
    errors: (summary?.error_count ?? 0).toLocaleString(),
    errorRate: count === 0 ? "-" : formatPercent(summary?.error_rate),
    p50: noLatency ? "-" : formatDurationNs(summary?.p50_ns),
    p95: noLatency ? "-" : formatDurationNs(summary?.p95_ns),
    p99: noLatency ? "-" : formatDurationNs(summary?.p99_ns),
    noLatency,
    sampled: summary?.sampled === true,
    empty: count === 0,
  };
};

const duration = (item: ApiTraceListItem) =>
  typeof item.duration_ns === "number" && Number.isFinite(item.duration_ns) ? item.duration_ns : -1;

/** The slowest of a page of traces, slowest first. The page is the newest ones, so this is not "slowest ever". */
export const slowestTraces = (
  items: readonly ApiTraceListItem[],
  count: number,
): ApiTraceListItem[] => [...items].sort((a, b) => duration(b) - duration(a)).slice(0, count);

/** The first `count` of a newest-first page. */
export const recentTraces = (items: readonly ApiTraceListItem[], count: number) =>
  items.slice(0, Math.max(count, 0));
