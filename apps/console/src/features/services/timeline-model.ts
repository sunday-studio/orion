// Turns the three Core series into chart-ready points. Pure, so it can be tested without a DOM.
import type { ApiServiceLogCountBucket, ApiServiceMetricSeries } from "../../orion-sdk/index.ts";

export const RANGES = [
  { value: "1h", label: "1h", from: "-1h", ms: 3_600_000, responseRange: "day" },
  { value: "4h", label: "4h", from: "-4h", ms: 14_400_000, responseRange: "day" },
  { value: "24h", label: "24h", from: "-24h", ms: 86_400_000, responseRange: "day" },
  { value: "7d", label: "7d", from: "-7d", ms: 604_800_000, responseRange: "week" },
] as const;
export type RangeValue = (typeof RANGES)[number]["value"];

export const rangeFromParam = (value: string | null | undefined) =>
  RANGES.find((range) => range.value === value) ?? RANGES[1];

export type TimePoint = { t: number; v: number };

const time = (iso?: string) => {
  const parsed = iso ? new Date(iso).getTime() : Number.NaN;
  return Number.isFinite(parsed) ? parsed : undefined;
};

/** Error and fatal lines per bucket, scaled to lines per minute. */
export const errorsPerMinute = (
  buckets: readonly ApiServiceLogCountBucket[],
  stepSeconds = 60,
): TimePoint[] =>
  buckets.flatMap((bucket) => {
    const t = time(bucket.t);
    if (t === undefined) return [];
    const errors = (bucket.counts?.error ?? 0) + (bucket.counts?.fatal ?? 0);
    return [{ t, v: (errors * 60) / Math.max(stepSeconds, 1) }];
  });

export const totalErrors = (buckets: readonly ApiServiceLogCountBucket[]) =>
  buckets.reduce(
    (sum, bucket) => sum + (bucket.counts?.error ?? 0) + (bucket.counts?.fatal ?? 0),
    0,
  );

/** Errors per minute across a window of `minutes`, rounded to one decimal under 10 and to a whole number above. */
export const errorRate = (buckets: readonly ApiServiceLogCountBucket[], minutes: number) => {
  const rate = totalErrors(buckets) / Math.max(minutes, 1);
  return rate >= 10 ? Math.round(rate) : Math.round(rate * 10) / 10;
};

/**
 * How the last hour compares with the hourly average before it, from one day of buckets.
 * Undefined when the day before was quiet, since a ratio against nothing says nothing.
 */
export const timesNormal = (
  day: readonly ApiServiceLogCountBucket[],
  now: number,
): number | undefined => {
  const hourAgo = now - 3_600_000;
  const recent = day.filter((bucket) => (time(bucket.t) ?? 0) >= hourAgo);
  const before = day.filter((bucket) => (time(bucket.t) ?? 0) < hourAgo);
  const usual = totalErrors(before) / 23;
  if (usual < 1) return undefined;
  const ratio = totalErrors(recent) / usual;
  return ratio >= 2 ? Math.round(ratio) : undefined;
};

export type Line = { id: string; label: string; points: TimePoint[] };

const operatingSystem = (series: ApiServiceMetricSeries) => series.source !== "otlp";

/** One line per resource that reported the value. Series without any sample are left out. */
export const metricLines = (
  series: readonly ApiServiceMetricSeries[],
  pick: "cpu_percent" | "memory_bytes",
): Line[] =>
  series.filter(operatingSystem).flatMap((item, index) => {
    const points = (item.points ?? []).flatMap((point) => {
      const t = time(point.t);
      const v = point[pick];
      return t !== undefined && typeof v === "number" ? [{ t, v }] : [];
    });
    return points.length === 0
      ? []
      : [
          {
            id: `${item.node_id}:${item.key ?? index}`,
            label: item.name ?? item.key ?? "resource",
            points,
          },
        ];
  });

type ResponsePoint = { at?: string; avg_ms?: number; checks?: number; failed?: number };

/** A bucket where every check failed has no real duration (Core reports 0), so it is not a point on the line. */
const allFailed = (point: ResponsePoint) =>
  (point.failed ?? 0) > 0 && (point.failed ?? 0) >= (point.checks ?? 0);

export const responseLine = (points: readonly ResponsePoint[], from: number): TimePoint[] =>
  points.flatMap((point) => {
    const t = time(point.at);
    return t !== undefined && t >= from && typeof point.avg_ms === "number" && !allFailed(point)
      ? [{ t, v: point.avg_ms }]
      : [];
  });

/** Times of buckets where every check failed, drawn as marks on the response lane. */
export const failedChecks = (points: readonly ResponsePoint[], from: number): number[] =>
  points.flatMap((point) => {
    const t = time(point.at);
    return t !== undefined && t >= from && allFailed(point) ? [t] : [];
  });

const STEPS = [1, 2, 5, 10];

/** Round an axis maximum up to 1, 2, 5 or 10 times a power of ten. */
export const niceMax = (value: number) => {
  if (!(value > 0)) return 1;
  const power = 10 ** Math.floor(Math.log10(value));
  return (STEPS.find((step) => step * power >= value) ?? 10) * power;
};

export const formatBytes = (bytes: number) => {
  if (bytes >= 1024 ** 3) return `${+(bytes / 1024 ** 3).toFixed(1)} GB`;
  if (bytes >= 1024 ** 2) return `${Math.round(bytes / 1024 ** 2)} MB`;
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${Math.round(bytes)} B`;
};

export const formatMs = (ms: number) =>
  ms >= 1000 ? `${+(ms / 1000).toFixed(1)}s` : `${Math.round(ms)}ms`;

/** Straight line segments in plot coordinates. */
export const linePath = (
  points: readonly TimePoint[],
  x: (t: number) => number,
  y: (v: number) => number,
) =>
  points.map((p, i) => `${i === 0 ? "M" : "L"}${x(p.t).toFixed(1)} ${y(p.v).toFixed(1)}`).join(" ");

export const hasData = (lines: readonly { points: readonly unknown[] }[]) =>
  lines.some((line) => line.points.length > 0);
