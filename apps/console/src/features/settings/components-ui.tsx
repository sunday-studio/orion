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

/** One row of a raised list: label with its detail underneath, controls on the right. */
export const SettingRow = ({
  label,
  detail,
  children,
}: {
  label: ReactNode;
  detail?: ReactNode;
  children?: ReactNode;
}) => (
  <div className="flex min-h-[44px] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0">
    <div className="min-w-0 flex-1">
      <b className="block font-medium">{label}</b>
      {detail && <span className="mt-0.5 block text-xs leading-[1.45] text-faint">{detail}</span>}
    </div>
    {children && <div className="shrink-0 text-right">{children}</div>}
  </div>
);

/**
 * A settings section without a card around it (canvas `.xsec`): the title and a one-line hint on
 * the left, the controls on the right, hairlines between sections.
 */
export const SettingsSection = ({
  title,
  hint,
  children,
}: {
  title: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
}) => (
  <section className="grid grid-cols-[200px_minmax(0,1fr)] gap-7 border-t border-hairline py-[22px] first:border-t-0 first:pt-1.5">
    <div>
      <h2 className="text-sm font-medium">{title}</h2>
      {hint && <p className="mt-1 text-[12.5px] leading-normal text-quiet">{hint}</p>}
    </div>
    <div className="flex min-w-0 flex-col gap-3">{children}</div>
  </section>
);

/** The save row under the last section. */
export const SettingsFooter = ({ children }: { children: ReactNode }) => (
  <div className="flex justify-end border-t border-hairline pt-[18px]">{children}</div>
);
