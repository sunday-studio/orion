import { motion, useReducedMotion } from "motion/react";
import { useId } from "react";
import { cn } from "@/utils/cn";

/** Spring used for shared-layout moves: smooth, no bounce (see docs/web/motion.md). */
export const layoutSpring = { type: "spring", duration: 0.45, bounce: 0 } as const;

/**
 * The sliding highlight. Render it inside the active item with a `layoutId` shared by the
 * whole group and Motion moves it between items instead of fading one out and one in.
 */
export const SegmentThumb = ({ layoutId, className }: { layoutId: string; className?: string }) => {
  const reduce = useReducedMotion();
  return (
    <motion.span
      aria-hidden
      layoutId={layoutId}
      transition={reduce ? { duration: 0 } : layoutSpring}
      className={cn(
        "pointer-events-none absolute inset-0 rounded-md bg-chip shadow-[0_1px_2px_rgba(0,0,0,0.08)] dark:shadow-[0_0_0_1px_rgba(255,255,255,0.07)]",
        className,
      )}
    />
  );
};

export type SegmentedOption = { value: string; label: React.ReactNode };

/** Radio-style segmented control with a sliding thumb (canvas `.sw2`). */
export const Segmented = ({
  options,
  value,
  onValueChange,
  size = "md",
  className,
  "aria-label": ariaLabel,
  role = "radiogroup",
  surface = "page",
}: {
  options: readonly SegmentedOption[];
  value: string;
  onValueChange: (value: string) => void;
  size?: "sm" | "md";
  className?: string;
  "aria-label": string;
  /** Use "group" with aria-pressed items for view switchers rather than form choices. */
  role?: "radiogroup" | "group";
  /**
   * What the control sits on, so the tray never blends in: "page" (default), "card" for
   * dialogs and popovers, "field" for tinted headers and field panels.
   */
  surface?: "page" | "card" | "field";
}) => {
  const layoutId = useId();
  return (
    <div
      role={role}
      aria-label={ariaLabel}
      className={cn(
        "relative flex w-fit gap-0.5 rounded-lg p-[3px]",
        surface === "field" && "bg-hairline dark:bg-tray",
        surface === "card" && "bg-tray dark:bg-page",
        surface === "page" && "bg-tray",
        className,
      )}
    >
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role={role === "radiogroup" ? "radio" : undefined}
            aria-checked={role === "radiogroup" ? active : undefined}
            aria-pressed={role === "group" ? active : undefined}
            // Mouse clicks should not leave a focus ring; keyboard focus still shows it.
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onValueChange(option.value)}
            className={cn(
              "relative whitespace-nowrap rounded-md px-3 py-1 outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus/60",
              size === "sm" ? "text-xs" : "text-[13px]",
              active ? "text-ink" : "text-mid hover:text-ink",
            )}
          >
            {active && <SegmentThumb layoutId={layoutId} />}
            <span className="relative">{option.label}</span>
          </button>
        );
      })}
    </div>
  );
};
