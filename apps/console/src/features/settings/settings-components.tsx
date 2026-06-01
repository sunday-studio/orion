import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

export const Field = ({
  label,
  children,
  description,
  error,
}: {
  label: string;
  children: ReactNode;
  description?: string;
  error?: string;
}) => (
  <label className="block space-y-1">
    <span className="text-sm font-medium">{label}</span>
    {children}
    {description && <span className="block text-sm text-neutral-600">{description}</span>}
    {error && <span className="block text-sm text-red-700">{error}</span>}
  </label>
);

export const Section = ({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) => (
  <section className="space-y-3 border-t border-neutral-200 pt-5">
    <div className="space-y-1">
      <h2 className="text-sm font-medium">{title}</h2>
      {description && <p className="text-sm text-neutral-600">{description}</p>}
    </div>
    {children}
  </section>
);

export const ActivityItem = ({
  label,
  value,
  detail,
  tone = "neutral",
}: {
  label: string;
  value: string;
  detail?: ReactNode;
  tone?: "neutral" | "success" | "error" | "pending";
}) => (
  <div
    className={cn(
      "space-y-1 border-l-2 bg-neutral-50 px-3 py-2 text-sm",
      tone === "success" && "border-emerald-500",
      tone === "error" && "border-red-500",
      tone === "pending" && "border-amber-500",
      tone === "neutral" && "border-neutral-300",
    )}
  >
    <div className="text-neutral-600">{label}</div>
    <div className="font-medium">{value}</div>
    {detail && <div className="text-neutral-600">{detail}</div>}
  </div>
);
