import { type VariantProps, cva } from "class-variance-authority";
import type * as React from "react";
import { cn } from "@/utils/cn";

const badgeStyles = cva("inline-flex w-fit items-center gap-1.5 rounded px-1.5 text-xs leading-5", {
  variants: {
    tone: {
      up: "bg-[#ccfbf1] text-[#115e59]",
      down: "bg-[#ffe4e6] text-[#9f1239]",
      degraded: "bg-[#fef3c7] text-[#92400e]",
      unknown: "bg-[#f5f5f4] text-[#44403c]",
      maintenance: "bg-[#ede9fe] text-[#5b21b6]",
      stale: "bg-[#f1f5f9] text-[#334155]",
      covered: "bg-[#cffafe] text-[#155e75]",
      resolved: "bg-[#dbeafe] text-[#1e40af]",
      critical: "bg-[#f43f5e] text-white",
      medium: "bg-[#fef9c3] text-[#854d0e]",
    },
  },
  defaultVariants: { tone: "unknown" },
});

export type BadgeTone = NonNullable<VariantProps<typeof badgeStyles>["tone"]>;
type Shape = "dot" | "square" | "hollow" | "half" | "pause" | "none";

const defaultShape: Record<BadgeTone, Shape> = {
  up: "dot",
  down: "square",
  degraded: "half",
  unknown: "hollow",
  maintenance: "pause",
  stale: "hollow",
  covered: "dot",
  resolved: "dot",
  critical: "square",
  medium: "dot",
};

const Glyph = ({ shape }: { shape: Shape }) => {
  if (shape === "none") return null;
  if (shape === "pause") {
    return (
      <span aria-hidden className="flex h-2 gap-px">
        <span className="w-0.5 bg-current" />
        <span className="w-0.5 bg-current" />
      </span>
    );
  }
  return (
    <span
      aria-hidden
      className={cn(
        "size-1.5 shrink-0 bg-current",
        (shape === "dot" || shape === "half") && "rounded-full",
        shape === "hollow" && "rounded-full bg-transparent ring-1 ring-current",
        shape === "half" &&
          "bg-[linear-gradient(90deg,currentColor_50%,transparent_50%)] ring-1 ring-current",
      )}
    />
  );
};

export type BadgeProps = React.ComponentProps<"span"> &
  VariantProps<typeof badgeStyles> & { shape?: Shape };

/** Label plus glyph shape, so status is never conveyed by colour alone. */
export const Badge = ({ tone, shape, className, children, ...props }: BadgeProps) => (
  <span data-slot="ds-badge" className={cn(badgeStyles({ tone }), className)} {...props}>
    <Glyph shape={shape ?? defaultShape[tone ?? "unknown"]} />
    {children}
  </span>
);

const statusTone: Record<string, BadgeTone> = {
  up: "up",
  sent: "up",
  down: "down",
  failed: "down",
  open: "down",
  degraded: "degraded",
  warning: "degraded",
  acknowledged: "degraded",
  unknown: "unknown",
  pending: "unknown",
  draft: "unknown",
  maintenance: "maintenance",
  paused: "maintenance",
  stale: "stale",
  disabled: "stale",
  suppressed: "stale",
  covered: "covered",
  cooldown: "covered",
  resolved: "resolved",
  critical: "critical",
  high: "down",
  medium: "medium",
  low: "up",
};

export const toneForStatus = (status?: string | null): BadgeTone =>
  statusTone[status?.trim().toLowerCase() ?? ""] ?? "unknown";

export const StatusBadge = ({
  status,
  children,
  ...props
}: Omit<BadgeProps, "tone"> & { status?: string | null }) => (
  <Badge tone={toneForStatus(status)} {...props}>
    {children ?? status ?? "unknown"}
  </Badge>
);
