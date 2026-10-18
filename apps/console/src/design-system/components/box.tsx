import type * as React from "react";
import { cn } from "@/utils/cn";

/** Bordered panel with a quiet header bar (canvas `.box` / `.bh`). */
export const Box = ({ className, ...props }: React.ComponentProps<"section">) => (
  <section
    data-slot="ds-box"
    className={cn(
      "overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
      className,
    )}
    {...props}
  />
);

export const BoxHeader = ({
  title,
  aside,
  className,
}: {
  title: React.ReactNode;
  aside?: React.ReactNode;
  className?: string;
}) => (
  <div
    className={cn(
      "flex items-center justify-between border-b border-hairline bg-field px-3.5 py-2 text-[13px] font-medium",
      className,
    )}
  >
    <h2 className="text-[13px] font-medium">{title}</h2>
    {aside !== undefined && <span className="font-normal text-faint">{aside}</span>}
  </div>
);

/** Pale container holding raised cards (canvas `.field`). */
export const FieldPanel = ({
  title,
  aside,
  children,
  className,
  ...props
}: Omit<React.ComponentProps<"section">, "title"> & {
  title: React.ReactNode;
  aside?: React.ReactNode;
}) => (
  <section
    className={cn(
      "rounded-[14px] border border-hairline bg-field p-5 dark:shadow-[inset_0_2px_6px_rgba(0,0,0,0.55)]",
      className,
    )}
    {...props}
  >
    <div className="mx-0.5 mb-3 flex items-baseline justify-between">
      <h2 className="text-sm font-medium">{title}</h2>
      {aside !== undefined && <span className="text-[12.5px] text-faint">{aside}</span>}
    </div>
    {children}
  </section>
);

/** White raised card used inside a FieldPanel (canvas `.pc`). */
export const RaisedCard = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    className={cn(
      "rounded-[10px] border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.08),0_4px_14px_-6px_rgba(0,0,0,0.06)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.035),0_8px_18px_-10px_rgba(0,0,0,0.7)]",
      className,
    )}
    {...props}
  />
);

/** Small number card: label above a large figure (canvas `.stat3 > div`). */
export const StatBox = ({
  label,
  value,
  of,
  className,
}: {
  label: string;
  value: React.ReactNode;
  of?: React.ReactNode;
  className?: string;
}) => (
  <div className={cn("rounded-xl border border-hairline bg-card px-3.5 py-3", className)}>
    <span className="block text-xs leading-[18px] text-quiet">{label}</span>
    <b className="block text-xl font-semibold leading-[30px] tracking-[-0.02em]">
      {value}
      {of !== undefined && (
        <small className="ml-1 text-[13px] font-normal text-faint tracking-normal">of {of}</small>
      )}
    </b>
  </div>
);

/** Status dot used at the start of rows (8px). */
export const Dot = ({ color, className }: { color: string; className?: string }) => (
  <span
    aria-hidden
    className={cn("inline-block size-2 shrink-0 rounded-full", className)}
    style={{ background: color }}
  />
);
