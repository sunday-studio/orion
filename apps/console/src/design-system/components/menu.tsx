import { Menu as MenuPrimitive } from "@base-ui/react/menu";
import type * as React from "react";
import { cn } from "@/utils/cn";

export const Menu = MenuPrimitive.Root;
export const MenuTrigger = MenuPrimitive.Trigger;

export const MenuContent = ({
  className,
  align = "end",
  sideOffset = 6,
  ...props
}: React.ComponentProps<typeof MenuPrimitive.Popup> &
  Pick<React.ComponentProps<typeof MenuPrimitive.Positioner>, "align" | "sideOffset">) => (
  <MenuPrimitive.Portal>
    <MenuPrimitive.Positioner align={align} sideOffset={sideOffset} className="z-50">
      <MenuPrimitive.Popup
        data-slot="ds-menu"
        className={cn(
          "min-w-44 rounded-[10px] border border-hairline bg-card p-1 text-[13px] shadow-overlay outline-none transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0",
          className,
        )}
        {...props}
      />
    </MenuPrimitive.Positioner>
  </MenuPrimitive.Portal>
);

export const MenuItem = ({
  className,
  ...props
}: React.ComponentProps<typeof MenuPrimitive.Item>) => (
  <MenuPrimitive.Item
    className={cn(
      "flex cursor-default select-none items-center gap-2 rounded-md px-2 py-1.5 outline-none data-[disabled]:opacity-50 data-[highlighted]:bg-sub",
      className,
    )}
    {...props}
  />
);

export const MenuSeparator = ({
  className,
  ...props
}: React.ComponentProps<typeof MenuPrimitive.Separator>) => (
  <MenuPrimitive.Separator className={cn("my-1 h-px bg-hairline", className)} {...props} />
);
