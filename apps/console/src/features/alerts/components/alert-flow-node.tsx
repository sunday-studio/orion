import { cn } from "@/utils/cn";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

export const FlowNode = ({
  children,
  icon: Icon,
  tone = "neutral",
  title,
}: {
  children: ReactNode;
  icon: LucideIcon;
  tone?: "neutral" | "warning" | "success";
  title: string;
}) => (
  <div
    className={cn(
      "relative min-h-28 border bg-white p-4 shadow-xs",
      tone === "warning" && "border-amber-300 bg-amber-50",
      tone === "success" && "border-emerald-300 bg-emerald-50",
      tone === "neutral" && "border-neutral-300",
    )}
  >
    <div className="flex items-center gap-2 text-sm font-medium">
      <Icon className="size-4" />
      {title}
    </div>
    <div className="mt-3 text-sm text-neutral-600">{children}</div>
  </div>
);

export const Connector = () => (
  <div className="hidden h-px bg-neutral-300 md:block" aria-hidden="true" />
);
