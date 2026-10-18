import { type ReactNode, useLayoutEffect, useRef, useState } from "react";
import { cn } from "@/utils/cn";

/**
 * Animates its height as the content changes size, so swapping a form between types
 * glides instead of jumping. Content is measured with a ResizeObserver.
 */
export const AnimatedHeight = ({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) => {
  const inner = useRef<HTMLDivElement>(null);
  const [height, setHeight] = useState<number>();
  const [ready, setReady] = useState(false);

  useLayoutEffect(() => {
    const node = inner.current;
    if (!node) return;
    const measure = () => setHeight(node.offsetHeight);
    measure();
    const frame = requestAnimationFrame(() => setReady(true));
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, []);

  return (
    <div
      className={cn(
        "overflow-hidden",
        ready &&
          "transition-[height] duration-[380ms] ease-[cubic-bezier(0.32,0.72,0,1)] motion-reduce:transition-none",
        className,
      )}
      style={height === undefined ? undefined : { height }}
    >
      <div ref={inner} className="flex flex-col gap-3.5">
        {children}
      </div>
    </div>
  );
};
