import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  defaultSelectedSuggestions,
  describeMove,
  environmentsOf,
  filterServices,
  healthCounts,
  memberKey,
  planSuggestion,
  serviceHref,
  serviceTabFromParam,
  suggestionLikelihood,
  unlinkParams,
} from "./services.domain.ts";

const service = (name: string, health: string, environment = "production") =>
  ({
    id: name,
    name,
    slug: name,
    health,
    environment,
  }) as never;

describe("service tab route param", () => {
  it("accepts the record tabs", () => {
    for (const tab of ["overview", "monitors", "logs", "resources", "incidents", "settings"])
      assert.equal(serviceTabFromParam(tab), tab);
  });
  it("falls back to overview for missing or unknown values", () => {
    assert.equal(serviceTabFromParam(null), "overview");
    assert.equal(serviceTabFromParam("bogus"), "overview");
  });
  it("builds hrefs, leaving the default tab out and escaping the id", () => {
    assert.equal(serviceHref("abc"), "/services/abc");
    assert.equal(serviceHref("abc", "logs"), "/services/abc?tab=logs");
    assert.equal(serviceHref("a/b", "overview"), "/services/a%2Fb");
  });
});

describe("service list", () => {
  const list = [
    service("billing", "up"),
    service("checkout", "down"),
    service("nightly-jobs", "degraded", "staging"),
    service("storefront", "up"),
    service("quiet", ""),
  ];
  it("counts by health", () => {
    assert.deepEqual(healthCounts(list), { all: 5, down: 1, degraded: 1, up: 2, unknown: 1 });
  });
  it("sorts the worst first, then by name", () => {
    assert.deepEqual(
      filterServices(list, {}).map((s) => s.name),
      ["checkout", "nightly-jobs", "quiet", "billing", "storefront"],
    );
  });
  it("filters by health, environment and text", () => {
    assert.deepEqual(
      filterServices(list, { health: "up" }).map((s) => s.name),
      ["billing", "storefront"],
    );
    assert.deepEqual(
      filterServices(list, { environment: "staging" }).map((s) => s.name),
      ["nightly-jobs"],
    );
    assert.deepEqual(
      filterServices(list, { query: " CHECK " }).map((s) => s.name),
      ["checkout"],
    );
    assert.deepEqual(environmentsOf(list), ["production", "staging"]);
  });
});

const member = (kind: string, id: string, confidence: string, extra = {}) =>
  ({ kind, id, confidence, name: id, ...extra }) as never;

describe("suggestions", () => {
  const likely = {
    name: "checkout",
    slug: "checkout",
    environment: "production",
    members: [member("app", "a1", "high"), member("monitor", "m1", "high")],
  } as never;
  const maybe = {
    name: "nightly",
    slug: "nightly",
    members: [member("app", "a2", "high"), member("monitor", "m2", "medium")],
  } as never;
  const joinExisting = {
    service_id: "svc-1",
    name: "billing",
    members: [member("resource", "", "high", { node_id: "n1", key: "container:pg" })],
  } as never;

  it("calls a group likely only when every member is a high-confidence match", () => {
    assert.equal(suggestionLikelihood(likely), "likely");
    assert.equal(suggestionLikelihood(maybe), "maybe");
    assert.equal(suggestionLikelihood({ members: [] } as never), "maybe");
  });

  it("ticks only likely groups by default, never all of them", () => {
    assert.deepEqual(defaultSelectedSuggestions([likely, maybe, joinExisting]), [
      "new:checkout",
      "svc-1:billing",
    ]);
  });

  it("plans a new service for a group with no service_id", () => {
    const plan = planSuggestion(likely);
    assert.deepEqual(plan.create, {
      name: "checkout",
      slug: "checkout",
      environment: "production",
    });
    assert.equal(plan.serviceId, undefined);
    assert.deepEqual(plan.refs, [
      { kind: "app", id: "a1" },
      { kind: "monitor", id: "m1" },
    ]);
  });

  it("plans a join for a group that names an existing service and links resources by node and key", () => {
    const plan = planSuggestion(joinExisting);
    assert.equal(plan.create, undefined);
    assert.equal(plan.serviceId, "svc-1");
    assert.deepEqual(plan.refs, [{ kind: "resource", node_id: "n1", key: "container:pg" }]);
  });

  it("leaves out members the person unticked", () => {
    const plan = planSuggestion(likely, new Set([memberKey(likely.members[0])]));
    assert.deepEqual(plan.refs, [{ kind: "monitor", id: "m1" }]);
  });
});

describe("unlink", () => {
  it("maps a ref onto the query parameters Core expects", () => {
    assert.deepEqual(unlinkParams({ kind: "app", id: "a1" }), { kind: "app", target_id: "a1" });
    assert.deepEqual(unlinkParams({ kind: "resource", node_id: "n1", key: "k" }), {
      kind: "resource",
      node_id: "n1",
      key: "k",
    });
  });
  it("says where a moved link came from", () => {
    assert.equal(describeMove(undefined, []), undefined);
    assert.equal(describeMove("s1", [{ id: "s1", name: "billing" }]), "Moved from billing.");
    assert.equal(describeMove("gone", []), "Moved from another service.");
  });
});
