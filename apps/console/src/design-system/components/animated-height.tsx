import { motion, useReducedMotion } from "motion/react";
import { type ReactNode, useLayoutEffect, useRef, useState } from "react";
import { layoutSpring } from "@/design-system/components/segmented";
import { cn } from "@/utils/cn";

/**
 * Animates its height with a spring as the content changes size, so swapping a form between
 * types glides instead of jumping. Content is measured with a ResizeObserver.
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
  const reduce = useReducedMotion();

  useLayoutEffect(() => {
    const node = inner.current;
    if (!node) return;
    const measure = () => setHeight(node.offsetHeight);
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return (
    <motion.div
      className={cn("overflow-hidden", className)}
      initial={false}
      animate={height === undefined ? undefined : { height }}
      transition={reduce ? { duration: 0 } : layoutSpring}
    >
      <div ref={inner} className="flex flex-col gap-3.5">
        {children}
      </div>
    </motion.div>
  );
};
