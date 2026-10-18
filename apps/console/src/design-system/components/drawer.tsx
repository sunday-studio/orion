import { Drawer as DrawerPrimitive } from "@base-ui/react/drawer";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Drawer = DrawerPrimitive.Root;
export const DrawerTrigger = DrawerPrimitive.Trigger;
export const DrawerClose = DrawerPrimitive.Close;

/** Right-side detail panel (460 to 480px), for report and log detail. */
export const DrawerContent = ({
  className,
  children,
  ...props
}: React.ComponentProps<typeof DrawerPrimitive.Popup>) => (
  <DrawerPrimitive.Portal>
    <DrawerPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/30 transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
    <DrawerPrimitive.Viewport>
      <DrawerPrimitive.Popup
        data-slot="ds-drawer"
        className={cn(
          "fixed inset-y-0 right-0 z-50 flex w-[480px] max-w-full flex-col gap-4 overflow-y-auto border-l border-hairline bg-card p-6 shadow-overlay outline-none transition-transform duration-[320ms] ease-[cubic-bezier(0.32,0.72,0,1)] data-[ending-style]:translate-x-full data-[starting-style]:translate-x-full",
          className,
        )}
        {...props}
      >
        {children}
        <DrawerPrimitive.Close
          aria-label="Close"
          className={cn(focusRing, "absolute right-4 top-4 rounded p-1 text-quiet hover:text-ink")}
        >
          <XIcon className="size-4" />
        </DrawerPrimitive.Close>
      </DrawerPrimitive.Popup>
    </DrawerPrimitive.Viewport>
  </DrawerPrimitive.Portal>
);

export const DrawerTitle = ({
  className,
  ...props
}: React.ComponentProps<typeof DrawerPrimitive.Title>) => (
  <DrawerPrimitive.Title className={cn("pr-8 text-[15px] font-medium", className)} {...props} />
);

export const DrawerDescription = ({
  className,
  ...props
}: React.ComponentProps<typeof DrawerPrimitive.Description>) => (
  <DrawerPrimitive.Description className={cn("text-[13px] text-quiet", className)} {...props} />
);
