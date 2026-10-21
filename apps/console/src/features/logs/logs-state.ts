import { parseAsString, useQueryStates } from "nuqs";

export type LogKind = "service" | "status" | "internal";

export const RANGES = [
  { value: "15m", label: "Last 15 minutes" },
  { value: "1h", label: "Last hour" },
  { value: "24h", label: "Last 24 hours" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
] as const;

export const rangeLabel = (range: string) =>
  range === "custom" ? "Custom range" : (RANGES.find((r) => r.value === range)?.label ?? range);

/** Filters shared by every log screen; kept in the URL so a view can be copied as a link. */
export const useLogQuery = () => {
  const [state, set] = useQueryStates({
    tab: parseAsString.withDefault("service"),
    view: parseAsString.withDefault("list"),
    q: parseAsString.withDefault(""),
    range: parseAsString.withDefault("15m"),
    from: parseAsString.withDefault(""),
    to: parseAsString.withDefault(""),
    source: parseAsString.withDefault(""),
    severity: parseAsString.withDefault(""),
    event: parseAsString.withDefault(""),
    env: parseAsString.withDefault(""),
    type: parseAsString.withDefault(""),
    server: parseAsString.withDefault(""),
    monitor: parseAsString.withDefault(""),
    component: parseAsString.withDefault(""),
    attr: parseAsString.withDefault(""),
    limit: parseAsString.withDefault("25"),
    log: parseAsString.withDefault(""),
  });
  return { state, set };
};

export type LogQueryState = ReturnType<typeof useLogQuery>["state"];

const csv = (value: string) => value.split(",").filter(Boolean);
export const toggleCsv = (value: string, item: string) => {
  const items = csv(value);
  return (items.includes(item) ? items.filter((x) => x !== item) : [...items, item]).join(",");
};
export const inCsv = (value: string, item: string) => csv(value).includes(item);

const timeWindow = (state: LogQueryState) =>
  state.range === "custom" && state.from
    ? { from: state.from, to: state.to || undefined }
    : { from: `-${state.range === "custom" ? "15m" : state.range}`, to: undefined };

/** API filter params for the current tab (without paging). */
export const filterParams = (state: LogQueryState, kind: LogKind) => {
  const window = timeWindow(state);
  const q = state.q.trim() || undefined;
  if (kind === "status") {
    return {
      ...window,
      q,
      level: state.severity || undefined,
      type: state.type || undefined,
      server_id: state.server || undefined,
      monitor_id: state.monitor || undefined,
    };
  }
  const attrs = [state.attr, state.component ? `component:${state.component}` : ""]
    .filter(Boolean)
    .join(",");
  return {
    ...window,
    q,
    severity: state.severity || undefined,
    attr: attrs || undefined,
    ...(kind === "service"
      ? {
          app_id: state.source || undefined,
          event: state.event || undefined,
        }
      : {}),
  };
};

/** The screen's filter as a URL query string, as the export endpoint expects. */
export const toQueryString = (params: Record<string, string | undefined>) =>
  new URLSearchParams(
    Object.entries(params).filter((entry): entry is [string, string] => Boolean(entry[1])),
  ).toString();
