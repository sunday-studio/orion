import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import type * as React from "react";
import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Tabs = ({ className, ...props }: React.ComponentProps<typeof TabsPrimitive.Root>) => (
  <TabsPrimitive.Root className={cn("flex flex-col gap-4", className)} {...props} />
);

/** Left-aligned tab row with a full-width rule and a sliding underline. */
export const TabsList = ({
  className,
  children,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.List>) => (
  <TabsPrimitive.List
    activateOnFocus
    className={cn(
      "relative flex h-9 w-full items-center gap-5 self-stretch border-b border-hairline",
      className,
    )}
    {...props}
  >
    {children}
    <TabsPrimitive.Indicator className="absolute bottom-[-1px] left-0 h-0.5 w-[var(--active-tab-width)] translate-x-[var(--active-tab-left)] bg-brand transition-[translate,width] duration-200 ease-out motion-reduce:transition-none" />
  </TabsPrimitive.List>
);

export const TabsTab = ({
  className,
  ...props
}: React.ComponentProps<typeof TabsPrimitive.Tab>) => (
  <TabsPrimitive.Tab
    className={cn(
      focusRing,
      "relative h-full whitespace-nowrap rounded text-[13px] text-quiet hover:text-ink data-[active]:text-ink",
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
