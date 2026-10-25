import { Tooltip as TooltipPrimitive } from "@base-ui/react/tooltip";
import type * as React from "react";
import { cn } from "@/utils/cn";

export type TooltipProps = {
  content: React.ReactNode;
  children: React.ReactElement;
  side?: "top" | "right" | "bottom" | "left";
};

/** Wraps a single focusable child; the label is also shown on keyboard focus. */
export const Tooltip = ({ content, children, side = "top" }: TooltipProps) => (
  <TooltipPrimitive.Root>
    <TooltipPrimitive.Trigger render={children} />
    <TooltipPrimitive.Portal>
      <TooltipPrimitive.Positioner side={side} sideOffset={6} className="z-50">
        <TooltipPrimitive.Popup
          className={cn(
            "rounded-md bg-ink px-2 py-1 text-xs text-page transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0",
          )}
        >
          {content}
        </TooltipPrimitive.Popup>
      </TooltipPrimitive.Positioner>
    </TooltipPrimitive.Portal>
  </TooltipPrimitive.Root>
);
