import { type PointerEvent, type ReactNode, useCallback, useRef, useState } from "react";
import { cn } from "@/utils/cn";

/**
 * Pointer tracking for a plotted line: returns the index of the nearest point and the
 * handlers to put on an element covering the plot area.
 */
export const usePlotHover = (count: number) => {
  const ref = useRef<HTMLDivElement>(null);
  const [index, setIndex] = useState<number | null>(null);
  const onPointerMove = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      const box = ref.current?.getBoundingClientRect();
      if (!box || box.width === 0 || count < 1) return;
      const ratio = Math.min(Math.max((event.clientX - box.left) / box.width, 0), 1);
      setIndex(count === 1 ? 0 : Math.round(ratio * (count - 1)));
    },
    [count],
  );
  return {
    index,
    bind: { ref, onPointerMove, onPointerLeave: () => setIndex(null) },
  };
};

/** Vertical guide and dot on the hovered point; positions are percentages of the plot. */
export const ChartMarker = ({
  x,
  y,
  color = "#2a9f69",
}: {
  x: number;
  y: number;
  color?: string;
}) => (
  <>
    <span
      aria-hidden
      className="pointer-events-none absolute inset-y-0 w-px -translate-x-1/2 bg-input-border"
      style={{ left: `${x}%` }}
    />
    <span
      aria-hidden
      className="pointer-events-none absolute size-2.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-card"
      style={{ left: `${x}%`, top: `${y}%`, background: color }}
    />
  </>
);

/** Floating readout in the design-system tooltip style; flips to the left near the right edge. */
export const ChartTooltip = ({
  x,
  className,
  children,
}: {
  x: number;
  className?: string;
  children: ReactNode;
}) => (
  <div
    role="tooltip"
    className={cn(
      "pointer-events-none absolute top-0 z-10 whitespace-nowrap rounded-md bg-ink px-2 py-1 text-xs leading-[1.5] text-page shadow-overlay",
      className,
    )}
    style={
      x > 70
        ? { left: `${x}%`, transform: "translate(calc(-100% - 10px), 0)" }
        : { left: `${x}%`, transform: "translate(10px, 0)" }
    }
  >
    {children}
  </div>
);
