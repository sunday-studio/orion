import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { logRangeFromParam, mergeLogs, openInLogsHref, severityParam } from "./logs-model.ts";

describe("service logs", () => {
  it("merges apps newest first, drops repeats and caps the list", () => {
    const a = [
      { id: "1", timestamp: "2026-11-07T10:00:00Z", app_id: "a" },
      { id: "3", timestamp: "2026-11-07T10:02:00Z", app_id: "a" },
    ];
    const b = [
      { id: "2", timestamp: "2026-11-07T10:01:00Z", app_id: "b" },
      { id: "3", timestamp: "2026-11-07T10:02:00Z", app_id: "a" },
    ];
    assert.deepEqual(
      mergeLogs([a, b]).map((e) => e.id),
      ["3", "2", "1"],
    );
    assert.deepEqual(
      mergeLogs([a, b], 2).map((e) => e.id),
      ["3", "2"],
    );
    assert.deepEqual(mergeLogs([]), []);
  });

  it("sends a minimum severity only for warn and error", () => {
    assert.equal(severityParam("all"), undefined);
    assert.equal(severityParam("warn"), ">=warn");
    assert.equal(severityParam("error"), ">=error");
    assert.equal(severityParam("bogus"), undefined);
  });

  it("falls back to two hours and links to the Logs screen with every app", () => {
    assert.equal(logRangeFromParam("nope"), "2h");
    assert.equal(logRangeFromParam("24h"), "24h");
    assert.equal(openInLogsHref(["a", "b"], "2h"), "/logs?range=2h&source=a%2Cb");
    assert.equal(openInLogsHref([], "1h"), "/logs?range=1h");
  });
});
