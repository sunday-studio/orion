import { type VariantProps, cva } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/utils/cn";

const calloutStyles = cva("block rounded-lg px-3.5 py-2.5 text-[13px] leading-[1.5]", {
  variants: {
    tone: {
      down: "bg-[var(--tone-down-bg)] text-[var(--tone-down-fg)]",
      up: "bg-[var(--tone-up-bg)] text-[var(--tone-up-fg)]",
      warn: "bg-[var(--tone-degraded-bg)] text-[var(--tone-degraded-fg)]",
    },
  },
  defaultVariants: { tone: "warn" },
});

/** Inline banner: a bold lead-in followed by one sentence (canvas `.callout`). */
export const Callout = ({
  tone,
  lead,
  className,
  children,
  ...props
}: Omit<React.ComponentProps<"div">, "title"> &
  VariantProps<typeof calloutStyles> & { lead?: React.ReactNode }) => (
  <div
    role={tone === "down" ? "alert" : "status"}
    className={cn(calloutStyles({ tone }), className)}
    {...props}
  >
    {lead && <b className="mr-1.5 font-semibold">{lead}</b>}
    {children}
  </div>
);
