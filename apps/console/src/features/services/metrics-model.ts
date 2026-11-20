// Charts for the metrics an app sends over OTLP. A gauge is a line of its level, a sum is bars of
// the increase per bucket, a histogram or summary is percentile lines. Pure, no React.
import type { ApiServiceMetricPoint, ApiServiceMetricSeries } from "../../orion-sdk/index.ts";
import { type Line, formatBytes, formatMs } from "./timeline-model.ts";

export type MetricChart = {
  id: string;
  title: string;
  /** Attributes that tell series of one metric apart, and what a bar means. */
  subtitle: string;
  kind: "gauge" | "sum" | "histogram" | "summary";
  lines: Line[];
  /** Bars (sums) rather than lines. */
  bars: boolean;
  format: (value: number) => string;
  /** Said under the chart when the data needs a caveat. */
  note?: string;
};

const time = (iso?: string) => {
  const parsed = iso ? new Date(iso).getTime() : Number.NaN;
  return Number.isFinite(parsed) ? parsed : undefined;
};

const finite = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);

/** App-reported series: the ones the OS chart lanes leave out. */
export const isAppSeries = (series: ApiServiceMetricSeries) => series.source === "otlp";

const compact = (value: number) => {
  const abs = Math.abs(value);
  if (abs >= 1e9) return `${+(value / 1e9).toFixed(1)}B`;
  if (abs >= 1e6) return `${+(value / 1e6).toFixed(1)}M`;
  if (abs >= 1e4) return `${+(value / 1e3).toFixed(1)}k`;
  if (abs >= 100) return String(Math.round(value));
  return String(+value.toFixed(2));
};

const milliseconds = (ms: number) => {
  if (ms >= 1000) return formatMs(ms);
  if (ms >= 10) return `${Math.round(ms)}ms`;
  if (ms >= 1) return `${+ms.toFixed(1)}ms`;
  if (ms === 0) return "0ms";
  return `${Math.round(ms * 1000)}µs`;
};

const BYTES: Record<string, number> = { By: 1, KiBy: 1024, MiBy: 1024 ** 2, GiBy: 1024 ** 3 };

/** A value with its UCUM unit as people read it: 120ms, 1.5 MB, 98%, 12k. */
export const formatMetricValue = (value: number, unit?: string): string => {
  const u = (unit ?? "").trim();
  if (u === "ms") return milliseconds(value);
  if (u === "s") return milliseconds(value * 1000);
  if (u === "us" || u === "µs") return milliseconds(value / 1000);
  if (u === "ns") return milliseconds(value / 1e6);
  if (u in BYTES) return formatBytes(value * BYTES[u]);
  if (u === "%") return `${compact(value)}%`;
  if (u === "" || u === "1") return compact(value);
  // {request} and friends are annotations, not units: read them as the noun.
  const noun = /^\{(.+)\}$/.exec(u)?.[1];
  return `${compact(value)} ${noun ?? u}`;
};

/** The length of one bucket in words: minute, 5 minutes, hour. */
export const stepLabel = (stepSeconds?: number): string => {
  if (!finite(stepSeconds) || stepSeconds <= 0) return "bucket";
  if (stepSeconds < 60) return `${Math.round(stepSeconds)} seconds`;
  const minutes = Math.round(stepSeconds / 60);
  if (minutes === 1) return "minute";
  if (minutes < 60) return `${minutes} minutes`;
  const hours = Math.round(minutes / 60);
  return hours === 1 ? "hour" : `${hours} hours`;
};

type Pick = (point: ApiServiceMetricPoint) => number | undefined;

const lineOf = (
  id: string,
  label: string,
  points: readonly ApiServiceMetricPoint[],
  pick: Pick,
): Line | undefined => {
  const picked = points.flatMap((point) => {
    const t = time(point.t);
    const v = pick(point);
    return t !== undefined && finite(v) ? [{ t, v }] : [];
  });
  return picked.length > 0 ? { id, label, points: picked } : undefined;
};

const compactLines = (lines: (Line | undefined)[]) =>
  lines.filter((line): line is Line => line !== undefined);

/** "p50", "p99.9": the label of a quantile between 0 and 1. */
export const quantileLabel = (quantile: number) => `p${+(quantile * 100).toFixed(2)}`;

const quantileValue = (point: ApiServiceMetricPoint, quantile: number) =>
  point.quantiles?.find((item) => item.quantile === quantile)?.value;

/** p50, p95 and p99 when the series has them; otherwise the quantiles an exporter reported; otherwise the average. */
const percentileLines = (id: string, points: readonly ApiServiceMetricPoint[]): Line[] => {
  const standard = compactLines([
    lineOf(`${id}:p50`, "p50", points, (p) => p.p50),
    lineOf(`${id}:p95`, "p95", points, (p) => p.p95),
    lineOf(`${id}:p99`, "p99", points, (p) => p.p99),
  ]);
  if (standard.length > 0) return standard;
  const reported = [
    ...new Set(
      points.flatMap((p) =>
        (p.quantiles ?? []).flatMap((q) => (finite(q.quantile) ? [q.quantile] : [])),
      ),
    ),
  ]
    .sort((a, b) => a - b)
    .slice(0, 4);
  const fromQuantiles = compactLines(
    reported.map((q) =>
      lineOf(`${id}:q${q}`, quantileLabel(q), points, (p) => quantileValue(p, q)),
    ),
  );
  if (fromQuantiles.length > 0) return fromQuantiles;
  return compactLines([lineOf(`${id}:avg`, "avg", points, (p) => p.avg)]);
};

const kindOf = (series: ApiServiceMetricSeries): MetricChart["kind"] => {
  if (series.kind === "histogram" || series.kind === "summary" || series.kind === "sum") {
    return series.kind;
  }
  if (series.kind === "gauge") return "gauge";
  // A kind this version does not know: judge by what the points carry.
  const points = series.points ?? [];
  if (points.some((p) => finite(p.p50) || finite(p.p95) || finite(p.p99))) return "histogram";
  return "gauge";
};

/**
 * One chart per app-reported series, those with data first. Fields a point lacks are skipped,
 * never read as zero, so an older Core or a sparse series draws gaps instead of false lows.
 */
export const appMetricCharts = (
  series: readonly ApiServiceMetricSeries[],
  stepSeconds?: number,
): MetricChart[] => {
  const charts = series.filter(isAppSeries).map((item, index): MetricChart => {
    const id = `${item.app_id ?? ""}:${item.key ?? item.metric ?? index}`;
    const points = item.points ?? [];
    const kind = kindOf(item);
    const format = (value: number) => formatMetricValue(value, item.unit);
    const title = item.name || item.metric || item.key || "metric";
    const attributes = item.labels ?? "";
    if (kind === "sum") {
      const raw = compactLines([lineOf(id, "increase", points, (p) => p.value)]);
      const decreased = raw.some((line) => line.points.some((p) => p.v < 0));
      const lines = raw.map((line) => ({
        ...line,
        points: line.points.map((p) => ({ ...p, v: Math.max(p.v, 0) })),
      }));
      return {
        id,
        title,
        kind,
        bars: true,
        format,
        lines,
        subtitle: [attributes, `increase per ${stepLabel(stepSeconds)}`]
          .filter(Boolean)
          .join(" · "),
        note: decreased
          ? "This sum also went down in some buckets; those show as zero."
          : undefined,
      };
    }
    if (kind === "histogram" || kind === "summary") {
      const lines = percentileLines(id, points);
      const onlyAverage = lines.length === 1 && lines[0].label === "avg";
      return {
        id,
        title,
        kind,
        bars: false,
        format,
        lines,
        subtitle: attributes,
        note: onlyAverage
          ? "This series sent no buckets or percentiles, so it shows the average."
          : undefined,
      };
    }
    return {
      id,
      title,
      kind,
      bars: false,
      format,
      lines: compactLines([lineOf(id, "average", points, (p) => p.value)]),
      subtitle: attributes,
    };
  });
  // Stable: series with something to draw first, the rest in the order Core gave them.
  return charts
    .map((chart, index) => ({ chart, index }))
    .sort(
      (a, b) =>
        Number(b.chart.lines.length > 0) - Number(a.chart.lines.length > 0) || a.index - b.index,
    )
    .map(({ chart }) => chart);
};

/** The newest value of every line, as one sentence for a screen reader: "p50 20ms, p95 90ms". */
export const latestValues = (chart: MetricChart): string =>
  chart.lines
    .flatMap((line) => {
      const last = line.points.at(-1);
      return last ? [`${line.label} ${chart.format(last.v)}`] : [];
    })
    .join(", ");
