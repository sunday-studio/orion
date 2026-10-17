import type * as React from "react";
import { cn } from "@/utils/cn";

export type StepRowTone = "done" | "pending" | "idle";

export type StepRowData = {
  title: React.ReactNode;
  detail?: React.ReactNode;
  tone: StepRowTone;
};

const dotColor: Record<StepRowTone, string> = {
  done: "#2a9f69",
  pending: "#2a9f69",
  idle: "var(--color-input-border)",
};

/** A card of one-line rows: status dot, bold title, muted detail column (canvas `.lr`). */
export const StepRows = ({
  rows,
  className,
}: {
  rows: readonly StepRowData[];
  className?: string;
}) => (
  <ul className={cn("m-0 list-none p-0", className)}>
    {rows.map((row, index) => (
      <li
        key={index}
        className="flex items-center gap-3 border-b border-line-soft px-3.5 py-[9px] text-[13px] last:border-b-0"
      >
        {row.tone === "pending" ? (
          <span
            aria-hidden
            className="ml-px size-1.5 shrink-0 animate-pulse rounded-full motion-reduce:animate-none"
            style={{ background: dotColor.pending }}
          />
        ) : (
          <span
            aria-hidden
            className="inline-block size-2 shrink-0 rounded-full"
            style={{ background: dotColor[row.tone] }}
          />
        )}
        <div className="grid min-w-0 flex-1 grid-cols-[minmax(120px,240px)_minmax(0,1fr)] items-center gap-4">
          <b className="truncate font-medium">{row.title}</b>
          {row.detail !== undefined && (
            <small className="truncate text-xs text-faint">{row.detail}</small>
          )}
        </div>
      </li>
    ))}
  </ul>
);
