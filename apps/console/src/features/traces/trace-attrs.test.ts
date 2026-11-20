import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  attributeRows,
  eventOffsetInSpan,
  exceptionOf,
  kindLabel,
  serviceLabel,
  sortedEvents,
} from "./trace-attrs.ts";

describe("attribute rows", () => {
  it("sorts by key and writes every value as text", () => {
    const rows = attributeRows({
      "http.status_code": 500,
      "db.system": "postgres",
      retry: false,
      empty: "",
      nothing: null,
      nested: { a: [1, 2] },
    });
    assert.deepEqual(
      rows.map((r) => [r.key, r.text]),
      [
        ["db.system", "postgres"],
        ["empty", '""'],
        ["http.status_code", "500"],
        ["nested", '{"a":[1,2]}'],
        ["nothing", "null"],
        ["retry", "false"],
      ],
    );
  });

  it("offers a filter only for short scalars", () => {
    const rows = attributeRows({
      a: "x",
      b: 3,
      c: { k: 1 },
      d: "",
      e: null,
      f: "line\nbreak",
      g: "y".repeat(500),
    });
    const filterable = Object.fromEntries(rows.map((r) => [r.key, r.filterable]));
    assert.deepEqual(filterable, {
      a: true,
      b: true,
      c: false,
      d: false,
      e: false,
      f: false,
      g: false,
    });
  });

  it("gives no rows for missing or odd input", () => {
    for (const input of [undefined, null, "text", 4, [1, 2]]) {
      assert.deepEqual(attributeRows(input), []);
    }
  });
});

describe("span labels", () => {
  it("names the service, falling back to the app", () => {
    assert.equal(serviceLabel({ service_name: "api", app_name: "App" }), "api");
    assert.equal(serviceLabel({ app_name: "App" }), "App");
    assert.equal(serviceLabel({}), "unknown service");
  });

  it("hides an unspecified kind", () => {
    assert.equal(kindLabel("server"), "Server");
    assert.equal(kindLabel("unspecified"), "");
    assert.equal(kindLabel(undefined), "");
  });
});

describe("events", () => {
  it("places an event inside its span and never before it", () => {
    assert.equal(eventOffsetInSpan({ offset_ns: 15 }, 10), 5);
    assert.equal(eventOffsetInSpan({ offset_ns: 5 }, 10), 0);
    assert.equal(eventOffsetInSpan({}, 10), undefined);
    assert.equal(eventOffsetInSpan({ offset_ns: 7 }, undefined), 7);
  });

  it("orders events by time with undated ones last", () => {
    const events = sortedEvents([
      { name: "late", offset_ns: 9 },
      { name: "undated" },
      { name: "early", offset_ns: 1 },
    ]);
    assert.deepEqual(
      events.map((e) => e.name),
      ["early", "late", "undated"],
    );
    assert.deepEqual(sortedEvents(undefined), []);
  });

  it("reads the exception an instrumentation library recorded", () => {
    const span = {
      events: [
        { name: "other" },
        {
          name: "exception",
          attributes: { "exception.type": "TimeoutError", "exception.message": "deadline" },
        },
      ],
    };
    assert.deepEqual(exceptionOf(span), {
      type: "TimeoutError",
      message: "deadline",
      stack: undefined,
    });
    assert.equal(exceptionOf({ events: [{ name: "exception" }] }), undefined);
    assert.equal(exceptionOf({}), undefined);
  });
});
