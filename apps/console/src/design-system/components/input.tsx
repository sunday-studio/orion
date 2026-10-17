import { Input as InputPrimitive } from "@base-ui/react/input";
import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export type InputProps = React.ComponentProps<typeof InputPrimitive>;

export const Input = ({ className, ...props }: InputProps) => (
  <InputPrimitive
    data-slot="ds-input"
    className={cn(
      controlBase,
      "h-9 w-full min-w-0 rounded-lg border border-input-border bg-card px-3 text-[13px] text-ink placeholder:text-faint aria-invalid:border-destructive",
      className,
    )}
    {...props}
  />
);
