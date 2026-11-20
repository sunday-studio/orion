import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { parseAsArrayOf, parseAsString } from "nuqs/server";
import { attrParam, attributeFilterHref, tracesHref } from "./trace-links.ts";
import {
  EMPTY_TRACE_FILTERS,
  MAX_TRACE_ATTRS,
  type TraceFilters,
  addAttr,
  attrDraft,
  cleanAttrs,
  filterTokens,
  hasTraceFilters,
  removeAttr,
  removeToken,
  traceQueryParams,
} from "./traces-state.ts";

const filters = (patch: Partial<TraceFilters> = {}): TraceFilters => ({
  ...EMPTY_TRACE_FILTERS,
  ...patch,
});

// The parser the Traces page uses for the attr URL parameter.
const urlParser = parseAsArrayOf(parseAsString).withDefault([]);

describe("several attribute filters", () => {
  it("keeps valid pairs once, trimmed, in the order added", () => {
    assert.deepEqual(
      cleanAttrs(["a:1", " b : 2 ", "a:1", "nocolon", ":x", "y:", "url:https://x.test/p"]),
      ["a:1", "b:2", "url:https://x.test/p"],
    );
    assert.deepEqual(cleanAttrs([]), []);
  });

  it("adds a typed pair and ignores what is not one", () => {
    assert.deepEqual(addAttr(["a:1"], "b:2"), ["a:1", "b:2"]);
    assert.deepEqual(addAttr(["a:1"], " a : 1 "), ["a:1"]);
    assert.deepEqual(addAttr(["a:1"], "junk"), ["a:1"]);
    assert.deepEqual(addAttr([], "http.status_code:500"), ["http.status_code:500"]);
  });

  it("is capped at what Core accepts", () => {
    const full = Array.from({ length: MAX_TRACE_ATTRS }, (_, i) => `k${i}:v`);
    assert.equal(cleanAttrs([...full, "extra:1"]).length, MAX_TRACE_ATTRS);
    assert.equal(attrDraft(full, "extra:1"), "full");
    assert.equal(attrDraft(full, "k0:v"), "valid");
    assert.deepEqual(addAttr(full, "extra:1"), full);
    assert.equal(attrDraft(["a:1"], "b:2"), "valid");
    assert.equal(attrDraft(["a:1"], "  "), "empty");
    assert.equal(attrDraft(["a:1"], "nocolon"), "invalid");
  });

  it("removes one pair and keeps the others", () => {
    assert.deepEqual(removeAttr(["a:1", "b:2", "c:3"], "b:2"), ["a:1", "c:3"]);
    assert.deepEqual(removeAttr(["a:1"], "a:1"), []);
    assert.deepEqual(removeAttr([" a : 1 ", "b:2"], "a:1"), ["b:2"]);
  });

  it("sends every pair as its own attr parameter and none when there are none", () => {
    assert.deepEqual(traceQueryParams(filters({ attr: ["a:1", "b:2"] })).attr, ["a:1", "b:2"]);
    assert.equal(traceQueryParams(filters({ attr: [] })).attr, undefined);
    assert.equal(traceQueryParams(filters({ attr: ["nocolon"] })).attr, undefined);
  });

  it("shows one token per pair and removes only that pair", () => {
    const current = filters({ attr: ["a:1", "b:2"], q: "pay" });
    const tokens = filterTokens(current);
    assert.deepEqual(
      tokens.map((t) => t.id),
      ["q", "attr:a:1", "attr:b:2"],
    );
    assert.deepEqual(removeToken(current, tokens[2]), { attr: ["a:1"] });
    assert.deepEqual(removeToken(current, tokens[0]), { q: "" });
    assert.equal(hasTraceFilters(filters({ attr: ["a:1"] })), true);
    assert.equal(hasTraceFilters(filters({ attr: ["junk"] })), false);
  });
});

describe("attribute filters in the URL", () => {
  const query = (href: string) => new URL(href, "http://x").searchParams.get("attr") ?? "";

  it("writes the list as the URL state reads it back", () => {
    const pairs = ["http.status_code:500", "db.system:postgres", "url.full:https://x.test/a"];
    assert.equal(
      query(tracesHref({ attr: pairs })),
      "http.status_code:500,db.system:postgres,url.full:https://x.test/a",
    );
    assert.deepEqual(urlParser.parse(query(tracesHref({ attr: pairs }))), pairs);
  });

  it("keeps a comma inside a value as part of that pair", () => {
    const pairs = ["http.route:/a,b", "db.system:postgres"];
    assert.deepEqual(urlParser.parse(query(tracesHref({ attr: pairs }))), pairs);
    // And what the page itself writes is the same text.
    assert.equal(urlParser.serialize(pairs), attrParam(pairs));
  });

  it("opens one attribute from a span and leaves empty lists out", () => {
    assert.deepEqual(urlParser.parse(query(attributeFilterHref("a.b", "x,y"))), ["a.b:x,y"]);
    assert.equal(tracesHref({ attr: [] }), "/logs/traces");
  });

  it("still reads the single attr of an older link", () => {
    assert.deepEqual(urlParser.parse("http.status_code:500"), ["http.status_code:500"]);
  });
});
