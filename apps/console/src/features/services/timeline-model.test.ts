import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  errorRate,
  errorsPerMinute,
  failedChecks,
  formatBytes,
  formatMs,
  hasData,
  linePath,
  metricLines,
  niceMax,
  rangeFromParam,
  responseLine,
  timesNormal,
  totalErrors,
} from "./timeline-model.ts";

describe("timeline model", () => {
  it("defaults the range to four hours", () => {
    assert.equal(rangeFromParam(null).value, "4h");
    assert.equal(rangeFromParam("7d").responseRange, "week");
    assert.equal(rangeFromParam("nope").value, "4h");
  });

  it("counts error and fatal lines per minute and ignores other levels", () => {
    const buckets = [
      { t: "2026-11-07T10:00:00Z", counts: { error: 3, fatal: 1, info: 90 } },
      { t: "2026-11-07T10:05:00Z", counts: { warn: 4 } },
      { t: "not a date", counts: { error: 9 } },
    ];
    assert.deepEqual(
      errorsPerMinute(buckets).map((p) => p.v),
      [4, 0],
    );
    // A 5 minute bucket holding 10 errors is 2 a minute.
    assert.equal(
      errorsPerMinute([{ t: "2026-11-07T10:00:00Z", counts: { error: 10 } }], 300)[0].v,
      2,
    );
    assert.equal(totalErrors(buckets), 13);
  });

  it("keeps only resources that reported the value, and skips OTLP series", () => {
    const series = [
      {
        node_id: "n1",
        key: "api",
        name: "api",
        source: "os",
        points: [{ t: "2026-11-07T10:00:00Z", cpu_percent: 5 }],
      },
      {
        node_id: "n1",
        key: "pg",
        name: "postgres",
        source: "os",
        points: [{ t: "2026-11-07T10:00:00Z" }],
      },
      {
        node_id: "n1",
        key: "m",
        name: "http.latency",
        source: "otlp",
        points: [{ t: "2026-11-07T10:00:00Z", cpu_percent: 1 }],
      },
    ] as never;
    assert.deepEqual(
      metricLines(series, "cpu_percent").map((l) => l.label),
      ["api"],
    );
    assert.deepEqual(metricLines(series, "memory_bytes"), []);
    assert.equal(hasData(metricLines(series, "memory_bytes")), false);
  });

  it("drops response points outside the window or without a value", () => {
    const from = new Date("2026-11-07T10:00:00Z").getTime();
    const points = [
      { at: "2026-11-07T09:00:00Z", avg_ms: 100 },
      { at: "2026-11-07T10:30:00Z", avg_ms: 120 },
      { at: "2026-11-07T10:40:00Z" },
    ];
    assert.deepEqual(
      responseLine(points, from).map((p) => p.v),
      [120],
    );
  });

  it("keeps all-failed buckets off the response line and marks them instead", () => {
    const from = new Date("2026-11-07T10:00:00Z").getTime();
    const points = [
      { at: "2026-11-07T10:05:00Z", avg_ms: 0, checks: 4, failed: 4 },
      { at: "2026-11-07T10:10:00Z", avg_ms: 180, checks: 4, failed: 1 },
    ];
    assert.deepEqual(
      responseLine(points, from).map((p) => p.v),
      [180],
    );
    assert.equal(failedChecks(points, from).length, 1);
  });

  it("rates errors per minute over a window", () => {
    const buckets = [
      { t: "2026-11-08T10:00:00Z", counts: { error: 40, fatal: 2 } },
      { t: "2026-11-08T10:01:00Z", counts: { error: 8, info: 90 } },
    ] as never;
    assert.equal(errorRate(buckets, 5), 10);
    assert.equal(errorRate([], 5), 0);
    assert.equal(errorRate([{ t: "2026-11-08T10:00:00Z", counts: { error: 3 } }] as never, 5), 0.6);
  });

  it("compares the last hour with the usual hour, and stays quiet without a baseline", () => {
    const now = Date.parse("2026-11-08T12:00:00Z");
    const day = (usual: number, recent: number) =>
      [
        { t: "2026-11-08T03:00:00Z", counts: { error: usual } },
        { t: "2026-11-08T11:30:00Z", counts: { error: recent } },
      ] as never;
    assert.equal(timesNormal(day(46, 24), now), 12);
    assert.equal(timesNormal(day(0, 24), now), undefined);
    assert.equal(timesNormal(day(46, 3), now), undefined);
  });

  it("rounds axes and formats units", () => {
    assert.equal(niceMax(0), 1);
    assert.equal(niceMax(48), 50);
    assert.equal(niceMax(101), 200);
    assert.equal(formatBytes(48 * 1024 ** 2), "48 MB");
    assert.equal(formatBytes(1.5 * 1024 ** 3), "1.5 GB");
    assert.equal(formatMs(250.4), "250ms");
    assert.equal(formatMs(1500), "1.5s");
  });

  it("draws a path from plot coordinates", () => {
    assert.equal(
      linePath(
        [
          { t: 0, v: 0 },
          { t: 10, v: 5 },
        ],
        (t) => t * 2,
        (v) => 10 - v,
      ),
      "M0.0 10.0 L20.0 5.0",
    );
  });
});
