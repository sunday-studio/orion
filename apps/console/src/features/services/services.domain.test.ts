import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { ServiceServiceSuggestion } from "../../orion-sdk/index.ts";
import {
  appCandidates,
  defaultMonitorChoice,
  defaultSelectedSuggestions,
  describeMove,
  elsewhereNote,
  environmentsOf,
  filterServices,
  healthCounts,
  incidentsForMonitors,
  memberKey,
  monitorCandidates,
  monitorLinkChoices,
  planSuggestion,
  refsForKeys,
  resourceCandidates,
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
  } as unknown as ServiceServiceSuggestion;
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
    const plan = planSuggestion(likely, new Set([memberKey(likely.members![0])]));
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

describe("link candidates", () => {
  it("leaves out system monitors and monitors already in this service", () => {
    const candidates = monitorCandidates(
      [
        { id: "m1", name: "api", type: "http" },
        { id: "m2", name: "sys", system: true },
        { id: "m3", name: "here", service_id: "svc-1" },
        { id: "m4", name: "other", service_id: "svc-2" },
      ] as never,
      "svc-1",
    );
    assert.deepEqual(
      candidates.map((c) => c.label),
      ["api", "other"],
    );
    assert.equal(candidates[1].elsewhere, "svc-2");
  });

  it("warns that linking a thing from another service moves it", () => {
    const [candidate] = appCandidates(
      [{ id: "a1", name: "web", service_id: "svc-2" }] as never,
      "svc-1",
    );
    assert.equal(
      elsewhereNote(candidate, [{ id: "svc-2", name: "billing" }]),
      "Moves from billing",
    );
    assert.equal(elsewhereNote({ ...candidate, elsewhere: undefined }, []), undefined);
  });

  it("keys resources by node and key, and turns ticked keys into refs", () => {
    const candidates = resourceCandidates(
      [
        { key: "container:api", name: "api", kind: "container" },
        { key: "proc:pg", name: "postgres", kind: "process", service_id: "svc-1" },
      ] as never,
      { id: "n1", name: "edge-01" },
      "svc-1",
    );
    assert.equal(candidates.length, 1);
    assert.equal(candidates[0].hint, "container on edge-01");
    assert.deepEqual(refsForKeys(candidates, new Set(["resource:n1:container:api"])), [
      { kind: "resource", node_id: "n1", key: "container:api" },
    ]);
    assert.deepEqual(refsForKeys(candidates, new Set()), []);
  });
});

describe("service incidents", () => {
  it("keeps only incidents on the service's monitors, newest first", () => {
    const incidents = [
      { id: "i1", monitor_id: "m1", opened_at: "2026-11-07T10:00:00Z" },
      { id: "i2", monitor_id: "other", opened_at: "2026-11-07T11:00:00Z" },
      { id: "i3", monitor_id: "m2", opened_at: "2026-11-07T12:00:00Z" },
      { id: "i4", opened_at: "2026-11-07T13:00:00Z" },
    ] as never;
    assert.deepEqual(
      incidentsForMonitors(incidents, ["m1", "m2"]).map((i) => i.id),
      ["i3", "i1"],
    );
    assert.deepEqual(incidentsForMonitors(incidents, []), []);
  });
});

describe("monitor link choices", () => {
  const suggestions = [
    {
      service_id: "svc-1",
      name: "checkout",
      members: [
        {
          kind: "monitor",
          id: "m1",
          confidence: "high",
          reason: "This monitor checks api.example.com",
        },
        { kind: "app", id: "a1", confidence: "high" },
      ],
    },
    {
      name: "web-store",
      slug: "web-store",
      members: [{ kind: "monitor", id: "m1", confidence: "medium" }],
    },
    {
      service_id: "svc-3",
      name: "billing",
      members: [{ kind: "monitor", id: "other", confidence: "high" }],
    },
  ] as never;
  const services = [
    { id: "svc-1", name: "checkout", environment: "production", monitor_count: 1, app_count: 2 },
  ] as never;

  it("offers only groups that name this monitor, each with its reason, then a new service", () => {
    const choices = monitorLinkChoices(suggestions, { id: "m1", name: "api-https" }, services);
    assert.deepEqual(
      choices.map((c) => c.name),
      ["checkout", "web-store", "New service"],
    );
    assert.equal(choices[0].serviceId, "svc-1");
    assert.deepEqual(choices[0].reasons, ["This monitor checks api.example.com"]);
    assert.equal(choices[0].detail, "production · 1 monitor, 2 apps");
    assert.equal(choices[1].serviceId, undefined);
    assert.deepEqual(choices[1].create, {
      name: "web-store",
      slug: "web-store",
      environment: undefined,
    });
    assert.deepEqual(choices[2].create, { name: "api-https" });
  });

  it("starts on a likely suggestion and never on the new-service fallback when there is one", () => {
    const choices = monitorLinkChoices(suggestions, { id: "m1", name: "api-https" }, services);
    assert.equal(defaultMonitorChoice(choices), choices[0].key);
    const onlyMaybe = choices.filter((c) => c.likelihood !== "likely");
    assert.equal(defaultMonitorChoice(onlyMaybe), onlyMaybe[0].key);
    assert.equal(defaultMonitorChoice([]), "");
  });
});
