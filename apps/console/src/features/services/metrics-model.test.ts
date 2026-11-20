import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { ApiServiceMetricSeries } from "../../orion-sdk/index.ts";
import {
  appMetricCharts,
  formatMetricValue,
  isAppSeries,
  labelText,
  latestValues,
  quantileLabel,
  stepLabel,
} from "./metrics-model.ts";

const at = (minute: number) => `2026-11-20T10:${String(minute).padStart(2, "0")}:00Z`;
const T = (minute: number) => new Date(at(minute)).getTime();

const series = (patch: Partial<ApiServiceMetricSeries>): ApiServiceMetricSeries => ({
  app_id: "app1",
  key: "k",
  source: "otlp",
  name: "http.server.duration",
  unit: "ms",
  ...patch,
});

describe("histogram series", () => {
  it("draws p50, p95 and p99 lines from the points that have them", () => {
    const [chart] = appMetricCharts([
      series({
        kind: "histogram",
        points: [
          { t: at(0), count: 10, p50: 20, p95: 90, p99: 140 },
          { t: at(1), count: 4, p50: 25, p95: 95 },
          { t: at(2) },
        ],
      }),
    ]);
    assert.equal(chart.kind, "histogram");
    assert.equal(chart.bars, false);
    assert.deepEqual(
      chart.lines.map((l) => [l.label, l.points.length]),
      [
        ["p50", 2],
        ["p95", 2],
        ["p99", 1],
      ],
    );
    assert.deepEqual(chart.lines[0].points, [
      { t: T(0), v: 20 },
      { t: T(1), v: 25 },
    ]);
  });

  it("falls back to the average when no percentile exists, and says so", () => {
    const [chart] = appMetricCharts([
      series({ kind: "histogram", points: [{ t: at(0), avg: 12, count: 3 }] }),
    ]);
    assert.deepEqual(
      chart.lines.map((l) => l.label),
      ["avg"],
    );
    assert.match(chart.note ?? "", /average/);
  });

  it("gives an empty chart, not a broken one, for points with only counts", () => {
    const [chart] = appMetricCharts([
      series({ kind: "histogram", points: [{ t: at(0), count: 5 }] }),
    ]);
    assert.deepEqual(chart.lines, []);
  });

  it("uses the quantiles a summary reported when it has no p50, p95, p99", () => {
    const [chart] = appMetricCharts([
      series({
        kind: "summary",
        points: [
          {
            t: at(0),
            quantiles: [
              { quantile: 0.9, value: 30 },
              { quantile: 0.5, value: 10 },
            ],
          },
        ],
      }),
    ]);
    assert.deepEqual(
      chart.lines.map((l) => l.label),
      ["p50", "p90"],
    );
  });
});

describe("sum series", () => {
  it("shows the increase per bucket as bars and says what a bar covers", () => {
    const [chart] = appMetricCharts(
      [
        series({
          kind: "sum",
          unit: "{request}",
          labels: "route=/pay",
          points: [
            { t: at(0), value: 12 },
            { t: at(5), value: 7, value_max: 3 },
          ],
        }),
      ],
      300,
    );
    assert.equal(chart.bars, true);
    assert.equal(chart.subtitle, "route=/pay · increase per 5 minutes");
    assert.deepEqual(chart.lines[0].points, [
      { t: T(0), v: 12 },
      { t: T(5), v: 7 },
    ]);
    assert.equal(chart.format(1500), "1500 request");
  });

  it("draws a decrease as zero and notes it", () => {
    const [chart] = appMetricCharts([
      series({
        kind: "sum",
        points: [
          { t: at(0), value: -4 },
          { t: at(1), value: 2 },
        ],
      }),
    ]);
    assert.deepEqual(
      chart.lines[0].points.map((p) => p.v),
      [0, 2],
    );
    assert.match(chart.note ?? "", /went down/);
  });
});

describe("gauge series and mixed responses", () => {
  it("draws the average of a gauge", () => {
    const [chart] = appMetricCharts([
      series({ kind: "gauge", unit: "By", points: [{ t: at(0), value: 2048, value_max: 4096 }] }),
    ]);
    assert.equal(chart.lines[0].points[0].v, 2048);
    assert.equal(chart.format(2048), "2 KB");
  });

  it("treats an unknown kind by what its points carry", () => {
    const [histogramLike] = appMetricCharts([
      series({ kind: "exponential", points: [{ t: at(0), p50: 3 }] }),
    ]);
    assert.equal(histogramLike.kind, "histogram");
    const [gaugeLike] = appMetricCharts([
      series({ kind: undefined, points: [{ t: at(0), value: 1 }] }),
    ]);
    assert.equal(gaugeLike.kind, "gauge");
  });

  it("leaves out measured resources and lists charts with data first", () => {
    const charts = appMetricCharts([
      series({ key: "empty", kind: "gauge", points: [] }),
      series({ key: "os", source: "os", kind: "process", points: [{ t: at(0), cpu_percent: 5 }] }),
      series({ key: "full", kind: "gauge", points: [{ t: at(0), value: 1 }] }),
      series({ key: "nopoints", kind: "gauge", points: undefined }),
    ]);
    assert.deepEqual(
      charts.map((c) => c.id),
      ["app1:full", "app1:empty", "app1:nopoints"],
    );
    assert.equal(isAppSeries(series({ source: "os" })), false);
  });

  it("ignores points with a bad time or value", () => {
    const [chart] = appMetricCharts([
      series({
        kind: "gauge",
        points: [
          { t: "garbage", value: 1 },
          { t: at(0), value: Number.NaN },
          { t: at(1), value: 3 },
        ],
      }),
    ]);
    assert.deepEqual(chart.lines[0].points, [{ t: T(1), v: 3 }]);
  });
});

describe("value and step labels", () => {
  it("formats by unit", () => {
    assert.equal(formatMetricValue(0.5, "ms"), "500µs");
    assert.equal(formatMetricValue(120, "ms"), "120ms");
    assert.equal(formatMetricValue(1500, "ms"), "1.5s");
    assert.equal(formatMetricValue(0.25, "s"), "250ms");
    assert.equal(formatMetricValue(97.456, "%"), "97.46%");
    assert.equal(formatMetricValue(12_500, "1"), "12.5k");
    assert.equal(formatMetricValue(3, "{error}"), "3 error");
    assert.equal(formatMetricValue(3, undefined), "3");
    assert.equal(formatMetricValue(2, "MiBy"), "2 MB");
    assert.equal(formatMetricValue(7, "Cel"), "7 Cel");
  });

  it("names the length of a bucket", () => {
    assert.equal(stepLabel(60), "minute");
    assert.equal(stepLabel(300), "5 minutes");
    assert.equal(stepLabel(3600), "hour");
    assert.equal(stepLabel(7200), "2 hours");
    assert.equal(stepLabel(30), "30 seconds");
    assert.equal(stepLabel(undefined), "bucket");
    assert.equal(quantileLabel(0.999), "p99.9");
  });
});

describe("latest values", () => {
  it("reads the newest point of each line", () => {
    const [chart] = appMetricCharts([
      series({
        kind: "histogram",
        points: [
          { t: at(0), p50: 10, p95: 50 },
          { t: at(1), p50: 20, p95: 90 },
        ],
      }),
    ]);
    assert.equal(latestValues(chart), "p50 20ms, p95 90ms");
  });

  it("is empty for a chart without data", () => {
    const [chart] = appMetricCharts([series({ kind: "gauge", points: [] })]);
    assert.equal(latestValues(chart), "");
  });
});

describe("series labels", () => {
  it("reads the JSON Core sends and drops the service name", () => {
    assert.equal(
      labelText(
        '{"http.route":"/checkout","resource.service.instance.id":"pod-1","resource.service.name":"checkout-api"}',
      ),
      "http.route=/checkout instance=pod-1",
    );
    assert.equal(labelText('{"resource.service.name":"checkout-api"}'), "");
    assert.equal(labelText('{"code":500,"ok":true}'), "code=500 ok=true");
  });

  it("keeps text that is not a JSON object as it is", () => {
    assert.equal(labelText("route=/pay"), "route=/pay");
    assert.equal(labelText("[1,2]"), "[1,2]");
    assert.equal(labelText(undefined), "");
    assert.equal(labelText(""), "");
  });
});
