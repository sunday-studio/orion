// Filters of the Traces page and how they become Core query parameters. Pure, no React or nuqs,
// so the rules run under `node --test`. The nuqs hook that keeps them in the URL is in use-trace-query.ts.
import { normalizeMinDuration } from "./trace-format.ts";

export const TRACE_RANGES = [
  { value: "15m", label: "Last 15 minutes" },
  { value: "1h", label: "Last hour" },
  { value: "4h", label: "Last 4 hours" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
] as const;

export const DEFAULT_TRACE_RANGE = "1h";

export type TraceFilters = {
  /** App ids, comma separated. */
  app: string;
  /** service.name of the root span. */
  service: string;
  /** Orion service id: every app linked to it. */
  serviceId: string;
  status: string;
  minDuration: string;
  /** Substring of the root span name. */
  q: string;
  range: string;
  from: string;
  to: string;
  /** key:value, matched against spans and resources. */
  attr: string;
};

export const EMPTY_TRACE_FILTERS: TraceFilters = {
  app: "",
  service: "",
  serviceId: "",
  status: "",
  minDuration: "",
  q: "",
  range: DEFAULT_TRACE_RANGE,
  from: "",
  to: "",
  attr: "",
};

export const traceRangeLabel = (range: string) =>
  range === "custom"
    ? "Custom range"
    : (TRACE_RANGES.find((item) => item.value === range)?.label ?? range);

const knownRange = (range: string) => TRACE_RANGES.some((item) => item.value === range);

/** A key:value pair with both halves present, or null. The value may hold colons. */
export const parseAttr = (input: string): { key: string; value: string } | null => {
  const text = input.trim();
  const at = text.indexOf(":");
  if (at < 1) return null;
  const key = text.slice(0, at).trim();
  const value = text.slice(at + 1).trim();
  return key && value ? { key, value } : null;
};

export const normalizeStatus = (status: string): "ok" | "error" | undefined =>
  status === "ok" || status === "error" ? status : undefined;

/** The time window as Core takes it: relative, or two RFC 3339 times for a custom range. */
export const traceWindow = (filters: Pick<TraceFilters, "range" | "from" | "to">) => {
  if (filters.range === "custom" && filters.from) {
    return { from: filters.from, to: filters.to || undefined };
  }
  const range = knownRange(filters.range) ? filters.range : DEFAULT_TRACE_RANGE;
  return { from: `-${range}`, to: undefined };
};

export type TraceQueryParams = {
  app_id?: string;
  service_id?: string;
  service?: string;
  status?: "ok" | "error";
  min_duration?: string;
  name?: string;
  from: string;
  to?: string;
  attr?: string;
};

/** Filters for GET /v1/traces and GET /v1/traces/facets. Empty or invalid values are left out. */
export const traceQueryParams = (filters: TraceFilters): TraceQueryParams => {
  const attr = parseAttr(filters.attr);
  return {
    ...traceWindow(filters),
    app_id: filters.app || undefined,
    service_id: filters.serviceId || undefined,
    service: filters.service || undefined,
    status: normalizeStatus(filters.status),
    min_duration: normalizeMinDuration(filters.minDuration) || undefined,
    name: filters.q.trim() || undefined,
    attr: attr ? `${attr.key}:${attr.value}` : undefined,
  };
};

export type FilterToken = { key: keyof TraceFilters | "range"; field: string; value: string };

/** One token per applied filter, in the order they sit in the toolbar. The time range is not a token. */
export const filterTokens = (
  filters: TraceFilters,
  labels: { app?: (id: string) => string; serviceId?: (id: string) => string } = {},
): FilterToken[] => {
  const tokens: FilterToken[] = [];
  const add = (key: keyof TraceFilters, field: string, value: string | undefined) => {
    if (value) tokens.push({ key, field, value });
  };
  add(
    "serviceId",
    "Service",
    filters.serviceId ? (labels.serviceId?.(filters.serviceId) ?? filters.serviceId) : "",
  );
  add(
    "app",
    "App",
    filters.app
      ? filters.app
          .split(",")
          .filter(Boolean)
          .map((id) => labels.app?.(id) ?? id)
          .join(", ")
      : "",
  );
  add("service", "Root service", filters.service);
  add("status", "Status", normalizeStatus(filters.status));
  add(
    "minDuration",
    "Duration",
    normalizeMinDuration(filters.minDuration)
      ? `≥ ${normalizeMinDuration(filters.minDuration)}`
      : "",
  );
  add("q", "Name", filters.q.trim());
  const attr = parseAttr(filters.attr);
  add("attr", "Attribute", attr ? `${attr.key} = ${attr.value}` : "");
  return tokens;
};

export const hasTraceFilters = (filters: TraceFilters) =>
  filterTokens(filters).length > 0 || filters.range !== DEFAULT_TRACE_RANGE;

/** The slowest root span on a page of rows, so each row's bar can be drawn relative to it. */
export const slowestDuration = (rows: readonly { duration_ns?: number }[]): number =>
  rows.reduce((max, row) => {
    const value = row.duration_ns;
    return typeof value === "number" && Number.isFinite(value) && value > max ? value : max;
  }, 0);

/** Width of a duration bar, 0 to 100 percent of the slowest trace. A non-zero duration is never invisible. */
export const durationShare = (durationNs: number | undefined, slowestNs: number): number => {
  if (!durationNs || !Number.isFinite(durationNs) || durationNs <= 0 || slowestNs <= 0) return 0;
  return Math.min(Math.max((durationNs / slowestNs) * 100, 2), 100);
};

/** The facet counts for ok and error traces, whichever status filter is set. */
export const statusCounts = (
  facets?: { value?: string; count?: number }[],
): { ok: number; error: number } => {
  const count = (value: string) => facets?.find((item) => item.value === value)?.count ?? 0;
  return { ok: count("ok"), error: count("error") };
};

/**
 * How many traces match the filters, from the facets. Core's total ignores the status filter (the
 * ok and error counts do not), so a status filter takes its own count.
 */
export const matchingTotal = (
  facets: { total?: number; status?: { value?: string; count?: number }[] } | undefined,
  status: string,
): number | undefined => {
  if (!facets) return undefined;
  const wanted = normalizeStatus(status);
  if (!wanted) return facets.total;
  const counts = statusCounts(facets.status);
  return wanted === "ok" ? counts.ok : counts.error;
};
