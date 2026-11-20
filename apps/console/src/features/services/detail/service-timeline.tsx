import { useState } from "react";
import { Box, BoxHeader, Segmented, Skeleton } from "@/design-system";
import { useNow } from "@/features/overview/overview-time";
import {
  type ServiceServiceLinkedMonitor,
  useGetMonitorResponseTimes,
  useGetServiceLogCounts,
  useGetServiceMetrics,
} from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import {
  type Line,
  RANGES,
  type RangeValue,
  errorsPerMinute,
  failedChecks,
  formatBytes,
  formatMs,
  hasData,
  metricLines,
  rangeFromParam,
  responseLine,
} from "../timeline-model";
import { GAP, LANE_H, LEFT, Lane, W } from "./timeline-lane";

const worstMonitor = (monitors: readonly ServiceServiceLinkedMonitor[]) =>
  monitors.find((m) => m.health === "down") ??
  monitors.find((m) => m.health === "degraded") ??
  monitors[0];

/** Response time, errors, CPU and memory for one service on a shared time axis. Every lane is real Core data or says why it is empty. */
export const ServiceTimeline = ({
  serviceId,
  monitors,
  incidentOpenedAt,
  range: controlledRange,
  onRangeChange,
}: {
  serviceId: string;
  monitors: readonly ServiceServiceLinkedMonitor[];
  incidentOpenedAt?: string;
  /** The range of the whole page, when the page shares one with other panels. Without it the timeline keeps its own. */
  range?: RangeValue;
  onRangeChange?: (range: RangeValue) => void;
}) => {
  const [ownRange, setOwnRange] = useState<RangeValue>("4h");
  const rangeValue = controlledRange ?? ownRange;
  const setRangeValue = onRangeChange ?? setOwnRange;
  const range = rangeFromParam(rangeValue);
  const now = useNow(30_000);
  const from = now - range.ms;
  const step = range.ms <= 14_400_000 ? 60 : range.ms <= 86_400_000 ? 300 : 1800;
  const metrics = useGetServiceMetrics(serviceId, { from: range.from, max_points: 240 });
  const counts = useGetServiceLogCounts(serviceId, { from: range.from, step_seconds: step });
  const monitor = worstMonitor(monitors);
  const response = useGetMonitorResponseTimes(
    monitor?.id ?? "",
    { range: range.responseRange },
    { query: { enabled: Boolean(monitor?.id) } },
  );

  const responseLines: Line[] = [
    {
      id: "response",
      label: monitor?.name ?? "",
      points: responseLine(response.data?.points ?? [], from),
    },
  ];
  const failedAt = failedChecks(response.data?.points ?? [], from);
  const errorLines: Line[] = [
    { id: "errors", label: "errors", points: errorsPerMinute(counts.data?.buckets ?? [], step) },
  ];
  const cpu = metricLines(metrics.data?.series ?? [], "cpu_percent");
  const memory = metricLines(metrics.data?.series ?? [], "memory_bytes");

  const lane = (n: number) => 18 + n * (LANE_H + GAP);
  const height = lane(3) - GAP + 24;
  const opened = incidentOpenedAt ? new Date(incidentOpenedAt).getTime() : undefined;
  const openedX =
    opened !== undefined && opened >= from && opened <= now
      ? LEFT + ((opened - from) / (now - from)) * (W - LEFT)
      : undefined;
  const loading = metrics.isPending || counts.isPending;
  const failed = metrics.isError || counts.isError;
  const noMetrics = !hasData(cpu) && !hasData(memory);

  return (
    <Box>
      <BoxHeader
        title={`Last ${range.label}`}
        aside={
          <Segmented
            aria-label="Range"
            size="sm"
            surface="field"
            options={RANGES.map((r) => ({ value: r.value, label: r.label }))}
            value={rangeValue}
            onValueChange={(next) => setRangeValue(next as RangeValue)}
          />
        }
      />
      <div className="px-3 py-2.5">
        {loading ? (
          <Skeleton className="h-64 w-full" />
        ) : (
          <>
            {failed && (
              <p role="alert" className="pb-2 text-[12.5px] text-(--tone-down-fg)">
                Some history could not be loaded. Lanes without data say so.
              </p>
            )}
            <svg
              viewBox={`0 0 ${W} ${height}`}
              role="img"
              aria-label={`History for the last ${range.label}: response time, errors per minute, CPU and memory.`}
              className="block w-full"
            >
              {openedX !== undefined && (
                <g>
                  <rect
                    x={openedX}
                    y={10}
                    width={W - openedX}
                    height={height - 34}
                    fill="var(--color-dot-down)"
                    opacity=".08"
                  />
                  <line
                    x1={openedX}
                    x2={openedX}
                    y1={10}
                    y2={height - 24}
                    stroke="var(--color-dot-down)"
                    strokeDasharray="4 3"
                  />
                  <text x={openedX - 5} y={8} textAnchor="end" className="fill-quiet" fontSize="10">
                    {formatDate(incidentOpenedAt, TIME_FORMAT)} incident opened
                  </text>
                </g>
              )}
              <Lane
                title="Response time"
                top={lane(0)}
                lines={responseLines}
                format={formatMs}
                from={from}
                to={now}
                marks={failedAt}
                empty={
                  failedAt.length > 0
                    ? "Every check in this range failed, so there is no response time."
                    : monitor
                      ? "No checks in this range yet."
                      : "Link a monitor to chart response time."
                }
              />
              <Lane
                title="Errors per minute"
                top={lane(1)}
                lines={errorLines}
                bars
                format={(v) => String(Math.round(v * 10) / 10)}
                from={from}
                to={now}
                empty="No error logs in this range, or no app is linked."
              />
              <Lane
                title="CPU and memory"
                top={lane(2)}
                lines={cpu}
                secondary={memory}
                format={(v) => `${v}%`}
                secondaryFormat={formatBytes}
                from={from}
                to={now}
                max={100}
                empty={
                  noMetrics
                    ? "History is not collected yet. It appears once a linked resource has reported."
                    : "No CPU or memory history for the linked resources."
                }
              />
              <text x={LEFT} y={height - 6} className="fill-faint" fontSize="10">
                {formatDate(new Date(from).toISOString(), TIME_FORMAT)}
              </text>
              <text x={W} y={height - 6} textAnchor="end" className="fill-faint" fontSize="10">
                Now
              </text>
            </svg>
            {metrics.data?.truncated && (
              <p className="pt-1 text-xs text-quiet">
                This service has more series than fit here. Showing the first 100.
              </p>
            )}
          </>
        )}
      </div>
    </Box>
  );
};
