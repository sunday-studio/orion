import { useLayoutEffect, useRef } from "react";

const EASE = "cubic-bezier(0.22, 1, 0.36, 1)";
const MOVE_MS = 380;
const ENTER_MS = 300;

const reduced = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * Shared layout motion for lists: children that carry `data-flip-id` glide to their new
 * position when siblings are added, removed or reordered, and new children fade in softly.
 * Nothing animates on first render. Uses the Web Animations API, so a new change
 * retargets from the current position instead of snapping.
 */
export const useFlip = <T extends HTMLElement>() => {
  const ref = useRef<T>(null);
  const previous = useRef<Map<string, number>>(new Map());
  const mounted = useRef(false);

  useLayoutEffect(() => {
    const root = ref.current;
    if (!root) return;
    const next = new Map<string, number>();
    // Positions are relative to the container so page scrolling never looks like movement.
    const origin = root.getBoundingClientRect().top;
    const nodes = Array.from(root.querySelectorAll<HTMLElement>(":scope > [data-flip-id]"));
    for (const node of nodes)
      next.set(node.dataset.flipId ?? "", node.getBoundingClientRect().top - origin);

    if (mounted.current && !reduced()) {
      for (const node of nodes) {
        const id = node.dataset.flipId ?? "";
        const before = previous.current.get(id);
        const after = next.get(id) ?? 0;
        if (before === undefined) {
          node.animate(
            [
              { opacity: 0, transform: "translateY(4px)" },
              { opacity: 1, transform: "none" },
            ],
            { duration: ENTER_MS, easing: EASE },
          );
        } else if (Math.abs(before - after) > 1) {
          node.animate([{ transform: `translateY(${before - after}px)` }, { transform: "none" }], {
            duration: MOVE_MS,
            easing: EASE,
          });
        }
      }
    }
    mounted.current = true;
    previous.current = next;
  });

  return ref;
};
