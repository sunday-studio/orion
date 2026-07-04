import * as React from "react";

import { cn } from "@/utils/cn";

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "flex min-h-20 w-full min-w-0 border border-neutral-300 bg-transparent px-3 py-2 text-sm shadow-xs outline-none placeholder:text-muted-foreground",
        "focus-visible:border-ring focus-visible:ring-neutral-200 focus-visible:ring-[3px]",
        "disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive/20",
        className,
      )}
      {...props}
    />
  );
}

export { Textarea };
