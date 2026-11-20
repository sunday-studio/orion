import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { countBySpan, logsForPanel } from "./trace-logs.ts";

const lines = [
  { id: "3", timestamp: "2026-11-20T10:00:03Z", span_id: "b" },
  { id: "1", timestamp: "2026-11-20T10:00:01Z", span_id: "a" },
  { id: "2", timestamp: "2026-11-20T10:00:02Z" },
  { id: "4", timestamp: "2026-11-20T10:00:04Z", span_id: "a" },
  { id: "0" },
];

describe("trace log lines", () => {
  it("lists lines oldest first, undated ones at the top, without touching the input", () => {
    const copy = [...lines];
    assert.deepEqual(
      logsForPanel(lines).map((l) => l.id),
      ["0", "1", "2", "3", "4"],
    );
    assert.deepEqual(lines, copy);
  });

  it("narrows to one span", () => {
    assert.deepEqual(
      logsForPanel(lines, "a").map((l) => l.id),
      ["1", "4"],
    );
    assert.deepEqual(logsForPanel(lines, "nope"), []);
  });

  it("counts lines per span and skips lines without one", () => {
    assert.deepEqual([...countBySpan(lines)].sort(), [
      ["a", 2],
      ["b", 1],
    ]);
    assert.equal(countBySpan([]).size, 0);
  });
});
