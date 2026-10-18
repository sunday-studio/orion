import { type RefObject, useLayoutEffect, useRef, useState } from "react";
import { cn } from "@/utils/cn";

type Thumb = { left: number; width: number } | null;

/**
 * Measures the active item inside `container` (the element with `data-segment-active`) so a
 * single thumb can slide under it. Weight never changes with state, so nothing reflows.
 */
export const useSlidingThumb = (container: RefObject<HTMLElement | null>, key: unknown) => {
  const [thumb, setThumb] = useState<Thumb>(null);
  const [ready, setReady] = useState(false);

  useLayoutEffect(() => {
    const root = container.current;
    if (!root) return;
    const measure = () => {
      const active = root.querySelector<HTMLElement>("[data-segment-active]");
      setThumb(active ? { left: active.offsetLeft, width: active.offsetWidth } : null);
    };
    measure();
    const frame = requestAnimationFrame(() => setReady(true));
    const observer = new ResizeObserver(measure);
    observer.observe(root);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, [container, key]);

  return { thumb, ready };
};

export const SegmentThumb = ({
  thumb,
  ready,
  className,
}: {
  thumb: Thumb;
  ready: boolean;
  className?: string;
}) =>
  thumb && (
    <span
      aria-hidden
      className={cn(
        "pointer-events-none absolute bottom-[3px] top-[3px] rounded-md bg-chip shadow-[0_1px_2px_rgba(0,0,0,0.08)]",
        ready &&
          "transition-[left,width] duration-[380ms] ease-[cubic-bezier(0.32,0.72,0,1)] motion-reduce:transition-none",
        className,
      )}
      style={{ left: thumb.left, width: thumb.width }}
    />
  );

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
}: {
  options: readonly SegmentedOption[];
  value: string;
  onValueChange: (value: string) => void;
  size?: "sm" | "md";
  className?: string;
  "aria-label": string;
  /** Use "group" with aria-pressed items for view switchers rather than form choices. */
  role?: "radiogroup" | "group";
}) => {
  const ref = useRef<HTMLDivElement>(null);
  const { thumb, ready } = useSlidingThumb(ref, `${value}|${options.map((o) => o.value).join()}`);
  return (
    <div
      ref={ref}
      role={role}
      aria-label={ariaLabel}
      className={cn("relative flex w-fit gap-0.5 rounded-lg bg-tray p-[3px]", className)}
    >
      <SegmentThumb thumb={thumb} ready={ready} />
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role={role === "radiogroup" ? "radio" : undefined}
            aria-checked={role === "radiogroup" ? active : undefined}
            aria-pressed={role === "group" ? active : undefined}
            data-segment-active={active ? "" : undefined}
            onClick={() => onValueChange(option.value)}
            className={cn(
              "outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-focus/60",
              "relative z-[1] whitespace-nowrap rounded-md px-3 py-1 transition-colors",
              size === "sm" ? "text-xs" : "text-[13px]",
              active ? "text-ink" : "text-mid hover:text-ink",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
};
