import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { acceptPlan, runLinks } from "./link-runner.ts";

describe("runLinks", () => {
  it("keeps going after a failure and reports each one", async () => {
    const seen: string[] = [];
    const outcome = await runLinks(
      [
        { kind: "app", id: "a1" },
        { kind: "monitor", id: "bad" },
        { kind: "monitor", id: "m2" },
      ],
      async (ref) => {
        seen.push(ref.id ?? "");
        if (ref.id === "bad") throw new Error("system monitors cannot be linked");
        return { link: { id: ref.id, kind: ref.kind } };
      },
    );
    assert.deepEqual(seen, ["a1", "bad", "m2"]);
    assert.equal(outcome.linked.length, 2);
    assert.deepEqual(outcome.failed, [
      { ref: { kind: "monitor", id: "bad" }, message: "system monitors cannot be linked" },
    ]);
  });
});

describe("acceptPlan", () => {
  it("creates the service first, then links to the new id", async () => {
    const calls: string[] = [];
    const result = await acceptPlan(
      { create: { name: "checkout" }, refs: [{ kind: "app", id: "a1" }] },
      {
        create: async (input) => {
          calls.push(`create ${input.name}`);
          return { service: { id: "svc-9", name: "checkout" } };
        },
        link: async (serviceId, ref) => {
          calls.push(`link ${serviceId} ${ref.id}`);
          return { link: { id: ref.id } };
        },
      },
    );
    assert.deepEqual(calls, ["create checkout", "link svc-9 a1"]);
    assert.equal(result.serviceId, "svc-9");
    assert.equal(result.error, undefined);
  });

  it("joins an existing service without creating one", async () => {
    let created = false;
    const result = await acceptPlan(
      { serviceId: "svc-1", refs: [{ kind: "monitor", id: "m1" }] },
      {
        create: async () => {
          created = true;
          return {};
        },
        link: async () => ({ link: { id: "m1" } }),
      },
    );
    assert.equal(created, false);
    assert.equal(result.outcome.linked.length, 1);
  });

  it("links nothing when the service cannot be created", async () => {
    let linked = 0;
    const result = await acceptPlan(
      { create: { name: "checkout" }, refs: [{ kind: "app", id: "a1" }] },
      {
        create: async () => {
          throw new Error("slug already exists");
        },
        link: async () => {
          linked += 1;
          return {};
        },
      },
    );
    assert.equal(linked, 0);
    assert.equal(result.error, "slug already exists");
  });
});
