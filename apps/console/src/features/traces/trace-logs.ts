// Which log lines the trace page shows, and how many sit on each span. Pure, no React.
import type { ApiLogEntry } from "../../orion-sdk/index.ts";

const stamp = (entry: ApiLogEntry) => {
  const t = entry.timestamp ? new Date(entry.timestamp).getTime() : Number.NaN;
  return Number.isFinite(t) ? t : 0;
};

/** Log lines in time order, narrowed to one span when asked. The input is left alone. */
export const logsForPanel = (entries: readonly ApiLogEntry[], spanId?: string): ApiLogEntry[] =>
  entries
    .filter((entry) => !spanId || entry.span_id === spanId)
    .sort((a, b) => stamp(a) - stamp(b));

/** Log lines per span id, for the inspector's "N lines on this span". */
export const countBySpan = (entries: readonly ApiLogEntry[]): Map<string, number> => {
  const counts = new Map<string, number>();
  for (const entry of entries) {
    if (entry.span_id) counts.set(entry.span_id, (counts.get(entry.span_id) ?? 0) + 1);
  }
  return counts;
};
