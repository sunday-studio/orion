import { ChevronRight, CircleAlert, TriangleAlert } from "lucide-react";
import { type KeyboardEvent, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { cn } from "@/utils/cn";
import { serviceLabel } from "../trace-attrs";
import { axisTicks, formatDurationNs } from "../trace-format";
import { type SpanNode, type TraceTree, type VisibleRow, barGeometry } from "../trace-tree";

const ROW_H = 30;
const HEAD_H = 30;
const NAME_W = 240;
const OVERSCAN = 8;
const INDENT = 14;
const MAX_INDENT_LEVELS = 16;

const barColor = (node: SpanNode) =>
  node.isError ? "var(--color-dot-down)" : "var(--color-brand-chart)";

const rowLabel = (node: SpanNode) =>
  [
    node.span.name || "(unnamed)",
    serviceLabel(node.span),
    formatDurationNs(node.durationNs),
    node.isError ? "failed" : "",
    node.orphan ? "parent missing" : "",
  ]
    .filter(Boolean)
    .join(", ");

const Row = ({
  row,
  tree,
  selected,
  tabbable,
  top,
  onSelect,
  onToggle,
}: {
  row: VisibleRow;
  tree: TraceTree;
  selected: boolean;
  tabbable: boolean;
  top: number;
  onSelect: () => void;
  onToggle: () => void;
}) => {
  const { node } = row;
  const bar = barGeometry(node, tree);
  const labelLeft = bar.leftPct + bar.widthPct > 78;
  const hiddenErrors = row.collapsed ? node.subtreeErrors - (node.isError ? 1 : 0) : 0;
  return (
    <div
      role="treeitem"
      data-key={node.key}
      aria-level={node.depth + 1}
      aria-expanded={row.hasChildren ? !row.collapsed : undefined}
      aria-selected={selected}
      aria-label={rowLabel(node)}
      tabIndex={tabbable ? 0 : -1}
      onClick={onSelect}
      style={{ top, height: ROW_H, gridTemplateColumns: `${NAME_W}px minmax(0,1fr)` }}
      className={cn(
        "absolute inset-x-0 grid cursor-pointer items-center border-b border-line-soft text-[12.5px] outline-none transition-colors duration-150",
        "focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand",
        selected ? "bg-selected-row" : "hover:bg-sub",
      )}
    >
      <div
        className="flex min-w-0 items-center gap-1 overflow-hidden pr-2"
        style={{ paddingLeft: 8 + Math.min(node.depth, MAX_INDENT_LEVELS) * INDENT }}
      >
        {row.hasChildren ? (
          <span
            aria-hidden
            onClick={(event) => {
              event.stopPropagation();
              onToggle();
            }}
            className="flex size-4 shrink-0 items-center justify-center rounded text-quiet hover:bg-tray hover:text-ink"
          >
            <ChevronRight
              className={cn(
                "size-3.5 transition-transform duration-150",
                !row.collapsed && "rotate-90",
              )}
            />
          </span>
        ) : (
          <span aria-hidden className="size-4 shrink-0" />
        )}
        {node.isError && (
          <CircleAlert aria-hidden className="size-3.5 shrink-0 text-(--tone-down-fg)" />
        )}
        <span className={cn("truncate", node.isError ? "font-medium" : "")}>
          {node.span.name || "(unnamed)"}
        </span>
        <span className="min-w-0 max-w-[45%] truncate text-[11.5px] text-faint">
          {serviceLabel(node.span)}
        </span>
        {hiddenErrors > 0 && (
          <span className="shrink-0 rounded bg-(--tone-down-bg) px-1 text-[11px] text-(--tone-down-fg)">
            {hiddenErrors} failed
          </span>
        )}
        {node.skew?.kind === "before-parent" && (
          <TriangleAlert aria-hidden className="size-3.5 shrink-0 text-(--tone-degraded-fg)" />
        )}
      </div>
      <div className="relative h-full">
        <span
          aria-hidden
          className={cn(
            "absolute top-1/2 h-2.5 -translate-y-1/2 rounded-[3px]",
            node.orphan && "opacity-70",
          )}
          style={{
            left: `${bar.leftPct}%`,
            width: `${bar.widthPct}%`,
            background: barColor(node),
            outline: node.orphan ? "1px dashed var(--color-quiet)" : undefined,
            outlineOffset: 1,
          }}
        />
        <span
          aria-hidden
          className="absolute top-1/2 -translate-y-1/2 whitespace-nowrap px-1.5 text-[11.5px] tabular-nums text-quiet"
          style={
            labelLeft
              ? { right: `${100 - bar.leftPct}%` }
              : { left: `${bar.leftPct + bar.widthPct}%` }
          }
        >
          {formatDurationNs(node.durationNs)}
        </span>
      </div>
    </div>
  );
};

/**
 * The span tree against a shared time axis. Only the rows in view are rendered, so a trace of
 * thousands of spans stays light. It is a tree widget: arrows move, right and left open and close.
 */
export const Waterfall = ({
  tree,
  rows,
  selectedKey,
  onSelect,
  onToggle,
}: {
  tree: TraceTree;
  rows: VisibleRow[];
  selectedKey?: string;
  onSelect: (key: string) => void;
  onToggle: (key: string) => void;
}) => {
  const scroller = useRef<HTMLDivElement>(null);
  const wantFocus = useRef(false);
  const [view, setView] = useState({ top: 0, height: 640 });
  const ticks = useMemo(() => axisTicks(tree.totalNs), [tree.totalNs]);

  useLayoutEffect(() => {
    const node = scroller.current;
    if (!node) return;
    const measure = () => setView({ top: node.scrollTop, height: node.clientHeight || 640 });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  const start = Math.max(Math.floor(view.top / ROW_H) - OVERSCAN, 0);
  const end = Math.min(Math.ceil((view.top + view.height) / ROW_H) + OVERSCAN, rows.length);
  const selectedIndex = rows.findIndex((row) => row.node.key === selectedKey);
  const tabIndexRow = selectedIndex >= 0 ? selectedIndex : 0;

  // After a key press, scroll the new row into view, then put focus on it once it is rendered.
  useEffect(() => {
    if (!wantFocus.current || selectedIndex < 0) return;
    const container = scroller.current;
    if (!container) return;
    const top = selectedIndex * ROW_H;
    if (top < container.scrollTop) container.scrollTop = top;
    else if (top + ROW_H > container.scrollTop + container.clientHeight - HEAD_H) {
      container.scrollTop = top + ROW_H - container.clientHeight + HEAD_H;
    }
    const target = container.querySelector<HTMLElement>(
      `[data-key="${CSS.escape(selectedKey ?? "")}"]`,
    );
    if (target) {
      wantFocus.current = false;
      target.focus({ preventScroll: true });
    }
  }, [selectedKey, selectedIndex, start, end]);

  const go = (index: number) => {
    const row = rows[Math.min(Math.max(index, 0), rows.length - 1)];
    if (!row) return;
    wantFocus.current = true;
    onSelect(row.node.key);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (rows.length === 0 || selectedIndex < 0) {
      if (event.key === "ArrowDown" || event.key === "Home") {
        event.preventDefault();
        go(0);
      }
      return;
    }
    const row = rows[selectedIndex];
    switch (event.key) {
      case "ArrowDown":
        go(selectedIndex + 1);
        break;
      case "ArrowUp":
        go(selectedIndex - 1);
        break;
      case "Home":
        go(0);
        break;
      case "End":
        go(rows.length - 1);
        break;
      case "ArrowRight":
        if (row.collapsed) onToggle(row.node.key);
        else if (row.hasChildren) go(selectedIndex + 1);
        break;
      case "ArrowLeft": {
        if (row.hasChildren && !row.collapsed) {
          onToggle(row.node.key);
        } else if (row.node.parentKey) {
          go(rows.findIndex((r) => r.node.key === row.node.parentKey));
        }
        break;
      }
      case " ":
      case "Enter":
        break;
      default:
        return;
    }
    event.preventDefault();
  };

  return (
    <div
      ref={scroller}
      onScroll={(event) =>
        setView({ top: event.currentTarget.scrollTop, height: event.currentTarget.clientHeight })
      }
      className="max-h-[min(70vh,720px)] min-h-0 overflow-auto rounded-[10px]"
    >
      <div className="min-w-[560px]">
        <div
          aria-hidden
          className="sticky top-0 z-10 grid items-center border-b border-hairline bg-sub text-[11.5px] text-quiet"
          style={{ height: HEAD_H, gridTemplateColumns: `${NAME_W}px minmax(0,1fr)` }}
        >
          <span className="px-3">Span</span>
          <span className="relative h-full">
            {ticks.map((tick) => (
              <span
                key={tick.fraction}
                className="absolute top-1/2 -translate-y-1/2 tabular-nums"
                style={
                  tick.fraction === 1
                    ? { right: 8 }
                    : { left: `${tick.fraction * 100}%`, paddingLeft: tick.fraction === 0 ? 0 : 4 }
                }
              >
                {tick.label}
              </span>
            ))}
          </span>
        </div>
        <div
          role="tree"
          aria-label={`Spans of the trace, ${rows.length} shown`}
          aria-multiselectable={false}
          onKeyDown={onKeyDown}
          className="relative"
          style={{ height: rows.length * ROW_H }}
        >
          <div
            aria-hidden
            className="pointer-events-none absolute inset-y-0 right-0"
            style={{ left: NAME_W }}
          >
            {ticks.slice(1, -1).map((tick) => (
              <span
                key={tick.fraction}
                className="absolute inset-y-0 w-px bg-line-soft"
                style={{ left: `${tick.fraction * 100}%` }}
              />
            ))}
          </div>
          {/* The row that holds the tab stop is always rendered, so the tree can be entered from anywhere. */}
          {[
            ...(tabIndexRow < start || tabIndexRow >= end ? [tabIndexRow] : []),
            ...Array.from({ length: Math.max(end - start, 0) }, (_, offset) => start + offset),
          ].map((index) => {
            const row = rows[index];
            if (!row) return null;
            return (
              <Row
                key={row.node.key}
                row={row}
                tree={tree}
                top={index * ROW_H}
                selected={row.node.key === selectedKey}
                tabbable={index === tabIndexRow}
                onSelect={() => onSelect(row.node.key)}
                onToggle={() => onToggle(row.node.key)}
              />
            );
          })}
        </div>
      </div>
    </div>
  );
};
