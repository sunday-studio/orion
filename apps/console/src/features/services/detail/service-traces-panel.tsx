import { useState } from "react";
import { Link } from "react-router-dom";
import { Badge, Box, BoxHeader, Button, Segmented, Skeleton, StatBox } from "@/design-system";
import { focusRing } from "@/design-system/styles";
import { formatDurationNs } from "@/features/traces/trace-format";
import { traceHref, tracesHref } from "@/features/traces/trace-links";
import { errorStatus } from "@/features/traces/use-trace-data";
import { type ApiTraceListItem, useGetServiceTraceSummary, useListTraces } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { recentTraces, slowestTraces, traceSummaryView } from "../traces-model";
import { type RangeValue, rangeFromParam } from "../timeline-model";

const SHOWN = 5;
// The newest page Core returns is what "slowest" ranks, so the label says so.
const PAGE = 50;

const TraceRow = ({ item }: { item: ApiTraceListItem }) => (
  <li className="border-t border-line-soft first:border-t-0">
    <Link
      to={traceHref(item.trace_id ?? "")}
      className={cn(
        focusRing,
        "grid grid-cols-[minmax(0,1fr)_auto_64px] items-center gap-3 px-3.5 py-2 text-[13px] transition-colors duration-150 hover:bg-sub focus-visible:ring-inset",
      )}
    >
      <span className="min-w-0">
        <b className="block truncate font-medium">{item.root_name || "(unnamed)"}</b>
        <span className="text-xs text-faint">
          {formatDate(item.start_time, `MMM d, ${TIME_FORMAT}:ss`)}
          {(item.span_count ?? 0) > 0 && ` · ${item.span_count} spans`}
        </span>
      </span>
      {item.status === "error" ? <Badge tone="down">Error</Badge> : <span />}
      <span className="text-right tabular-nums">{formatDurationNs(item.duration_ns)}</span>
    </Link>
  </li>
);

/** Trace count, error rate and latency of the apps linked to the service, over the page's range. */
export const ServiceTracesPanel = ({
  serviceId,
  range: rangeValue,
  hasApps,
}: {
  serviceId: string;
  range: RangeValue;
  hasApps: boolean;
}) => {
  const range = rangeFromParam(rangeValue);
  const [order, setOrder] = useState<"recent" | "slowest">("recent");
  const enabled = hasApps;
  const summary = useGetServiceTraceSummary(
    serviceId,
    { from: range.from },
    { query: { enabled } },
  );
  const list = useListTraces(
    { service_id: serviceId, from: range.from, limit: PAGE },
    { query: { enabled } },
  );
  const view = traceSummaryView(summary.data);
  const items = list.data?.traces ?? [];
  const rows = order === "recent" ? recentTraces(items, SHOWN) : slowestTraces(items, SHOWN);
  const unavailable = [summary.error, list.error].some((error) => errorStatus(error) === 503);
  const failed = summary.isError || list.isError;
  const openAll = tracesHref({ serviceId, range: range.value });

  return (
    <Box>
      <BoxHeader
        title="Traces"
        aside={
          hasApps ? (
            <Link
              to={openAll}
              className="rounded text-[12.5px] text-mid underline hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
            >
              Open in Traces
            </Link>
          ) : undefined
        }
      />
      {!hasApps ? (
        <p className="px-3.5 py-3 text-[13px] text-quiet">
          Link an app that sends OpenTelemetry traces to see its traces, error rate and latency
          here.
        </p>
      ) : summary.isPending || list.isPending ? (
        <Skeleton className="m-3 h-28" />
      ) : failed ? (
        <div className="flex flex-wrap items-center justify-between gap-3 px-3.5 py-3 text-[13px]">
          <span role="alert" className="text-(--tone-down-fg)">
            {unavailable ? "Traces are not available on this Core." : "Unable to load traces."}
          </span>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              void summary.refetch();
              void list.refetch();
            }}
          >
            Retry
          </Button>
        </div>
      ) : view.empty ? (
        <p className="px-3.5 py-3 text-[13px] text-quiet">
          No traces in the last {range.label}. Point an OpenTelemetry traces exporter at a linked
          app and they appear here.
        </p>
      ) : (
        <div className="grid gap-3 p-3">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-5">
            <StatBox label={`Traces, ${range.label}`} value={view.traces} />
            <StatBox
              label="Error rate"
              value={
                <span className={view.errors !== "0" ? "text-(--tone-down-fg)" : undefined}>
                  {view.errorRate}
                  {view.errors !== "0" && (
                    <small className="ml-1.5 text-[13px] font-normal tracking-normal text-faint">
                      {view.errors} failed
                    </small>
                  )}
                </span>
              }
            />
            <StatBox label="p50" value={view.p50} />
            <StatBox label="p95" value={view.p95} />
            <StatBox label="p99" value={view.p99} />
          </div>
          {(view.sampled || view.noLatency) && (
            <p className="m-0 text-xs text-quiet">
              {view.noLatency
                ? "No trace in this range has its root span yet, so there is no latency."
                : "Percentiles use the newest 50,000 traces in this range."}
            </p>
          )}
          <div className="grid gap-1.5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-[13px] font-medium">
                {order === "recent" ? "Recent traces" : `Slowest of the latest ${PAGE}`}
              </h3>
              <Segmented
                aria-label="Order"
                size="sm"
                surface="card"
                role="group"
                options={[
                  { value: "recent", label: "Recent" },
                  { value: "slowest", label: "Slowest" },
                ]}
                value={order}
                onValueChange={(next) => setOrder(next as "recent" | "slowest")}
              />
            </div>
            <ul className="m-0 list-none overflow-hidden rounded-[10px] border border-hairline p-0">
              {rows.map((item) => (
                <TraceRow key={`${item.app_id}:${item.trace_id}`} item={item} />
              ))}
            </ul>
          </div>
        </div>
      )}
    </Box>
  );
};
