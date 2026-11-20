import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { recentTraces, slowestTraces, traceSummaryView } from "./traces-model.ts";

describe("service trace summary", () => {
  it("formats counts, rate and percentiles", () => {
    const view = traceSummaryView({
      trace_count: 1250,
      error_count: 30,
      error_rate: 0.024,
      rooted_count: 1200,
      p50_ns: 12_000_000,
      p95_ns: 180_000_000,
      p99_ns: 1_400_000_000,
      sampled: true,
    });
    assert.deepEqual(view, {
      traces: "1,250",
      errors: "30",
      errorRate: "2.4%",
      p50: "12ms",
      p95: "180ms",
      p99: "1.4s",
      noLatency: false,
      sampled: true,
      empty: false,
    });
  });

  it("shows dashes, not zeros, when there is nothing to measure", () => {
    const none = traceSummaryView({ trace_count: 0, rooted_count: 0 });
    assert.equal(none.empty, true);
    assert.equal(none.errorRate, "-");
    assert.equal(none.p95, "-");
    assert.equal(traceSummaryView(undefined).traces, "0");
  });

  it("has no latency while no trace has its root", () => {
    const view = traceSummaryView({ trace_count: 5, rooted_count: 0, error_rate: 0, p50_ns: 0 });
    assert.equal(view.noLatency, true);
    assert.equal(view.p50, "-");
    assert.equal(view.errorRate, "0%");
  });
});

describe("trace lists", () => {
  const items = [
    { trace_id: "a", duration_ns: 5 },
    { trace_id: "b", duration_ns: 50 },
    { trace_id: "c" },
    { trace_id: "d", duration_ns: 20 },
  ];

  it("picks the slowest and leaves undated durations last", () => {
    assert.deepEqual(
      slowestTraces(items, 3).map((t) => t.trace_id),
      ["b", "d", "a"],
    );
    assert.deepEqual(
      items.map((t) => t.trace_id),
      ["a", "b", "c", "d"],
    );
  });

  it("keeps the newest in the order given", () => {
    assert.deepEqual(
      recentTraces(items, 2).map((t) => t.trace_id),
      ["a", "b"],
    );
    assert.deepEqual(recentTraces(items, -1), []);
  });
});
