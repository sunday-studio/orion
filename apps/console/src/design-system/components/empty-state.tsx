import type * as React from "react";
import { cn } from "@/utils/cn";

export type EmptyStateProps = {
  title: string;
  description?: string;
  /** Every empty or error state ends in a button (design rule 5). */
  action?: React.ReactNode;
  tone?: "neutral" | "error";
  className?: string;
};

export const EmptyState = ({
  title,
  description,
  action,
  tone = "neutral",
  className,
}: EmptyStateProps) => (
  <div
    role={tone === "error" ? "alert" : undefined}
    className={cn(
      "flex min-h-40 flex-col items-center justify-center gap-3 border border-dashed px-4 py-8 text-center",
      tone === "error" ? "border-destructive/40" : "border-hairline",
      className,
    )}
  >
    <div className="space-y-1">
      <p className="text-sm font-medium">{title}</p>
      {description && <p className="text-xs text-quiet">{description}</p>}
    </div>
    {action}
  </div>
);
