import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Textarea = ({ className, ...props }: React.ComponentProps<"textarea">) => (
  <textarea
    data-slot="ds-textarea"
    className={cn(
      controlBase,
      "flex min-h-16 w-full min-w-0 rounded-lg border border-input-border bg-card px-3 py-2 text-[13px] text-ink placeholder:text-faint aria-invalid:border-destructive",
      className,
    )}
    {...props}
  />
);
