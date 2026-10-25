import { Checkbox as CheckboxPrimitive } from "@base-ui/react/checkbox";
import { CheckIcon } from "lucide-react";
import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Checkbox = ({
  className,
  ...props
}: React.ComponentProps<typeof CheckboxPrimitive.Root>) => (
  <CheckboxPrimitive.Root
    data-slot="ds-checkbox"
    className={cn(
      controlBase,
      "flex size-4 shrink-0 items-center justify-center rounded border border-input-border bg-card text-white data-[checked]:border-brand data-[checked]:bg-brand",
      className,
    )}
    {...props}
  >
    <CheckboxPrimitive.Indicator className="flex">
      <CheckIcon className="size-3" />
    </CheckboxPrimitive.Indicator>
  </CheckboxPrimitive.Root>
);
