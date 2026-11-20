import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  EMPTY_TRACE_FILTERS,
  type TraceFilters,
  durationShare,
  filterTokens,
  hasTraceFilters,
  normalizeStatus,
  parseAttr,
  slowestDuration,
  statusCounts,
  traceQueryParams,
  traceRangeLabel,
  traceWindow,
} from "./traces-state.ts";
import {
  attributeFilterHref,
  logsHrefForTrace,
  traceHref,
  traceLogWindow,
  tracesHref,
} from "./trace-links.ts";

const filters = (patch: Partial<TraceFilters> = {}): TraceFilters => ({
  ...EMPTY_TRACE_FILTERS,
  ...patch,
});

describe("trace query parameters", () => {
  it("sends only the last hour when nothing is filtered", () => {
    assert.deepEqual(traceQueryParams(filters()), {
      from: "-1h",
      to: undefined,
      app_id: undefined,
      service_id: undefined,
      service: undefined,
      status: undefined,
      min_duration: undefined,
      name: undefined,
      attr: undefined,
    });
  });

  it("maps every filter to its Core name", () => {
    const params = traceQueryParams(
      filters({
        app: "a1,a2",
        service: "checkout",
        serviceId: "svc_1",
        status: "error",
        minDuration: " 250 ms",
        q: "  POST /pay ",
        range: "24h",
        attr: "http.status_code:500",
      }),
    );
    assert.equal(params.app_id, "a1,a2");
    assert.equal(params.service, "checkout");
    assert.equal(params.service_id, "svc_1");
    assert.equal(params.status, "error");
    assert.equal(params.min_duration, "250ms");
    assert.equal(params.name, "POST /pay");
    assert.equal(params.from, "-24h");
    assert.equal(params.attr, "http.status_code:500");
  });

  it("drops values Core would reject", () => {
    const params = traceQueryParams(
      filters({ status: "weird", minDuration: "soon", attr: "nocolon", range: "forever" }),
    );
    assert.equal(params.status, undefined);
    assert.equal(params.min_duration, undefined);
    assert.equal(params.attr, undefined);
    assert.equal(params.from, "-1h");
  });

  it("uses the custom window when both a range and a start are given", () => {
    assert.deepEqual(traceWindow({ range: "custom", from: "2026-11-20T10:00:00Z", to: "" }), {
      from: "2026-11-20T10:00:00Z",
      to: undefined,
    });
    assert.deepEqual(
      traceWindow({ range: "custom", from: "2026-11-20T10:00:00Z", to: "2026-11-20T11:00:00Z" }),
      { from: "2026-11-20T10:00:00Z", to: "2026-11-20T11:00:00Z" },
    );
    // A custom range without a start cannot be sent: fall back to the default window.
    assert.equal(traceWindow({ range: "custom", from: "", to: "" }).from, "-1h");
  });

  it("labels ranges", () => {
    assert.equal(traceRangeLabel("4h"), "Last 4 hours");
    assert.equal(traceRangeLabel("custom"), "Custom range");
  });
});

describe("attribute filter", () => {
  it("splits on the first colon only", () => {
    assert.deepEqual(parseAttr("url.full:https://x.test/a"), {
      key: "url.full",
      value: "https://x.test/a",
    });
    assert.deepEqual(parseAttr(" db.system : postgres "), { key: "db.system", value: "postgres" });
    assert.equal(parseAttr(":value"), null);
    assert.equal(parseAttr("key:"), null);
    assert.equal(parseAttr("key"), null);
  });

  it("accepts only ok and error as a status", () => {
    assert.equal(normalizeStatus("ok"), "ok");
    assert.equal(normalizeStatus("error"), "error");
    assert.equal(normalizeStatus("all"), undefined);
  });
});

describe("filter tokens", () => {
  it("lists applied filters with readable values", () => {
    const tokens = filterTokens(
      filters({
        app: "a1,a2",
        status: "error",
        minDuration: "1s",
        attr: "db.system:postgres",
        q: "pay",
      }),
      { app: (id) => (id === "a1" ? "Checkout" : id) },
    );
    assert.deepEqual(
      tokens.map((t) => [t.field, t.value]),
      [
        ["App", "Checkout, a2"],
        ["Status", "error"],
        ["Duration", "≥ 1s"],
        ["Name", "pay"],
        ["Attribute", "db.system = postgres"],
      ],
    );
  });

  it("treats a changed range as a filter, the default as none", () => {
    assert.equal(hasTraceFilters(filters()), false);
    assert.equal(hasTraceFilters(filters({ range: "24h" })), true);
    assert.equal(hasTraceFilters(filters({ q: "x" })), true);
    assert.equal(hasTraceFilters(filters({ minDuration: "junk" })), false);
  });
});

describe("duration bars and counts", () => {
  it("scales each bar to the slowest trace of the page", () => {
    const rows = [{ duration_ns: 100 }, { duration_ns: 50 }, {}, { duration_ns: Number.NaN }];
    const slowest = slowestDuration(rows);
    assert.equal(slowest, 100);
    assert.equal(durationShare(100, slowest), 100);
    assert.equal(durationShare(50, slowest), 50);
    assert.equal(durationShare(undefined, slowest), 0);
    assert.equal(durationShare(0, slowest), 0);
    assert.equal(durationShare(5, 1000), 2);
    assert.equal(slowestDuration([]), 0);
    assert.equal(durationShare(10, 0), 0);
  });

  it("reads ok and error counts from the facets", () => {
    assert.deepEqual(
      statusCounts([
        { value: "ok", count: 9 },
        { value: "error", count: 2 },
      ]),
      { ok: 9, error: 2 },
    );
    assert.deepEqual(statusCounts(undefined), { ok: 0, error: 0 });
  });
});

describe("links", () => {
  it("builds trace and list URLs", () => {
    assert.equal(traceHref("abc"), "/logs/traces/abc");
    assert.equal(traceHref("abc", "s 1"), "/logs/traces/abc?span=s%201");
    assert.equal(tracesHref({}), "/logs/traces");
    assert.equal(
      tracesHref({ serviceId: "svc_1", range: "4h", q: "" }),
      "/logs/traces?serviceId=svc_1&range=4h",
    );
    assert.equal(
      attributeFilterHref("http.route", "/pay/:id"),
      "/logs/traces?attr=http.route%3A%2Fpay%2F%3Aid",
    );
  });

  it("opens the logs of a trace in a window around it", () => {
    const href = logsHrefForTrace("t1", "2026-11-20T10:00:00Z", 2_000_000_000);
    const params = new URL(href, "http://x").searchParams;
    assert.equal(params.get("trace"), "t1");
    assert.equal(params.get("range"), "custom");
    assert.equal(params.get("from"), "2026-11-20T09:00:00.000Z");
    assert.equal(params.get("to"), "2026-11-20T11:00:02.000Z");
  });

  it("has no log window for a trace without a start", () => {
    assert.equal(traceLogWindow(undefined, 5), null);
    assert.equal(traceLogWindow("not a date", 5), null);
    assert.deepEqual(traceLogWindow("2026-11-20T10:00:00Z"), {
      from: "2026-11-20T09:00:00.000Z",
      to: "2026-11-20T11:00:00.000Z",
    });
  });

  it("falls back to a week when the trace has no start time", () => {
    const params = new URL(logsHrefForTrace("t1"), "http://x").searchParams;
    assert.equal(params.get("range"), "7d");
    assert.equal(params.get("from"), null);
  });
});
