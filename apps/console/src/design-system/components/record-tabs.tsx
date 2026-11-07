import { Tabs as TabsPrimitive } from "@base-ui/react/tabs";
import type * as React from "react";
import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export type RecordTab = {
  value: string;
  label: React.ReactNode;
  /** Small count after the label, for example the number of detected items. */
  count?: number;
};

type RecordTabsProps = Omit<React.ComponentProps<typeof TabsPrimitive.Root>, "children"> & {
  tabs: readonly RecordTab[];
  /** The record's own panels (TabsPanel), or nothing when the page renders its sections itself. */
  children?: React.ReactNode;
  "aria-label"?: string;
};

/**
 * Tabs for the sections of one record (a server, an app, a status page, a service).
 * Left-aligned under the title with a hairline rule and a 2px underline on the active tab
 * (canvas "Inside a record"). List pages never use it: sibling pages are sidebar links.
 */
export const RecordTabs = ({
  tabs,
  children,
  className,
  "aria-label": ariaLabel = "Sections",
  ...props
}: RecordTabsProps) => (
  <TabsPrimitive.Root className={cn("flex flex-col gap-3.5", className)} {...props}>
    <TabsPrimitive.List
      activateOnFocus
      aria-label={ariaLabel}
      className="relative flex w-full items-stretch justify-start gap-6 self-stretch border-b border-hairline"
    >
      {tabs.map((tab) => (
        <TabsPrimitive.Tab
          key={tab.value}
          value={tab.value}
          className={cn(
            focusRing,
            "relative whitespace-nowrap rounded px-0 pb-2.5 pt-2 text-[13.5px] text-quiet hover:text-ink data-[active]:font-medium data-[active]:text-ink",
          )}
        >
          {tab.label}
          {tab.count !== undefined && tab.count > 0 && (
            <span className="ml-1.5 rounded-full bg-tray px-1.5 text-[11px] font-medium leading-4 text-quiet">
              {tab.count}
            </span>
          )}
        </TabsPrimitive.Tab>
      ))}
      <TabsPrimitive.Indicator className="absolute bottom-[-1px] left-0 h-0.5 w-[var(--active-tab-width)] translate-x-[var(--active-tab-left)] rounded-sm bg-ink transition-[translate,width] duration-[380ms] ease-[cubic-bezier(0.32,0.72,0,1)] motion-reduce:transition-none" />
    </TabsPrimitive.List>
    {children}
  </TabsPrimitive.Root>
);
