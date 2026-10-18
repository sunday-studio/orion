import { type PointerEvent, useRef, useState } from "react";
import { ChartTooltip, Skeleton } from "@/design-system";
import type { ApiLogHistogramResponse } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { formatDate } from "@/utils/date";

const ORDER = ["trace", "debug", "info", "warn", "error", "fatal"];
const COLORS: Record<string, string> = {
  fatal: "#e5484d",
  error: "#e5484d",
  warn: "#f5a524",
  info: "#a9d9bf",
  debug: "#a9d9bf",
  trace: "#a9d9bf",
};

/** Volume over the window, stacked by severity. Drag across bars to zoom into that span. */
export const VolumeChart = ({
  title,
  data,
  pending,
  summary,
  onZoom,
}: {
  title: string;
  data?: ApiLogHistogramResponse;
  pending: boolean;
  summary?: string;
  onZoom: (from: string, to: string) => void;
}) => {
  const buckets = data?.buckets ?? [];
  const width = useRef<HTMLDivElement>(null);
  const [hover, setHover] = useState<number | null>(null);
  const [drag, setDrag] = useState<{ start: number; end: number } | null>(null);
  const max = Math.max(1, ...buckets.map((bucket) => bucket.total ?? 0));
  const step = (data?.bucket_seconds ?? 0) * 1000;

  const indexAt = (event: PointerEvent) => {
    const box = width.current?.getBoundingClientRect();
    if (!box || box.width === 0 || buckets.length === 0) return 0;
    const ratio = Math.min(Math.max((event.clientX - box.left) / box.width, 0), 0.9999);
    return Math.floor(ratio * buckets.length);
  };
  const finish = () => {
    if (drag && buckets.length > 0) {
      const [a, b] = [Math.min(drag.start, drag.end), Math.max(drag.start, drag.end)];
      if (b > a) {
        const from = buckets[a].start ?? "";
        const last = new Date(buckets[b].start ?? 0).getTime() + step;
        onZoom(from, new Date(last).toISOString());
      }
    }
    setDrag(null);
  };

  const errorCount = buckets.reduce(
    (sum, bucket) => sum + (bucket.severity?.error ?? 0) + (bucket.severity?.fatal ?? 0),
    0,
  );
  const active = hover === null ? undefined : buckets[hover];
  const x = hover === null ? 0 : ((hover + 0.5) / Math.max(buckets.length, 1)) * 100;
  const labels = [0, 0.25, 0.5, 0.75].map((r) => buckets[Math.floor(r * buckets.length)]?.start);

  return (
    <section className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
      <div className="flex items-center justify-between border-b border-hairline bg-field px-3.5 py-2 text-[13px]">
        <h2 className="font-medium">{title}</h2>
        <span className="text-faint">
          {summary ?? `${errorCount.toLocaleString()} errors`} · drag to zoom
        </span>
      </div>
      {pending ? (
        <Skeleton className="m-4 h-[140px]" />
      ) : buckets.length === 0 ? (
        <p className="px-4 py-10 text-center text-[13px] text-quiet">No data in this range.</p>
      ) : (
        <div className="px-3.5 pb-2.5 pt-4">
          <div
            ref={width}
            className="relative flex h-[120px] cursor-crosshair touch-none items-end gap-[2px]"
            onPointerDown={(event) => {
              event.currentTarget.setPointerCapture(event.pointerId);
              const index = indexAt(event);
              setDrag({ start: index, end: index });
            }}
            onPointerMove={(event) => {
              const index = indexAt(event);
              setHover(index);
              if (drag) setDrag({ ...drag, end: index });
            }}
            onPointerUp={finish}
            onPointerCancel={() => setDrag(null)}
            onPointerLeave={() => !drag && setHover(null)}
          >
            {buckets.map((bucket, index) => {
              const total = Math.max(bucket.total ?? 0, 1);
              const parts = Object.entries(bucket.severity ?? {}).sort(
                ([a], [b]) => ORDER.indexOf(a) - ORDER.indexOf(b),
              );
              return (
                <div
                  key={bucket.start}
                  className={cn(
                    "flex min-w-0 flex-1 flex-col-reverse overflow-hidden rounded-[2px] transition-opacity",
                    hover !== null && hover !== index && "opacity-70",
                  )}
                  style={{ height: `${Math.max(((bucket.total ?? 0) / max) * 100, 1.5)}%` }}
                >
                  {parts.map(([severity, count]) => (
                    <i
                      key={severity}
                      style={{
                        height: `${(count / total) * 100}%`,
                        background: COLORS[severity] ?? COLORS.info,
                      }}
                    />
                  ))}
                </div>
              );
            })}
            {drag && (
              <div
                aria-hidden
                className="pointer-events-none absolute inset-y-0 rounded border border-brand bg-brand/10"
                style={{
                  left: `${(Math.min(drag.start, drag.end) / buckets.length) * 100}%`,
                  width: `${((Math.abs(drag.end - drag.start) + 1) / buckets.length) * 100}%`,
                }}
              />
            )}
            {active && !drag && (
              <ChartTooltip x={x} className="!top-auto bottom-[calc(100%-8px)]">
                <div className="font-medium">
                  {formatDate(active.start, "HH:mm")}
                  {step
                    ? ` to ${formatDate(new Date(new Date(active.start ?? 0).getTime() + step), "HH:mm")}`
                    : ""}
                </div>
                <div>{(active.total ?? 0).toLocaleString()} lines</div>
                {(active.severity?.error ?? 0) + (active.severity?.fatal ?? 0) > 0 && (
                  <div className="text-[#ff8a95]">
                    {(active.severity?.error ?? 0) + (active.severity?.fatal ?? 0)} errors
                  </div>
                )}
              </ChartTooltip>
            )}
          </div>
          <div className="mt-2 flex justify-between text-xs text-faint">
            {labels.map((label, index) => (
              <span key={index}>{label ? formatDate(label, "MMM d HH:mm") : ""}</span>
            ))}
            <span>Now</span>
          </div>
        </div>
      )}
    </section>
  );
};
