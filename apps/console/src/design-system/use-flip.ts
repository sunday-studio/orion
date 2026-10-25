import { useLayoutEffect, useRef } from "react";

const reduced = () => window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/**
 * Rows that are new in a list fade in softly (opacity only: nothing moves up or down).
 * Children opt in with `data-flip-id`. The first set of rows after a load never animates;
 * only a change from one list to another does.
 */
export const useFlip = <T extends HTMLElement>() => {
  const ref = useRef<T>(null);
  const previous = useRef<Set<string>>(new Set());

  useLayoutEffect(() => {
    const root = ref.current;
    if (!root) return;
    const nodes = Array.from(root.querySelectorAll<HTMLElement>(":scope > [data-flip-id]"));
    const next = new Set(nodes.map((node) => node.dataset.flipId ?? ""));
    if (previous.current.size > 0 && !reduced()) {
      for (const node of nodes) {
        if (previous.current.has(node.dataset.flipId ?? "")) continue;
        node.animate([{ opacity: 0 }, { opacity: 1 }], {
          duration: 260,
          easing: "cubic-bezier(0.22, 1, 0.36, 1)",
        });
      }
    }
    previous.current = next;
  });

  return ref;
};
