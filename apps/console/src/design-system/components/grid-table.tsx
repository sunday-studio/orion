import type * as React from "react";
import { useFlip } from "@/design-system/use-flip";
import { cn } from "@/utils/cn";

/** Rows built on a CSS grid with a shared column template (canvas `.gt`). */
export const GridTable = ({ className, ...props }: React.ComponentProps<"div">) => {
  const ref = useFlip<HTMLDivElement>();
  return <div ref={ref} role="table" className={cn("bg-card text-[13px]", className)} {...props} />;
};

export const GridRow = ({
  columns,
  header = false,
  flipId,
  className,
  style,
  ...props
}: React.ComponentProps<"div"> & {
  columns: string;
  header?: boolean;
  /** Stable row id so the row glides when the list changes. */
  flipId?: string;
}) => (
  <div
    role="row"
    data-flip-id={flipId}
    style={{ gridTemplateColumns: columns, ...style }}
    className={cn(
      "grid items-center gap-3 border-t border-line-soft px-3.5 first:border-t-0 [&>*]:min-w-0 [&>*]:truncate",
      header
        ? "h-[30px] bg-sub text-[11.5px] font-medium text-quiet"
        : "h-[38px] transition-colors duration-150 hover:bg-sub",
      className,
    )}
    {...props}
  />
);

export const GridFooter = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    className={cn(
      "border-t border-line-soft px-3.5 py-2 text-center text-[12.5px] text-quiet",
      className,
    )}
    {...props}
  />
);
