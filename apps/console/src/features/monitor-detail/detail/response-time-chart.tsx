import { useState } from "react";
import { Box, BoxHeader, Segmented, Skeleton } from "@/design-system";
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
  const timeout = query.data?.timeout_ms;
  const peak = Math.max(0, ...points.map((point) => point.avg_ms ?? 0), timeout ?? 0);
  const top = niceMax(peak || 100);
  const w = 640;
  const h = 150;
  const x = (index: number) => (points.length < 2 ? 0 : (index / (points.length - 1)) * w);
  const y = (ms: number) => h - (Math.min(ms, top) / top) * h;
  const line = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)} ${y(p.avg_ms ?? 0).toFixed(1)}`)
    .join(" ");
  const fmt = (iso?: string) =>
    range === "day" ? formatDate(iso, TIME_FORMAT) : formatDate(iso, "MMM d");
  const ticks = [top, top / 2, 0];

  return (
    <Box>
      <BoxHeader
        title="Response time"
        aside={
          <Segmented
            aria-label="Range"
            size="sm"
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
      ) : points.length < 2 ? (
        <p className="px-4 py-10 text-center text-[13px] text-quiet">
          No response times in this range yet.
        </p>
      ) : (
        <div className="px-4 pb-3 pt-4">
          <div className="relative ml-8">
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
              <path d={`${line} L${w} ${h} L0 ${h} Z`} fill="#2a9f69" opacity="0.12" />
              <path
                d={line}
                fill="none"
                stroke="#2a9f69"
                strokeWidth="1.6"
                strokeLinejoin="round"
                vectorEffect="non-scaling-stroke"
              />
            </svg>
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
            <span>{fmt(points[0]?.at)}</span>
            <span>{fmt(points[Math.floor(points.length / 2)]?.at)}</span>
            <span>Now</span>
          </div>
          <div className="mt-3 flex items-center gap-4 text-xs text-quiet">
            <span className="flex items-center gap-1.5">
              <i className="block size-2 rounded-[2px] bg-[#2a9f69]" /> Response
            </span>
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
