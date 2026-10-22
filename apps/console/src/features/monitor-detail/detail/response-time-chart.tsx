import { useState } from "react";
import {
  Box,
  BoxHeader,
  ChartMarker,
  ChartTooltip,
  Segmented,
  Skeleton,
  usePlotHover,
} from "@/design-system";
import { useGetMonitorResponseTimes } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";

const RANGES = [
  { value: "day", label: "Day" },
  { value: "week", label: "Week" },
  { value: "month", label: "Month" },
] as const;

type Range = (typeof RANGES)[number]["value"];

const niceMax = (value: number) => {
  const steps = [100, 250, 500, 1000, 2000, 5000, 10000, 30000, 60000];
  return steps.find((step) => step >= value) ?? Math.ceil(value / 1000) * 1000;
};

const tick = (ms: number) => (ms >= 1000 ? `${ms / 1000}s` : `${ms}`);

/** Response time over the chosen range with the timeout as a dashed line (canvas monitor-detail). */
export const ResponseTimeChart = ({ monitorId }: { monitorId: string }) => {
  const [range, setRange] = useState<Range>("day");
  const query = useGetMonitorResponseTimes(monitorId, { range });
  const points = (query.data?.points ?? []).filter((point) => typeof point.avg_ms === "number");
  const all = query.data?.points ?? [];
  // A failed check has no duration, so a bucket where every check failed is marked instead of plotted.
  const failures = all.filter((point) => point.failed && typeof point.avg_ms !== "number");
  const times = all.map((point) => new Date(point.at ?? 0).getTime());
  const first = Math.min(...times);
  const last = Math.max(...times);
  const timeX = (at?: string) =>
    last === first ? 0 : ((new Date(at ?? 0).getTime() - first) / (last - first)) * 640;
  const timeout = query.data?.timeout_ms;
  const peak = Math.max(0, ...points.map((point) => point.avg_ms ?? 0), timeout ?? 0);
  const top = niceMax(peak || 100);
  const w = 640;
  const h = 150;
  const y = (ms: number) => h - (Math.min(ms, top) / top) * h;
  const line = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${timeX(p.at).toFixed(1)} ${y(p.avg_ms ?? 0).toFixed(1)}`)
    .join(" ");
  const fmt = (iso?: string) =>
    range === "day" ? formatDate(iso, TIME_FORMAT) : formatDate(iso, "MMM d");
  const ticks = [top, top / 2, 0];
  const hover = usePlotHover(points.length);
  const active = hover.index === null ? undefined : points[hover.index];
  const activeX = active ? (timeX(active.at) / w) * 100 : 0;
  const activeY = active ? (y(active.avg_ms ?? 0) / h) * 100 : 0;

  return (
    <Box>
      <BoxHeader
        title="Response time"
        aside={
          <Segmented
            aria-label="Range"
            size="sm"
            surface="field"
            options={RANGES}
            value={range}
            onValueChange={(next) => setRange(next as Range)}
          />
        }
      />
      {query.isPending ? (
        <Skeleton className="m-4 h-40" />
      ) : query.isError ? (
        <p className="px-4 py-10 text-center text-[13px] text-quiet">
          Unable to load response times. Core may need updating.
        </p>
      ) : points.length < 2 && failures.length === 0 ? (
        <p className="px-4 py-10 text-center text-[13px] text-quiet">
          No response times in this range yet.
        </p>
      ) : (
        <div className="px-4 pb-3 pt-4">
          <div className="relative ml-8">
            <div className="absolute inset-0 z-[1]" {...hover.bind} />
            <svg
              viewBox={`0 0 ${w} ${h}`}
              preserveAspectRatio="none"
              role="img"
              aria-label="Response time"
              className="block h-[150px] w-full overflow-visible"
            >
              {ticks.map((value) => (
                <line
                  key={value}
                  x1="0"
                  x2={w}
                  y1={y(value)}
                  y2={y(value)}
                  stroke="var(--color-line-soft)"
                  strokeDasharray="4 4"
                  vectorEffect="non-scaling-stroke"
                />
              ))}
              {timeout ? (
                <line
                  x1="0"
                  x2={w}
                  y1={y(timeout)}
                  y2={y(timeout)}
                  stroke="#e5484d"
                  strokeDasharray="5 4"
                  vectorEffect="non-scaling-stroke"
                />
              ) : null}
              {points.length >= 2 && (
                <path
                  d={`${line} L${timeX(points.at(-1)?.at)} ${h} L${timeX(points[0]?.at)} ${h} Z`}
                  fill="#2a9f69"
                  opacity="0.12"
                />
              )}
              {failures.map((point) => (
                <line
                  key={point.at}
                  x1={timeX(point.at)}
                  x2={timeX(point.at)}
                  y1={h - 14}
                  y2={h}
                  stroke="#e5484d"
                  strokeWidth="3"
                  vectorEffect="non-scaling-stroke"
                />
              ))}
              <path
                d={line}
                fill="none"
                stroke="#2a9f69"
                strokeWidth="1.6"
                strokeLinejoin="round"
                vectorEffect="non-scaling-stroke"
              />
            </svg>
            {active && (
              <>
                <ChartMarker x={activeX} y={activeY} />
                <ChartTooltip x={activeX}>
                  <div className="text-[11px] opacity-70">
                    {formatDate(active.at, "MMM d, HH:mm")}
                  </div>
                  <div className="font-medium">
                    {Math.round(active.avg_ms ?? 0).toLocaleString()} ms avg
                  </div>
                  {typeof active.max_ms === "number" && (
                    <div className="opacity-70">
                      max {Math.round(active.max_ms).toLocaleString()} ms
                    </div>
                  )}
                  {active.failed ? (
                    <div className="text-[#ff8a95]">
                      {active.failed} failed of {active.checks}
                    </div>
                  ) : null}
                </ChartTooltip>
              </>
            )}
            {ticks.map((value) => (
              <span
                key={value}
                className="absolute -left-8 w-6 -translate-y-1/2 text-right font-mono text-[11px] text-faint"
                style={{ top: `${(y(value) / h) * 100}%` }}
              >
                {tick(value)}
              </span>
            ))}
          </div>
          <div className="ml-8 mt-1.5 flex justify-between font-mono text-[11px] text-faint">
            <span>{fmt(all[0]?.at)}</span>
            <span>{fmt(all[Math.floor(all.length / 2)]?.at)}</span>
            <span>Now</span>
          </div>
          <div className="mt-3 flex items-center gap-4 text-xs text-quiet">
            <span className="flex items-center gap-1.5">
              <i className="block size-2 rounded-[2px] bg-[#2a9f69]" /> Response
            </span>
            {failures.length > 0 && (
              <span className="flex items-center gap-1.5 text-[#be123c]">
                <i className="block h-3 w-0.5 bg-[#e5484d]" /> Failed checks
              </span>
            )}
            {timeout ? (
              <span className="flex items-center gap-1.5 text-[#be123c]">
                - - Timeout {tick(timeout)}
              </span>
            ) : null}
          </div>
        </div>
      )}
    </Box>
  );
};
