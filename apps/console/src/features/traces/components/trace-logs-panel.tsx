import { Link } from "react-router-dom";
import { Button, RaisedCard, Skeleton } from "@/design-system";
import { LevelBadge } from "@/features/logs/components/log-states";
import type { ApiLogEntry } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { formatDate } from "@/utils/date";
import { logsHrefForTrace } from "../trace-links";
import { logsForPanel } from "../trace-logs";

export const TraceLogsPanel = ({
  traceId,
  startTime,
  durationNs,
  logCount,
  entries,
  pending,
  failed,
  spanId,
  spanName,
  knownSpans,
  onClearSpan,
  onSelectSpan,
  onRetry,
}: {
  traceId: string;
  startTime?: string;
  durationNs?: number;
  /** Lines Core counted for the trace id (may exceed what is loaded). */
  logCount: number;
  entries: readonly ApiLogEntry[];
  pending: boolean;
  failed: boolean;
  spanId?: string;
  spanName?: string;
  knownSpans: ReadonlySet<string>;
  onClearSpan: () => void;
  onSelectSpan: (spanId: string) => void;
  onRetry: () => void;
}) => {
  const shown = logsForPanel(entries, spanId);
  const logsHref = logsHrefForTrace(traceId, startTime, durationNs);
  return (
    <section aria-labelledby="trace-logs-title" className="grid gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="trace-logs-title" className="text-sm font-medium">
          Logs
          <span className="ml-1.5 font-normal text-faint">{logCount.toLocaleString()}</span>
        </h2>
        <Link
          to={logsHref}
          className="rounded text-[13px] underline focus-visible:ring-2 focus-visible:ring-focus"
        >
          Open all in Logs
        </Link>
      </div>
      {spanId && (
        <p className="flex flex-wrap items-center gap-2 text-[12.5px] text-quiet">
          Only lines on <b className="font-medium text-ink">{spanName || spanId}</b>
          <Button size="sm" variant="ghost" onClick={onClearSpan}>
            Show all lines
          </Button>
        </p>
      )}
      {pending ? (
        <Skeleton className="h-24 w-full rounded-[10px]" />
      ) : failed ? (
        <RaisedCard className="flex items-center justify-between gap-3 px-3.5 py-3 text-[13px]">
          <span role="alert" className="text-(--tone-down-fg)">
            Unable to load the log lines.
          </span>
          <Button size="sm" variant="secondary" onClick={onRetry}>
            Retry
          </Button>
        </RaisedCard>
      ) : shown.length === 0 ? (
        <RaisedCard className="px-3.5 py-3 text-[13px] text-quiet">
          {spanId
            ? "No log line names this span."
            : logCount > 0
              ? "The lines of this trace are outside the loaded window. Open them in Logs."
              : "No log line carries this trace id. Apps that add the trace id to their logs link them here."}
        </RaisedCard>
      ) : (
        <RaisedCard>
          <ul className="m-0 max-h-80 list-none overflow-auto p-0">
            {shown.map((entry) => {
              const linked = entry.span_id ? knownSpans.has(entry.span_id) : false;
              return (
                <li
                  key={entry.id}
                  className="grid grid-cols-[88px_72px_minmax(0,1fr)_auto] items-center gap-2.5 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0"
                >
                  <span className="tabular-nums text-faint">
                    {formatDate(entry.timestamp, "HH:mm:ss.SSS")}
                  </span>
                  <LevelBadge level={entry.severity} />
                  <span className="min-w-0 truncate">
                    <b className="font-medium">{entry.event ?? entry.body}</b>
                    {entry.event && entry.body && (
                      <span className="ml-1.5 text-faint">{entry.body}</span>
                    )}
                  </span>
                  <button
                    type="button"
                    disabled={!linked}
                    onClick={() => entry.span_id && onSelectSpan(entry.span_id)}
                    className={cn(
                      "rounded text-xs focus-visible:ring-2 focus-visible:ring-focus",
                      linked ? "underline" : "invisible",
                    )}
                    aria-label="Select the span of this log line"
                  >
                    span
                  </button>
                </li>
              );
            })}
          </ul>
        </RaisedCard>
      )}
    </section>
  );
};
