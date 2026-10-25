import type { ReactNode } from "react";
import { cn } from "@/utils/cn";

/** Label above a control, with optional helper text and an error line. */
export const Labeled = ({
  label,
  hint,
  error,
  className,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  className?: string;
  children: ReactNode;
}) => (
  <label className={cn("grid content-start gap-1.5", className)}>
    <span className="text-[12.5px] font-medium">{label}</span>
    {children}
    {hint && <small className="text-xs text-quiet">{hint}</small>}
    {error && <small className="text-xs text-[var(--tone-down-fg)]">{error}</small>}
  </label>
);

export const SettingRow = ({
  label,
  detail,
  children,
}: {
  label: ReactNode;
  detail?: ReactNode;
  children?: ReactNode;
}) => (
  <div className="grid min-h-[44px] grid-cols-[180px_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0">
    <b className="font-medium">{label}</b>
    <span className="min-w-0 truncate text-quiet">{detail}</span>
    <span className="text-right">{children}</span>
  </div>
);
