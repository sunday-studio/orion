import type * as React from "react";
import { cn } from "@/utils/cn";

export const Skeleton = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    data-slot="ds-skeleton"
    aria-hidden
    className={cn("animate-pulse rounded-md bg-hairline motion-reduce:animate-none", className)}
    {...props}
  />
);
