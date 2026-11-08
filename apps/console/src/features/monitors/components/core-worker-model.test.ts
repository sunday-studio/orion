import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { describeCoreWorkerWarning, shouldWarnForCoreWorker } from "./core-worker-model.ts";

describe("core worker warning", () => {
  it("says nothing before any worker has registered", () => {
    assert.equal(shouldWarnForCoreWorker(undefined), false);
    assert.equal(
      shouldWarnForCoreWorker({ worker_count: 0, online_count: 0, status: "unknown" }),
      false,
    );
  });

  it("says nothing when the workers are healthy", () => {
    assert.equal(
      shouldWarnForCoreWorker({
        worker_count: 1,
        online_count: 1,
        stale_count: 0,
        status: "healthy",
      }),
      false,
    );
  });

  it("still warns when the only worker went quiet", () => {
    const dead = { worker_count: 1, online_count: 0, stale_count: 1, status: "degraded" };
    assert.equal(shouldWarnForCoreWorker(dead), true);
    assert.match(describeCoreWorkerWarning(dead)?.lead ?? "", /stale/);
  });

  it("warns about a degraded worker that is still online", () => {
    const degraded = { worker_count: 1, online_count: 1, stale_count: 0, status: "degraded" };
    assert.match(describeCoreWorkerWarning(degraded)?.lead ?? "", /degraded/);
  });
});
