import { Link } from "react-router-dom";
import { Badge, Button, Callout, CodeBlock, RaisedCard, useToast } from "@/design-system";
import { formatDate } from "@/utils/date";
import {
  type AttrRow,
  attributeRows,
  eventOffsetInSpan,
  exceptionOf,
  kindLabel,
  serviceLabel,
  sortedEvents,
} from "../trace-attrs";
import { formatDurationNs, formatOffsetNs, shortId } from "../trace-format";
import { attributeFilterHref, traceHref } from "../trace-links";
import { type SpanNode, type TraceTree, skewMessage } from "../trace-tree";

const Meta = ({
  label,
  children,
  action,
}: {
  label: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}) => (
  <div className="grid min-h-[38px] grid-cols-[96px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-1 text-[13px] first:border-t-0">
    <span className="text-quiet">{label}</span>
    <span className="min-w-0 break-words">{children}</span>
    {action}
  </div>
);

const Attributes = ({ rows, linkable }: { rows: AttrRow[]; linkable: boolean }) =>
  rows.length === 0 ? (
    <p className="px-3.5 py-3 text-[13px] text-quiet">None.</p>
  ) : (
    <dl className="m-0">
      {rows.map((row) => (
        <div
          key={row.key}
          className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)_auto] items-baseline gap-3 border-t border-line-soft px-3.5 py-1.5 text-[12.5px] first:border-t-0"
        >
          <dt className="break-words font-mono text-quiet">{row.key}</dt>
          <dd className="m-0 break-words font-mono">{row.text}</dd>
          {linkable && row.filterable ? (
            <Link
              to={attributeFilterHref(row.key, row.text)}
              aria-label={`Find traces where ${row.key} is ${row.text}`}
              title="Find traces with this value"
              className="rounded text-xs text-quiet underline hover:text-ink"
            >
              find
            </Link>
          ) : (
            <span />
          )}
        </div>
      ))}
    </dl>
  );

const Section = ({
  title,
  count,
  children,
}: {
  title: string;
  count?: number;
  children: React.ReactNode;
}) => (
  <section className="grid gap-1.5">
    <h3 className="text-[13px] font-medium">
      {title}
      {count !== undefined && <span className="ml-1.5 font-normal text-faint">{count}</span>}
    </h3>
    <RaisedCard>{children}</RaisedCard>
  </section>
);

export const SpanInspector = ({
  node,
  tree,
  traceId,
  logCount,
  onSelect,
  onShowLogs,
}: {
  node?: SpanNode;
  tree: TraceTree;
  traceId: string;
  /** Log lines on this span among those loaded for the trace. */
  logCount: number;
  onSelect: (key: string) => void;
  onShowLogs: (spanId: string) => void;
}) => {
  const toast = useToast();
  if (!node) {
    return (
      <p className="text-[13px] text-quiet">
        Select a span to see its attributes, events and links.
      </p>
    );
  }
  const span = node.span;
  const parent = node.parentKey ? tree.nodes.get(node.parentKey) : undefined;
  const skew = skewMessage(node, formatDurationNs);
  const exception = exceptionOf(span);
  const events = sortedEvents(span.events);
  const links = span.links ?? [];
  const resource = attributeRows(span.resource);
  const copy = async (text: string, name: string) => {
    await navigator.clipboard.writeText(text);
    toast.add({ title: `${name} copied` });
  };
  const status = span.status === "error" ? "error" : span.status === "ok" ? "ok" : "unset";
  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <h2 className="break-words text-[15px] font-medium leading-snug">
          {span.name || "(unnamed)"}
        </h2>
        <div className="flex flex-wrap items-center gap-1.5">
          {status === "error" ? (
            <Badge tone="down">Error</Badge>
          ) : status === "ok" ? (
            <Badge tone="up">OK</Badge>
          ) : (
            <Badge tone="unknown">Status not set</Badge>
          )}
          {kindLabel(span.kind) && <Badge tone="info">{kindLabel(span.kind)}</Badge>}
          {node.orphan && <Badge tone="degraded">Parent missing</Badge>}
        </div>
      </div>
      {(span.status_message || exception) && (
        <Callout tone="down" lead={exception?.type ?? "Failed."}>
          {span.status_message || exception?.message}
        </Callout>
      )}
      {skew && <Callout tone="warn">{skew}</Callout>}
      <RaisedCard>
        <Meta label="Service">{serviceLabel(span)}</Meta>
        <Meta label="Duration">
          {formatDurationNs(node.durationNs)}
          <span className="ml-2 text-faint">self {formatDurationNs(node.selfNs)}</span>
        </Meta>
        <Meta label="Starts">
          {formatOffsetNs(node.startNs - tree.startNs)} into the trace
          <span className="block text-xs text-faint">
            {formatDate(span.start_time, "MMM d, HH:mm:ss.SSS")} UTC
          </span>
        </Meta>
        <Meta
          label="Span ID"
          action={
            <Button
              size="sm"
              variant="secondary"
              onClick={() => void copy(span.span_id ?? "", "Span ID")}
            >
              Copy
            </Button>
          }
        >
          <code className="font-mono text-[12.5px]">{span.span_id || "-"}</code>
        </Meta>
        <Meta
          label="Parent"
          action={
            parent ? (
              <Button size="sm" variant="secondary" onClick={() => onSelect(parent.key)}>
                Go to
              </Button>
            ) : undefined
          }
        >
          {parent ? (
            <span className="truncate">{parent.span.name}</span>
          ) : span.parent_span_id ? (
            <span className="text-quiet">
              <code className="font-mono text-[12.5px]">{shortId(span.parent_span_id)}</code> is not
              in this trace
            </span>
          ) : (
            <span className="text-quiet">None, this is a root</span>
          )}
        </Meta>
        {node.descendants > 0 && (
          <Meta label="Below">
            {node.descendants.toLocaleString()} {node.descendants === 1 ? "span" : "spans"}
            {node.subtreeErrors > 0 && (
              <span className="ml-2 text-(--tone-down-fg)">{node.subtreeErrors} failed</span>
            )}
          </Meta>
        )}
      </RaisedCard>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          size="sm"
          variant="secondary"
          disabled={logCount === 0}
          onClick={() => onShowLogs(span.span_id ?? "")}
        >
          {logCount === 0
            ? "No log lines on this span"
            : `Show ${logCount} log ${logCount === 1 ? "line" : "lines"} on this span`}
        </Button>
      </div>
      {exception?.stack && (
        <Section title="Stack trace">
          <CodeBlock className="max-h-64 overflow-auto whitespace-pre-wrap break-words">
            {exception.stack}
          </CodeBlock>
        </Section>
      )}
      <Section title="Attributes" count={Object.keys(span.attributes ?? {}).length}>
        <Attributes rows={attributeRows(span.attributes)} linkable />
      </Section>
      <Section title="Resource" count={resource.length}>
        <Attributes rows={resource} linkable />
      </Section>
      {events.length > 0 && (
        <Section title="Events" count={events.length}>
          {events.map((event, index) => {
            const at = eventOffsetInSpan(event, node.startNs);
            const rows = attributeRows(event.attributes);
            return (
              <div
                key={`${event.name}-${index}`}
                className="border-t border-line-soft first:border-t-0"
              >
                <div className="flex items-baseline justify-between gap-3 px-3.5 py-2 text-[13px]">
                  <b className="break-words font-medium">{event.name || "(unnamed event)"}</b>
                  <span className="shrink-0 tabular-nums text-faint">
                    {at === undefined ? "-" : `+${formatDurationNs(at)}`}
                  </span>
                </div>
                {rows.length > 0 && <Attributes rows={rows} linkable={false} />}
              </div>
            );
          })}
        </Section>
      )}
      {links.length > 0 && (
        <Section title="Links" count={links.length}>
          {links.map((link, index) => (
            <div
              key={`${link.trace_id}-${link.span_id}-${index}`}
              className="flex items-center justify-between gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0"
            >
              <span className="min-w-0 truncate font-mono text-[12.5px]">
                {link.trace_id === traceId ? "this trace" : shortId(link.trace_id, 12)} ·{" "}
                {shortId(link.span_id)}
              </span>
              {link.trace_id && link.trace_id !== traceId ? (
                <Link
                  to={traceHref(link.trace_id, link.span_id)}
                  className="shrink-0 rounded text-xs underline"
                >
                  Open trace
                </Link>
              ) : link.span_id && tree.nodes.has(link.span_id) ? (
                <button
                  type="button"
                  className="shrink-0 rounded text-xs underline focus-visible:ring-2 focus-visible:ring-focus"
                  onClick={() => onSelect(link.span_id ?? "")}
                >
                  Go to span
                </button>
              ) : null}
            </div>
          ))}
        </Section>
      )}
    </div>
  );
};
