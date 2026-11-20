import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { ApiTraceSpan } from "../../orion-sdk/index.ts";
import {
  ancestorKeys,
  barGeometry,
  buildTrace,
  defaultSelectedKey,
  errorKeys,
  parentKeys,
  revealed,
  skewMessage,
  visibleRows,
} from "./trace-tree.ts";

const MS = 1_000_000;
const span = (
  id: string,
  parent: string | undefined,
  startMs: number,
  durationMs: number,
  extra: Partial<ApiTraceSpan> = {},
): ApiTraceSpan => ({
  span_id: id,
  parent_span_id: parent,
  name: `span ${id}`,
  start_offset_ns: startMs * MS,
  duration_ns: durationMs * MS,
  service_name: "api",
  ...extra,
});

const keys = (nodes: readonly { key: string }[]) => nodes.map((n) => n.key);

describe("building the tree", () => {
  it("nests children under their parents in start order", () => {
    const tree = buildTrace([
      span("c2", "a", 30, 10),
      span("a", undefined, 0, 100),
      span("c1", "a", 10, 10),
      span("g1", "c1", 12, 5),
    ]);
    assert.deepEqual(keys(tree.roots), ["a"]);
    assert.deepEqual(keys(tree.order), ["a", "c1", "g1", "c2"]);
    assert.deepEqual(
      tree.order.map((n) => n.depth),
      [0, 1, 2, 1],
    );
    assert.equal(tree.nodes.get("a")?.descendants, 3);
    assert.equal(tree.trueRootCount, 1);
    assert.equal(tree.orphanCount, 0);
    assert.equal(tree.totalNs, 100 * MS);
  });

  it("keeps a span whose parent never arrived as an orphan root", () => {
    const tree = buildTrace([span("a", undefined, 0, 50), span("b", "missing", 10, 10)]);
    assert.deepEqual(keys(tree.roots), ["a", "b"]);
    assert.equal(tree.nodes.get("b")?.orphan, true);
    assert.equal(tree.orphanCount, 1);
    assert.equal(tree.trueRootCount, 1);
  });

  it("has no true root when only children arrived", () => {
    const tree = buildTrace([span("b", "root", 5, 10), span("c", "root", 0, 10)]);
    assert.deepEqual(keys(tree.roots), ["c", "b"]);
    assert.equal(tree.trueRootCount, 0);
    assert.equal(tree.orphanCount, 2);
    assert.equal(defaultSelectedKey(tree), "c");
  });

  it("lists several true roots by start time", () => {
    const tree = buildTrace([span("late", undefined, 40, 5), span("early", undefined, 0, 5)]);
    assert.deepEqual(keys(tree.roots), ["early", "late"]);
    assert.equal(tree.trueRootCount, 2);
  });

  it("gives a repeated span id its own node", () => {
    const tree = buildTrace([span("a", undefined, 0, 10), span("a", undefined, 5, 10)]);
    assert.equal(tree.order.length, 2);
    assert.equal(new Set(keys(tree.order)).size, 2);
  });

  it("does not loop on a span that is its own parent or on a parent cycle", () => {
    const self = buildTrace([span("a", "a", 0, 10)]);
    assert.deepEqual(keys(self.roots), ["a"]);
    assert.equal(self.nodes.get("a")?.orphan, true);

    const cycle = buildTrace([span("a", "b", 0, 10), span("b", "a", 5, 10), span("c", "b", 6, 2)]);
    assert.equal(cycle.order.length, 3);
    assert.equal(cycle.roots.length, 1);
    assert.equal(cycle.orphanCount, 1);
  });

  it("copes with no spans and with missing numbers", () => {
    const empty = buildTrace([]);
    assert.equal(empty.order.length, 0);
    assert.equal(empty.totalNs, 0);
    assert.equal(defaultSelectedKey(empty), undefined);

    const sparse = buildTrace([{ name: "x" }, { span_id: "n", duration_ns: -5 }]);
    assert.equal(sparse.order.length, 2);
    assert.equal(sparse.nodes.get("n")?.durationNs, 0);
  });

  it("survives a chain far deeper than the call stack", () => {
    const spans = Array.from({ length: 20_000 }, (_, i) =>
      span(`s${i}`, i === 0 ? undefined : `s${i - 1}`, i, 1),
    );
    const tree = buildTrace(spans);
    assert.equal(tree.order.length, 20_000);
    assert.equal(tree.order[19_999].depth, 19_999);
    assert.equal(tree.roots[0].descendants, 19_999);
  });
});

describe("timing", () => {
  it("measures self time without counting overlapping children twice", () => {
    const tree = buildTrace([
      span("a", undefined, 0, 100),
      span("b", "a", 10, 30),
      span("c", "a", 20, 30),
      span("d", "a", 90, 30),
    ]);
    // Children cover 10..50 and 90..100 (clipped to the parent): 50ms. Self time 50ms.
    assert.equal(tree.nodes.get("a")?.selfNs, 50 * MS);
    assert.equal(tree.nodes.get("b")?.selfNs, 30 * MS);
  });

  it("flags a child that starts before its parent as clock skew", () => {
    const tree = buildTrace([span("a", undefined, 10, 50), span("b", "a", 4, 10)]);
    const b = tree.nodes.get("b");
    assert.deepEqual(b?.skew, { kind: "before-parent", ns: 6 * MS });
    assert.match(skewMessage(b!, (ns) => `${ns / MS}ms`) ?? "", /Starts 6ms before its parent/);
  });

  it("notes a child that outlives its parent without calling it skew", () => {
    const tree = buildTrace([span("a", undefined, 0, 10), span("b", "a", 5, 20)]);
    assert.equal(tree.nodes.get("b")?.skew?.kind, "outlives-parent");
    assert.match(skewMessage(tree.nodes.get("b")!, String) ?? "", /Ends/);
    assert.equal(skewMessage(tree.nodes.get("a")!, String), undefined);
  });

  it("starts the axis at the earliest span even when no span is at zero", () => {
    const tree = buildTrace([span("a", undefined, 100, 20), span("b", "a", 105, 5)]);
    assert.equal(tree.startNs, 100 * MS);
    assert.equal(tree.totalNs, 20 * MS);
    assert.deepEqual(barGeometry(tree.nodes.get("b")!, tree), { leftPct: 25, widthPct: 25 });
  });

  it("keeps bars inside the track and visible", () => {
    const tree = buildTrace([span("a", undefined, 0, 1000), span("tiny", "a", 999.99, 0)]);
    const bar = barGeometry(tree.nodes.get("tiny")!, tree);
    assert.equal(bar.widthPct, 0.4);
    assert.ok(bar.leftPct + bar.widthPct <= 100);
    const flat = buildTrace([span("z", undefined, 0, 0)]);
    assert.deepEqual(barGeometry(flat.order[0], flat), { leftPct: 0, widthPct: 100 });
  });
});

describe("collapsing and selecting", () => {
  const tree = buildTrace([
    span("a", undefined, 0, 100),
    span("b", "a", 10, 30),
    span("c", "b", 12, 5, { status: "error" }),
    span("d", "a", 50, 10),
  ]);

  it("hides what sits below a collapsed span", () => {
    assert.deepEqual(
      visibleRows(tree, new Set()).map((r) => r.node.key),
      ["a", "b", "c", "d"],
    );
    const rows = visibleRows(tree, new Set(["b"]));
    assert.deepEqual(
      rows.map((r) => r.node.key),
      ["a", "b", "d"],
    );
    assert.equal(rows[1].collapsed, true);
    assert.equal(rows[1].hasChildren, true);
    assert.equal(rows[2].hasChildren, false);
  });

  it("ignores collapsing a span with no children", () => {
    assert.equal(visibleRows(tree, new Set(["d"])).length, 4);
  });

  it("opens the path to a hidden span", () => {
    assert.deepEqual(ancestorKeys(tree, "c"), ["b", "a"]);
    const next = revealed(tree, new Set(["a", "b", "d"]), "c");
    assert.deepEqual([...next].sort(), ["d"]);
    const same = new Set(["d"]);
    assert.equal(revealed(tree, same, "c"), same);
  });

  it("selects the first failing span, and counts errors below a span", () => {
    assert.equal(defaultSelectedKey(tree), "c");
    assert.deepEqual(errorKeys(tree), ["c"]);
    assert.equal(tree.nodes.get("a")?.subtreeErrors, 1);
    assert.equal(tree.nodes.get("d")?.subtreeErrors, 0);
    assert.deepEqual(parentKeys(tree), ["a", "b"]);
  });

  it("falls back to the first root when nothing failed", () => {
    assert.equal(defaultSelectedKey(buildTrace([span("r", undefined, 0, 1)])), "r");
  });
});
