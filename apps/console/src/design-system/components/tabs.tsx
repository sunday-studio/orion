import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import type * as React from "react";
import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Tabs = ({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Root>) => (
  <TabsPrimitive.Root className={cn("flex flex-col gap-4", className)} {...props} />
);

/** Centered tab row; the rule sits only under the tabs (design language, Layout). */
export const TabsList = ({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List>) => (
  <TabsPrimitive.List
    activateOnFocus
    className={cn(
      "inline-flex h-9 w-max items-center gap-5 self-center border-b border-hairline",
      className,
    )}
    {...props}
  />
);

export const TabsTab = ({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Tab>) => (
  <TabsPrimitive.Tab
    className={cn(
      focusRing,
      "relative h-full whitespace-nowrap rounded px-0.5 text-[13px] text-quiet hover:text-ink data-[active]:font-medium data-[active]:text-ink after:absolute after:inset-x-0 after:bottom-[-1px] after:h-0.5 after:bg-brand after:opacity-0 data-[active]:after:opacity-100",
      className,
    )}
    {...props}
  />
);

export const TabsPanel = ({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Panel>) => (
  <TabsPrimitive.Panel className={cn("outline-none", className)} {...props} />
);

export const TabCount = ({ className, children, ...props }: React.ComponentProps<"span">) => (
  <span className={cn("ml-1.5 text-xs text-faint", className)} {...props}>
    ({children})
  </span>
);
