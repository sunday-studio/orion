import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  axisTicks,
  formatDurationNs,
  formatOffsetNs,
  formatPercent,
  minDurationLabel,
  normalizeMinDuration,
  parseDurationMs,
  shortId,
} from "./trace-format.ts";

describe("duration format", () => {
  it("picks the unit a person would", () => {
    assert.equal(formatDurationNs(0), "0ns");
    assert.equal(formatDurationNs(850), "850ns");
    assert.equal(formatDurationNs(1_500), "1.5µs");
    assert.equal(formatDurationNs(250_000), "250µs");
    assert.equal(formatDurationNs(2_500_000), "2.5ms");
    assert.equal(formatDurationNs(120_000_000), "120ms");
    assert.equal(formatDurationNs(1_250_000_000), "1.3s");
    assert.equal(formatDurationNs(12_340_000_000), "12s");
  });

  it("moves up a unit instead of printing 1000 of the smaller one", () => {
    assert.equal(formatDurationNs(999_600), "1ms");
    assert.equal(formatDurationNs(999_600_000), "1s");
  });

  it("reads minutes and hours", () => {
    assert.equal(formatDurationNs(60e9), "1m");
    assert.equal(formatDurationNs(185e9), "3m 5s");
    assert.equal(formatDurationNs(3600e9), "1h");
    assert.equal(formatDurationNs(3720e9), "1h 2m");
  });

  it("says dash for anything that is not a duration", () => {
    for (const bad of [undefined, null, Number.NaN, Number.POSITIVE_INFINITY, -1]) {
      assert.equal(formatDurationNs(bad as number), "-");
    }
  });

  it("signs offsets", () => {
    assert.equal(formatOffsetNs(12_000_000), "+12ms");
    assert.equal(formatOffsetNs(-3_000_000), "-3ms");
    assert.equal(formatOffsetNs(undefined), "-");
  });
});

describe("axis ticks", () => {
  it("runs from zero to the whole trace", () => {
    const ticks = axisTicks(100_000_000);
    assert.deepEqual(
      ticks.map((tick) => tick.fraction),
      [0, 0.25, 0.5, 0.75, 1],
    );
    assert.deepEqual(
      ticks.map((tick) => tick.label),
      ["0ns", "25ms", "50ms", "75ms", "100ms"],
    );
  });

  it("copes with an empty trace", () => {
    assert.deepEqual(
      axisTicks(0, 2).map((tick) => tick.label),
      ["0ns", "0ns", "0ns"],
    );
    assert.equal(axisTicks(Number.NaN, 0).length, 2);
  });
});

describe("percent and ids", () => {
  it("formats a rate", () => {
    assert.equal(formatPercent(0), "0%");
    assert.equal(formatPercent(0.0123), "1.2%");
    assert.equal(formatPercent(0.12), "12%");
    assert.equal(formatPercent(1), "100%");
    assert.equal(formatPercent(1.4), "100%");
    assert.equal(formatPercent(undefined), "-");
  });

  it("shortens an id", () => {
    assert.equal(shortId("0123456789abcdef"), "01234567");
    assert.equal(shortId("abc"), "abc");
    assert.equal(shortId(undefined), "-");
  });
});

describe("minimum duration input", () => {
  it("reads units the way Core does", () => {
    assert.equal(parseDurationMs("250ms"), 250);
    assert.equal(parseDurationMs("2s"), 2000);
    assert.equal(parseDurationMs("1.5s"), 1500);
    assert.equal(parseDurationMs("500"), 500);
    assert.equal(parseDurationMs("2 m"), 120_000);
    assert.equal(parseDurationMs("fast"), undefined);
    assert.equal(parseDurationMs("-5ms"), undefined);
    assert.equal(parseDurationMs(""), undefined);
  });

  it("keeps only values Core accepts", () => {
    assert.equal(normalizeMinDuration(" 250 ms "), "250ms");
    assert.equal(normalizeMinDuration("soon"), "");
    assert.equal(normalizeMinDuration(undefined), "");
    assert.equal(minDurationLabel("2s"), "≥ 2s");
  });
});
