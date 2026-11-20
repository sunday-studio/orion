import { parseAsString, useQueryState } from "nuqs";
import { useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import {
  Badge,
  Button,
  Callout,
  EmptyState,
  RaisedCard,
  Skeleton,
  StatBox,
  useToast,
} from "@/design-system";
import { formatDate } from "@/utils/date";
import { SpanInspector } from "./components/span-inspector";
import { TraceLoadError } from "./components/trace-states";
import { TraceLogsPanel } from "./components/trace-logs-panel";
import { Waterfall } from "./components/waterfall";
import { formatDurationNs, shortId } from "./trace-format";
import { TRACES_PATH, logsHrefForTrace, traceLogWindow } from "./trace-links";
import { countBySpan } from "./trace-logs";
import {
  buildTrace,
  defaultSelectedKey,
  errorKeys,
  parentKeys,
  revealed,
  visibleRows,
} from "./trace-tree";
import { errorStatus, useTrace, useTraceLogs } from "./use-trace-data";

const NotFound = () => (
  <EmptyState
    className="min-h-52 rounded-xl border-solid bg-card"
    title="Trace not found"
    description="It may be older than the trace retention, or its spans have not arrived yet."
    action={
      <Link to={TRACES_PATH}>
        <Button size="sm" variant="secondary">
          Back to traces
        </Button>
      </Link>
    }
  />
);

export const TraceDetailPage = () => {
  const { traceId = "" } = useParams();
  const toast = useToast();
  const query = useTrace(traceId);
  const trace = query.data;
  const [spanParam, setSpanParam] = useQueryState("span", parseAsString.withDefault(""));
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set());
  const [logSpan, setLogSpan] = useState("");

  const tree = useMemo(() => buildTrace(trace?.spans ?? []), [trace?.spans]);
  const window = traceLogWindow(trace?.start_time, trace?.duration_ns);
  const logs = useTraceLogs(traceId, window?.from, window?.to, Boolean(trace));
  const entries = useMemo(() => logs.data?.entries ?? [], [logs.data]);
  const spanLogs = useMemo(() => countBySpan(entries), [entries]);
  const knownSpans = useMemo(() => new Set(tree.nodes.keys()), [tree]);

  const selectedKey = tree.nodes.has(spanParam) ? spanParam : defaultSelectedKey(tree);
  const selected = selectedKey ? tree.nodes.get(selectedKey) : undefined;
  const rows = useMemo(() => visibleRows(tree, collapsed), [tree, collapsed]);
  const failed = useMemo(() => errorKeys(tree), [tree]);

  const select = (key: string) => {
    setCollapsed((current) => revealed(tree, current, key));
    void setSpanParam(key, { history: "replace" });
  };
  const toggle = (key: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  const nextError = () => {
    if (failed.length === 0) return;
    const at = selectedKey ? failed.indexOf(selectedKey) : -1;
    select(failed[(at + 1) % failed.length]);
  };

  if (query.isPending) return <Skeleton className="mt-10 h-72 w-full rounded-xl" />;
  if (query.isError || !trace) {
    return (
      <div className="pt-10">
        {errorStatus(query.error) === 404 ? (
          <NotFound />
        ) : (
          <TraceLoadError
            error={query.error}
            what="this trace"
            onRetry={() => void query.refetch()}
          />
        )}
      </div>
    );
  }

  const title = trace.root_name || "(unnamed trace)";
  const spanCount = trace.span_count ?? tree.order.length;
  const copyId = async () => {
    await navigator.clipboard.writeText(traceId);
    toast.add({ title: "Trace ID copied" });
  };

  return (
    <div className="w-full pb-10">
      <p role="status" className="sr-only">
        {`Trace ${title} loaded: ${spanCount} spans, ${trace.error_count ?? 0} failed`}
      </p>
      <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-3 pt-7">
        <div className="min-w-0">
          <nav aria-label="Breadcrumb" className="text-[13.5px] leading-[1.4] text-quiet">
            <Link to="/logs" className="rounded hover:text-ink">
              Logs
            </Link>{" "}
            /{" "}
            <Link to={TRACES_PATH} className="rounded hover:text-ink">
              Traces
            </Link>{" "}
            / <span aria-current="page">{shortId(traceId, 8)}</span>
          </nav>
          <h1 className="break-words text-[28px] font-medium leading-10 tracking-[-0.025em]">
            {title}
          </h1>
          <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[13.5px] text-quiet">
            {trace.status === "error" ? (
              <Badge tone="down">Error</Badge>
            ) : (
              <Badge tone="up">OK</Badge>
            )}
            <span>{trace.service_name || tree.services[0] || "unknown service"}</span>
            <span aria-hidden>·</span>
            <span>{formatDate(trace.start_time, "MMM d, HH:mm:ss.SSS")} UTC</span>
            <span aria-hidden>·</span>
            <button
              type="button"
              onClick={() => void copyId()}
              title="Copy the trace ID"
              className="rounded font-mono text-[12.5px] hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
            >
              {shortId(traceId, 12)}
            </button>
          </div>
        </div>
        <div className="flex items-center gap-1.5">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              void navigator.clipboard.writeText(globalThis.location.href);
              toast.add({ title: "Link copied" });
            }}
          >
            Copy link
          </Button>
          <Link to={logsHrefForTrace(traceId, trace.start_time, trace.duration_ns)}>
            <Button size="sm">
              View logs{trace.log_count ? ` (${trace.log_count.toLocaleString()})` : ""}
            </Button>
          </Link>
        </div>
      </div>

      <div className="flex flex-col gap-3.5">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatBox label="Duration" value={formatDurationNs(trace.duration_ns)} />
          <StatBox label="Spans" value={spanCount.toLocaleString()} />
          <StatBox
            label="Failed spans"
            value={
              <span className={(trace.error_count ?? 0) > 0 ? "text-(--tone-down-fg)" : undefined}>
                {(trace.error_count ?? 0).toLocaleString()}
              </span>
            }
          />
          <StatBox label="Services" value={tree.services.length.toLocaleString()} />
        </div>
        {trace.has_root === false && (
          <Callout tone="warn" lead="The root span has not arrived.">
            This trace is the {spanCount.toLocaleString()} spans received so far, named after the
            earliest one. It fills in as the rest are exported.
          </Callout>
        )}
        {tree.trueRootCount > 1 && (
          <Callout tone="info" lead={`${tree.trueRootCount} root spans.`}>
            More than one span has no parent. They are listed by start time.
          </Callout>
        )}
        {trace.truncated && (
          <Callout tone="warn" lead="Showing part of this trace.">
            Core returns at most 5,000 spans. {spanCount.toLocaleString()} are stored.
          </Callout>
        )}

        <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_400px]">
          <div className="flex min-w-0 flex-col gap-4">
            <section aria-labelledby="waterfall-title" className="grid gap-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 id="waterfall-title" className="text-sm font-medium">
                  Waterfall
                </h2>
                <div className="flex flex-wrap items-center gap-1.5">
                  {failed.length > 0 && (
                    <Button size="sm" variant="secondary" onClick={nextError}>
                      Next failed span
                    </Button>
                  )}
                  <Button size="sm" variant="secondary" onClick={() => setCollapsed(new Set())}>
                    Expand all
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => setCollapsed(new Set(parentKeys(tree)))}
                  >
                    Collapse all
                  </Button>
                </div>
              </div>
              <RaisedCard>
                {rows.length === 0 ? (
                  <p className="px-3.5 py-6 text-center text-[13px] text-quiet">
                    This trace has no spans.
                  </p>
                ) : (
                  <Waterfall
                    tree={tree}
                    rows={rows}
                    selectedKey={selectedKey}
                    onSelect={select}
                    onToggle={toggle}
                  />
                )}
              </RaisedCard>
            </section>
            <TraceLogsPanel
              traceId={traceId}
              startTime={trace.start_time}
              durationNs={trace.duration_ns}
              logCount={trace.log_count ?? 0}
              entries={entries}
              pending={logs.isPending && Boolean(window)}
              failed={logs.isError}
              spanId={logSpan || undefined}
              spanName={logSpan ? tree.nodes.get(logSpan)?.span.name : undefined}
              knownSpans={knownSpans}
              onClearSpan={() => setLogSpan("")}
              onSelectSpan={select}
              onRetry={() => void logs.refetch()}
            />
          </div>
          <aside
            aria-label="Span details"
            className="min-w-0 rounded-xl border border-hairline bg-field p-3.5 lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:overflow-y-auto"
          >
            <SpanInspector
              key={selectedKey}
              node={selected}
              tree={tree}
              traceId={traceId}
              logCount={selected?.span.span_id ? (spanLogs.get(selected.span.span_id) ?? 0) : 0}
              onSelect={select}
              onShowLogs={setLogSpan}
            />
          </aside>
        </div>
      </div>
    </div>
  );
};
