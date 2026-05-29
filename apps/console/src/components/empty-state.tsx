import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

type EmptyStateProps = {
  title: string;
  action?: ReactNode;
  description?: string;
  className?: string;
  tone?: "neutral" | "error";
};

export const EmptyState = ({
  title,
  action,
  description,
  className,
  tone = "neutral",
}: EmptyStateProps) => {
  return (
    <div
      className={cn(
        "flex min-h-64 flex-col items-center justify-center px-4 py-10 text-center",
        tone === "error" && "border border-rose-200 bg-rose-50",
        className,
      )}
    >
      <div
        className={cn(
          "mb-4 h-10 w-10 bg-[radial-gradient(currentColor_1px,transparent_1px)] bg-[size:5px_5px] [clip-path:polygon(50%_0,100%_50%,50%_100%,0_50%)]",
          tone === "error" ? "text-rose-400" : "text-neutral-400",
        )}
      />
      <div className="space-y-1">
        <p className="font-medium text-neutral-900 text-sm">{title}</p>
        {description && <p className="max-w-sm text-neutral-500 text-sm">{description}</p>}
      </div>
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
};
