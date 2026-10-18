import { ChartMarker, ChartTooltip, usePlotHover } from "@/design-system";
import type { ApiAgentReportResponse } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";

/** CPU usage over the loaded reports, oldest left (canvas "CPU, last hour"). */
export const CpuChart = ({ reports }: { reports: ApiAgentReportResponse[] }) => {
  const points = [...reports]
    .map((report) => ({
      at: report.timestamp ?? report.created_at,
      value: report.cpu?.usage_percent,
    }))
    .filter(
      (point): point is { at: string; value: number } =>
        Boolean(point.at) && typeof point.value === "number",
    )
    .sort((a, b) => new Date(a.at).getTime() - new Date(b.at).getTime());
  const hover = usePlotHover(points.length);
  if (points.length < 2) {
    return (
      <p className="px-4 py-8 text-center text-[13px] text-quiet">No CPU reports in this window.</p>
    );
  }
  const w = 640;
  const h = 120;
  const x = (index: number) => (index / (points.length - 1)) * w;
  const y = (value: number) => h - (Math.min(Math.max(value, 0), 100) / 100) * h;
  const line = points
    .map((p, i) => `${i === 0 ? "M" : "L"}${x(i).toFixed(1)} ${y(p.value).toFixed(1)}`)
    .join(" ");
  const area = `${line} L${w} ${h} L0 ${h} Z`;
  const last = points[points.length - 1];
  const mid = points[Math.floor(points.length / 2)];
  const active = hover.index === null ? undefined : points[hover.index];
  const activeX = hover.index === null ? 0 : (hover.index / (points.length - 1)) * 100;
  const activeY = active ? (y(active.value) / h) * 100 : 0;
  return (
    <div className="relative px-4 pb-3 pt-3">
      <div className="relative ml-9">
        <div className="absolute inset-0 z-[1]" {...hover.bind} />
        <svg
          viewBox={`0 0 ${w} ${h}`}
          preserveAspectRatio="none"
          role="img"
          aria-label="CPU usage"
          className="block h-[120px] w-full overflow-visible"
        >
          {[0, 50, 100].map((tick) => (
            <line
              key={tick}
              x1="0"
              x2={w}
              y1={y(tick)}
              y2={y(tick)}
              stroke="var(--color-line-soft)"
              strokeDasharray="4 4"
              vectorEffect="non-scaling-stroke"
            />
          ))}
          <path d={area} fill="#2a9f69" opacity="0.12" />
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
              <div className="text-[11px] opacity-70">{formatDate(active.at, "HH:mm:ss")}</div>
              <div className="font-medium">CPU {Math.round(active.value)}%</div>
            </ChartTooltip>
          </>
        )}
        {[100, 50, 0].map((tick) => (
          <span
            key={tick}
            className="absolute -left-9 w-7 -translate-y-1/2 text-right font-mono text-[11px] text-faint"
            style={{ top: `${100 - tick}%` }}
          >
            {tick === 100 ? "100%" : tick}
          </span>
        ))}
        <span
          aria-hidden
          className="absolute size-2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-[#2a9f69]"
          style={{ left: "100%", top: `${(y(last.value) / h) * 100}%` }}
        />
      </div>
      <div className="ml-9 mt-1.5 flex justify-between font-mono text-[11px] text-faint">
        <span>{formatDate(points[0].at, TIME_FORMAT)}</span>
        <span>{formatDate(mid.at, TIME_FORMAT)}</span>
        <span>{formatDate(last.at, TIME_FORMAT)}</span>
      </div>
    </div>
  );
};
