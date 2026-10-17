import { Switch as SwitchPrimitive } from "@base-ui/react/switch";
import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Switch = ({
  className,
  ...props
}: React.ComponentProps<typeof SwitchPrimitive.Root>) => (
  <SwitchPrimitive.Root
    data-slot="ds-switch"
    className={cn(
      controlBase,
      "relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border border-input-border bg-sub p-0.5 data-[checked]:border-brand data-[checked]:bg-brand",
      className,
    )}
    {...props}
  >
    <SwitchPrimitive.Thumb className="block size-3.5 rounded-full bg-white shadow-card transition-transform data-[checked]:translate-x-4" />
  </SwitchPrimitive.Root>
);
