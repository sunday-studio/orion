import * as React from "react";

import { cn } from "@/lib/utils";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "flex h-9 w-full min-w-0 border border-neutral-300 bg-transparent px-3 py-1 shadow-xs outline-none placeholder:text-muted-foreground text-sm",
        "focus-visible:border-ring focus-visible:ring-neutral-200 focus-visible:ring-[3px]",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
