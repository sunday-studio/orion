// Merging and filtering for the service Logs tab. Core searches one app at a time, so the
// service page asks for each linked app and merges the lines. Pure, no React.
import type { ApiLogEntry } from "../../orion-sdk/index.ts";

export const LOG_RANGES = [
  { value: "15m", label: "Last 15 minutes" },
  { value: "1h", label: "Last hour" },
  { value: "2h", label: "Last 2 hours" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
] as const;

export const LEVEL_FILTERS = [
  { value: "all", label: "All levels" },
  { value: "warn", label: "Warn and above" },
  { value: "error", label: "Error and above" },
] as const;

export const logRangeFromParam = (value: string | null | undefined) =>
  LOG_RANGES.find((range) => range.value === value)?.value ?? "2h";

/** Core accepts a minimum such as ">=warn". "all" sends no filter. */
export const severityParam = (level: string) =>
  level === "warn" || level === "error" ? `>=${level}` : undefined;

const stamp = (entry: ApiLogEntry) => {
  const t = entry.timestamp ? new Date(entry.timestamp).getTime() : Number.NaN;
  return Number.isFinite(t) ? t : 0;
};

/** Newest first across every app, without the same line twice, cut to `limit`. */
export const mergeLogs = (batches: readonly (readonly ApiLogEntry[])[], limit = 500) => {
  const seen = new Set<string>();
  const merged: ApiLogEntry[] = [];
  for (const entry of batches.flat()) {
    const key = entry.id ?? `${entry.app_id}:${entry.timestamp}:${entry.body}`;
    if (seen.has(key)) continue;
    seen.add(key);
    merged.push(entry);
  }
  return merged.sort((a, b) => stamp(b) - stamp(a)).slice(0, limit);
};

/** The ranges the Logs screen offers. The service tab also has 2h, which Logs rounds up to 24h. */
const LOGS_PAGE_RANGES = ["15m", "1h", "24h", "7d"];

export const logsPageRange = (range: string) =>
  LOGS_PAGE_RANGES.includes(range) ? range : range === "2h" ? "24h" : "1h";

/** Link for "Open in Logs": the Logs screen takes several app ids in `source`. */
export const openInLogsHref = (appIds: readonly string[], range: string) => {
  const params = new URLSearchParams({ range: logsPageRange(range) });
  if (appIds.length > 0) params.set("source", appIds.join(","));
  return `/logs?${params.toString()}`;
};
