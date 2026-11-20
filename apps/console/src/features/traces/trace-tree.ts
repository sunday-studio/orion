// Turns the flat span list of one trace into the tree the waterfall draws. Pure, no React.
// A trace is whatever arrived, so this copes with orphans, several roots, repeated span ids,
// parent loops and clocks that disagree. start_offset_ns from Core places every bar.
import type { ApiTraceSpan } from "../../orion-sdk/index.ts";

export type SpanSkew = {
  /** "before-parent": the child starts earlier than its parent, which clocks disagreeing explains. "outlives-parent": it ends later, which async work does legitimately. */
  kind: "before-parent" | "outlives-parent";
  ns: number;
};

export type SpanNode = {
  /** Unique within the trace even when an exporter repeated a span id. */
  key: string;
  span: ApiTraceSpan;
  parentKey?: string;
  depth: number;
  startNs: number;
  endNs: number;
  durationNs: number;
  /** Time in this span that no child covers. */
  selfNs: number;
  isError: boolean;
  /** It names a parent that is not in the trace (or one that would make a loop). */
  orphan: boolean;
  skew?: SpanSkew;
  children: SpanNode[];
  /** Spans below this one. */
  descendants: number;
  /** Error spans in this subtree, this span included. */
  subtreeErrors: number;
};

export type TraceTree = {
  roots: SpanNode[];
  /** Every node, parents before children, in the order the waterfall lists them. */
  order: SpanNode[];
  nodes: Map<string, SpanNode>;
  startNs: number;
  endNs: number;
  totalNs: number;
  orphanCount: number;
  /** Spans with no parent at all. Zero means the real root has not arrived. */
  trueRootCount: number;
  services: string[];
};

const finite = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);

const byStart = (a: SpanNode, b: SpanNode, index: Map<string, number>) =>
  a.startNs - b.startNs || (index.get(a.key) ?? 0) - (index.get(b.key) ?? 0);

/** Length of the union of `intervals` clipped to [from, to]. */
const coveredNs = (intervals: readonly (readonly [number, number])[], from: number, to: number) => {
  const clipped = intervals
    .map(([a, b]) => [Math.max(a, from), Math.min(b, to)] as const)
    .filter(([a, b]) => b > a)
    .sort((x, y) => x[0] - y[0]);
  let covered = 0;
  let cursor = -Infinity;
  for (const [a, b] of clipped) {
    const start = Math.max(a, cursor);
    if (b > start) covered += b - start;
    cursor = Math.max(cursor, b);
  }
  return covered;
};

export const buildTrace = (spans: readonly ApiTraceSpan[]): TraceTree => {
  const index = new Map<string, number>();
  const nodes = new Map<string, SpanNode>();
  const firstBySpanId = new Map<string, string>();
  const list: SpanNode[] = [];

  spans.forEach((span, i) => {
    const id = span.span_id ?? "";
    const key = id && !nodes.has(id) ? id : `${id || "span"}#${i}`;
    const startNs = finite(span.start_offset_ns) ? span.start_offset_ns : 0;
    const durationNs = finite(span.duration_ns) && span.duration_ns > 0 ? span.duration_ns : 0;
    const node: SpanNode = {
      key,
      span,
      depth: 0,
      startNs,
      endNs: startNs + durationNs,
      durationNs,
      selfNs: durationNs,
      isError: span.status === "error",
      orphan: false,
      children: [],
      descendants: 0,
      subtreeErrors: 0,
    };
    nodes.set(key, node);
    index.set(key, i);
    list.push(node);
    if (id && !firstBySpanId.has(id)) firstBySpanId.set(id, key);
  });

  // Resolve parents. A parent that is missing, is the span itself or sits below it makes an orphan root.
  const parentOf = new Map<string, string>();
  for (const node of list) {
    const parentId = node.span.parent_span_id ?? "";
    if (!parentId) continue;
    const parentKey = firstBySpanId.get(parentId);
    if (parentKey && parentKey !== node.key) parentOf.set(node.key, parentKey);
    else node.orphan = true;
  }
  const state = new Map<string, 1 | 2>();
  for (const node of list) {
    const path: string[] = [];
    let current: string | undefined = node.key;
    while (current && !state.has(current)) {
      state.set(current, 1);
      path.push(current);
      current = parentOf.get(current);
    }
    if (current && state.get(current) === 1) {
      // The chain came back to a span on this walk: cut the loop at the last link.
      const last = path[path.length - 1];
      parentOf.delete(last);
      const cut = nodes.get(last);
      if (cut) cut.orphan = true;
    }
    for (const key of path) state.set(key, 2);
  }

  const roots: SpanNode[] = [];
  for (const node of list) {
    const parentKey = parentOf.get(node.key);
    if (parentKey) {
      node.parentKey = parentKey;
      nodes.get(parentKey)?.children.push(node);
    } else {
      roots.push(node);
    }
  }
  const sort = (items: SpanNode[]) => items.sort((a, b) => byStart(a, b, index));
  sort(roots);

  // Depth and listing order, iterative so a very deep trace cannot overflow the stack.
  const order: SpanNode[] = [];
  const stack = [...roots].reverse().map((node) => ({ node, depth: 0 }));
  while (stack.length > 0) {
    const item = stack.pop();
    if (!item) break;
    item.node.depth = item.depth;
    order.push(item.node);
    sort(item.node.children);
    for (let i = item.node.children.length - 1; i >= 0; i -= 1) {
      stack.push({ node: item.node.children[i], depth: item.depth + 1 });
    }
  }

  for (let i = order.length - 1; i >= 0; i -= 1) {
    const node = order[i];
    node.subtreeErrors =
      (node.isError ? 1 : 0) + node.children.reduce((n, c) => n + c.subtreeErrors, 0);
    node.descendants = node.children.reduce((n, c) => n + 1 + c.descendants, 0);
    node.selfNs = Math.max(
      node.durationNs -
        coveredNs(
          node.children.map((c) => [c.startNs, c.endNs] as const),
          node.startNs,
          node.endNs,
        ),
      0,
    );
    const parent = node.parentKey ? nodes.get(node.parentKey) : undefined;
    if (parent) {
      if (node.startNs < parent.startNs) {
        node.skew = { kind: "before-parent", ns: parent.startNs - node.startNs };
      } else if (node.endNs > parent.endNs) {
        node.skew = { kind: "outlives-parent", ns: node.endNs - parent.endNs };
      }
    }
  }

  const startNs = order.length ? Math.min(...order.map((n) => n.startNs)) : 0;
  const endNs = order.length ? Math.max(...order.map((n) => n.endNs)) : 0;
  const services: string[] = [];
  for (const node of order) {
    const name = node.span.service_name;
    if (name && !services.includes(name)) services.push(name);
  }
  return {
    roots,
    order,
    nodes,
    startNs,
    endNs,
    totalNs: Math.max(endNs - startNs, 0),
    orphanCount: list.filter((n) => n.orphan).length,
    trueRootCount: roots.filter((n) => !n.orphan).length,
    services,
  };
};

export type VisibleRow = { node: SpanNode; hasChildren: boolean; collapsed: boolean };

/** The rows to draw: every span except those below a collapsed one. */
export const visibleRows = (tree: TraceTree, collapsed: ReadonlySet<string>): VisibleRow[] => {
  const rows: VisibleRow[] = [];
  const stack = [...tree.roots].reverse();
  while (stack.length > 0) {
    const node = stack.pop();
    if (!node) break;
    const isCollapsed = collapsed.has(node.key) && node.children.length > 0;
    rows.push({ node, hasChildren: node.children.length > 0, collapsed: isCollapsed });
    if (!isCollapsed) {
      for (let i = node.children.length - 1; i >= 0; i -= 1) stack.push(node.children[i]);
    }
  }
  return rows;
};

/** Keys of the spans above `key`, nearest first. */
export const ancestorKeys = (tree: TraceTree, key: string): string[] => {
  const keys: string[] = [];
  let parent = tree.nodes.get(key)?.parentKey;
  while (parent && keys.length <= tree.order.length) {
    keys.push(parent);
    parent = tree.nodes.get(parent)?.parentKey;
  }
  return keys;
};

/** Collapsed set with every ancestor of `key` opened, so the span can be seen. */
export const revealed = (
  tree: TraceTree,
  collapsed: ReadonlySet<string>,
  key: string,
): ReadonlySet<string> => {
  const hidden = ancestorKeys(tree, key).filter((k) => collapsed.has(k));
  if (hidden.length === 0) return collapsed;
  const next = new Set(collapsed);
  for (const k of hidden) next.delete(k);
  return next;
};

/** Every span that has children, for "collapse all". */
export const parentKeys = (tree: TraceTree): string[] =>
  tree.order.filter((n) => n.children.length > 0).map((n) => n.key);

export type Bar = { leftPct: number; widthPct: number };

/** Where a span's bar sits, as percentages of the trace. Never narrower than a sliver, never outside 0..100. */
export const barGeometry = (node: SpanNode, tree: TraceTree, minPct = 0.4): Bar => {
  if (tree.totalNs <= 0) return { leftPct: 0, widthPct: 100 };
  const left = ((node.startNs - tree.startNs) / tree.totalNs) * 100;
  const width = (node.durationNs / tree.totalNs) * 100;
  const widthPct = Math.min(Math.max(width, minPct), 100);
  const leftPct = Math.min(Math.max(left, 0), 100 - widthPct);
  return { leftPct, widthPct };
};

/** The span to show first: the earliest one that failed, else the first root. */
export const defaultSelectedKey = (tree: TraceTree): string | undefined =>
  tree.order.find((n) => n.isError)?.key ?? tree.roots[0]?.key;

/** Keys of the spans carrying an error, in listing order. */
export const errorKeys = (tree: TraceTree): string[] =>
  tree.order.filter((n) => n.isError).map((n) => n.key);

/** How a span relates to its parent in words, for the inspector. */
export const skewMessage = (node: SpanNode, format: (ns: number) => string): string | undefined => {
  if (!node.skew) return undefined;
  return node.skew.kind === "before-parent"
    ? `Starts ${format(node.skew.ns)} before its parent. The two services' clocks may differ.`
    : `Ends ${format(node.skew.ns)} after its parent, as work started in the background does.`;
};
