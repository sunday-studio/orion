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
  formatBytes,
  formatMs,
  hasData,
  linePath,
  metricLines,
  niceMax,
  rangeFromParam,
  responseLine,
} from "../timeline-model";

const W = 900;
const LEFT = 40;
const LANE_H = 64;
const GAP = 26;
const COLORS = ["#3f3f46", "#2a9f69", "#6366f1", "#f59e0b", "#a1a1aa"];

type LaneProps = {
  title: string;
  top: number;
  lines: Line[];
  bars?: boolean;
  format: (value: number) => string;
  from: number;
  to: number;
  /** What to say when this lane has nothing to draw. */
  empty: string;
  max?: number;
};

const Lane = ({ title, top, lines, bars, format, from, to, empty, max }: LaneProps) => {
  const x = (t: number) => LEFT + ((t - from) / Math.max(to - from, 1)) * (W - LEFT);
  const peak = niceMax(max ?? Math.max(0, ...lines.flatMap((l) => l.points.map((p) => p.v))));
  const y = (v: number) => top + LANE_H - (Math.min(v, peak) / peak) * LANE_H;
  return (
    <g>
      <text x={LEFT} y={top - 8} className="fill-quiet" fontSize="11">
        {title}
        {lines.length > 1 && !bars ? ` · ${lines.map((l) => l.label).join(", ")}` : ""}
        {lines.length === 1 && !bars ? ` · ${lines[0]?.label}` : ""}
      </text>
      <line x1={LEFT} x2={W} y1={top + LANE_H} y2={top + LANE_H} className="stroke-hairline" />
      {hasData(lines) ? (
        <>
          <line
            x1={LEFT}
            x2={W}
            y1={top}
            y2={top}
            className="stroke-hairline"
            strokeDasharray="3 4"
          />
          <text x={LEFT - 6} y={top + 4} textAnchor="end" className="fill-faint" fontSize="10">
            {format(peak)}
          </text>
          <text
            x={LEFT - 6}
            y={top + LANE_H + 3}
            textAnchor="end"
            className="fill-faint"
            fontSize="10"
          >
            0
          </text>
          {lines.map((line, index) =>
            bars ? (
              <g key={line.id} fill="#e5484d" opacity=".85">
                {line.points.map((p) => (
                  <rect
                    key={p.t}
                    x={x(p.t) - 2}
                    y={y(p.v)}
                    width={4}
                    height={Math.max(top + LANE_H - y(p.v), p.v > 0 ? 1 : 0)}
                  >
                    <title>{`${formatDate(new Date(p.t).toISOString(), TIME_FORMAT)}: ${format(p.v)}`}</title>
                  </rect>
                ))}
              </g>
            ) : (
              <path
                key={line.id}
                d={linePath(line.points, x, y)}
                fill="none"
                stroke={COLORS[index % COLORS.length]}
                strokeWidth="2"
                strokeLinejoin="round"
                strokeDasharray={title.startsWith("Memory") ? "5 3" : undefined}
              />
            ),
          )}
        </>
      ) : (
        <text x={LEFT + 8} y={top + LANE_H / 2 + 4} className="fill-faint" fontSize="12">
          {empty}
        </text>
      )}
    </g>
  );
};

const worstMonitor = (monitors: readonly ServiceServiceLinkedMonitor[]) =>
  monitors.find((m) => m.health === "down") ??
  monitors.find((m) => m.health === "degraded") ??
  monitors[0];

/** Response time, errors, CPU and memory for one service on a shared time axis. Every lane is real Core data or says why it is empty. */
export const ServiceTimeline = ({
  serviceId,
  monitors,
  incidentOpenedAt,
}: {
  serviceId: string;
  monitors: readonly ServiceServiceLinkedMonitor[];
  incidentOpenedAt?: string;
}) => {
  const [rangeValue, setRangeValue] = useState<RangeValue>("4h");
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
  const errorLines: Line[] = [
    { id: "errors", label: "errors", points: errorsPerMinute(counts.data?.buckets ?? [], step) },
  ];
  const cpu = metricLines(metrics.data?.series ?? [], "cpu_percent");
  const memory = metricLines(metrics.data?.series ?? [], "memory_bytes");

  const lane = (n: number) => 18 + n * (LANE_H + GAP);
  const height = lane(4) - GAP + 24;
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
              <p role="alert" className="pb-2 text-[12.5px] text-[#be123c]">
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
                    fill="#e5484d"
                    opacity=".08"
                  />
                  <line
                    x1={openedX}
                    x2={openedX}
                    y1={10}
                    y2={height - 24}
                    stroke="#e5484d"
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
                empty={
                  monitor
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
                title="CPU"
                top={lane(2)}
                lines={cpu}
                format={(v) => `${v}%`}
                from={from}
                to={now}
                empty={
                  noMetrics
                    ? "History is not collected yet. It appears once a linked resource has reported."
                    : "No CPU history for the linked resources."
                }
              />
              <Lane
                title="Memory"
                top={lane(3)}
                lines={memory}
                format={formatBytes}
                from={from}
                to={now}
                empty={
                  noMetrics
                    ? "History is not collected yet. It appears once a linked resource has reported."
                    : "No memory history for the linked resources."
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
