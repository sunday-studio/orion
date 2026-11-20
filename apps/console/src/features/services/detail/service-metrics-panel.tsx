import { useState } from "react";
import { Box, BoxHeader, Button, Skeleton } from "@/design-system";
import { useNow } from "@/features/overview/overview-time";
import { useGetServiceMetrics } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";
import { type MetricChart, appMetricCharts, latestValues } from "../metrics-model";
import { type RangeValue, rangeFromParam } from "../timeline-model";
import { LANE_COLORS, LANE_H, LEFT, Lane, W } from "./timeline-lane";

const TOP = 14;
const HEIGHT = TOP + LANE_H + 22;
const FIRST_CHARTS = 4;

const Legend = ({ chart }: { chart: MetricChart }) =>
  chart.lines.length > 1 ? (
    <ul className="m-0 flex list-none flex-wrap gap-x-3 gap-y-1 p-0 text-xs text-quiet">
      {chart.lines.map((line, index) => (
        <li key={line.id} className="flex items-center gap-1.5">
          <span
            aria-hidden
            className="h-0.5 w-3.5 rounded-full"
            style={{ background: LANE_COLORS[index % LANE_COLORS.length] }}
          />
          {line.label}
        </li>
      ))}
    </ul>
  ) : null;

const ChartCard = ({
  chart,
  from,
  to,
  rangeLabel,
}: {
  chart: MetricChart;
  from: number;
  to: number;
  rangeLabel: string;
}) => {
  const latest = latestValues(chart);
  return (
    <figure className="m-0 grid gap-1 border-t border-line-soft px-3 py-2.5 first:border-t-0">
      <figcaption className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
        <span className="min-w-0">
          <b className="break-words text-[13px] font-medium">{chart.title}</b>
          {chart.subtitle && <span className="ml-2 text-xs text-faint">{chart.subtitle}</span>}
        </span>
        <Legend chart={chart} />
      </figcaption>
      <svg
        viewBox={`0 0 ${W} ${HEIGHT}`}
        role="img"
        aria-label={`${chart.title}, last ${rangeLabel}${latest ? `. Latest ${latest}` : ". No data"}`}
        className="block w-full"
      >
        <Lane
          title=""
          top={TOP}
          lines={chart.lines}
          bars={chart.bars}
          barColor="var(--color-brand-chart)"
          hideLabels
          format={chart.format}
          from={from}
          to={to}
          empty="No data in this range."
        />
        <text x={LEFT} y={HEIGHT - 6} className="fill-faint" fontSize="10">
          {formatDate(new Date(from).toISOString(), TIME_FORMAT)}
        </text>
        <text x={W} y={HEIGHT - 6} textAnchor="end" className="fill-faint" fontSize="10">
          Now
        </text>
      </svg>
      {chart.note && <p className="m-0 text-xs text-quiet">{chart.note}</p>}
    </figure>
  );
};

/** Metrics the linked apps send over OpenTelemetry: levels as lines, counters as bars, latency as percentiles. */
export const ServiceMetricsPanel = ({
  serviceId,
  range: rangeValue,
}: {
  serviceId: string;
  range: RangeValue;
}) => {
  const range = rangeFromParam(rangeValue);
  const now = useNow(30_000);
  const [showAll, setShowAll] = useState(false);
  // Same request as the timeline above, so Core is asked once.
  const metrics = useGetServiceMetrics(serviceId, { from: range.from, max_points: 240 });
  const charts = appMetricCharts(metrics.data?.series ?? [], metrics.data?.step_seconds);
  const shown = showAll ? charts : charts.slice(0, FIRST_CHARTS);

  return (
    <Box>
      <BoxHeader
        title="App metrics"
        aside={charts.length > 0 ? `${charts.length} series` : undefined}
      />
      {metrics.isPending ? (
        <Skeleton className="m-3 h-32" />
      ) : metrics.isError ? (
        <p role="alert" className="px-3.5 py-3 text-[13px] text-(--tone-down-fg)">
          Unable to load metrics.
        </p>
      ) : charts.length === 0 ? (
        <p className="px-3.5 py-3 text-[13px] text-quiet">
          No app metrics yet. Apps linked to this service that send OpenTelemetry metrics show up
          here: gauges as lines, counters as bars and latency histograms as p50, p95 and p99.
        </p>
      ) : (
        <>
          {shown.map((chart) => (
            <ChartCard
              key={chart.id}
              chart={chart}
              from={now - range.ms}
              to={now}
              rangeLabel={range.label}
            />
          ))}
          {charts.length > FIRST_CHARTS && (
            <div className="border-t border-line-soft px-3 py-2">
              <Button
                size="sm"
                variant="secondary"
                aria-expanded={showAll}
                onClick={() => setShowAll((value) => !value)}
              >
                {showAll ? "Show fewer" : `Show all ${charts.length}`}
              </Button>
            </div>
          )}
          {metrics.data?.truncated && (
            <p className="m-0 px-3 pb-2.5 text-xs text-quiet">
              This service has more series than fit here. Showing the most recently seen.
            </p>
          )}
        </>
      )}
    </Box>
  );
};
