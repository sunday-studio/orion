import { parseAsString, useQueryStates } from "nuqs";
import { DEFAULT_TRACE_RANGE, type TraceFilters } from "./traces-state";

/** Traces page filters, kept in the URL so a view can be shared as a link. */
export const useTraceQuery = () => {
  const [state, set] = useQueryStates({
    app: parseAsString.withDefault(""),
    service: parseAsString.withDefault(""),
    serviceId: parseAsString.withDefault(""),
    status: parseAsString.withDefault(""),
    minDuration: parseAsString.withDefault(""),
    q: parseAsString.withDefault(""),
    range: parseAsString.withDefault(DEFAULT_TRACE_RANGE),
    from: parseAsString.withDefault(""),
    to: parseAsString.withDefault(""),
    attr: parseAsString.withDefault(""),
    limit: parseAsString.withDefault("25"),
  });
  return { state, set };
};

export type TraceQueryState = ReturnType<typeof useTraceQuery>["state"];

export const toFilters = (state: TraceQueryState): TraceFilters => ({
  app: state.app,
  service: state.service,
  serviceId: state.serviceId,
  status: state.status,
  minDuration: state.minDuration,
  q: state.q,
  range: state.range,
  from: state.from,
  to: state.to,
  attr: state.attr,
});

/** Page sizes the toolbar offers; Core allows up to 200. */
export const PAGE_SIZES = ["25", "50", "100", "200"] as const;

export const pageLimit = (value: string) =>
  (PAGE_SIZES as readonly string[]).includes(value) ? Number(value) : 25;
