import type * as React from "react";
import { cn } from "@/utils/cn";

export const Card = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    data-slot="ds-card"
    className={cn("rounded-[10px] border border-hairline bg-card shadow-card", className)}
    {...props}
  />
);

/** Pale container that groups several cards or settings (design language, Layout). */
export const Field = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div
    data-slot="ds-field"
    className={cn("space-y-3 rounded-[14px] bg-field p-5", className)}
    {...props}
  />
);

export const CardHeader = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div className={cn("space-y-0.5 border-b border-hairline px-4 py-3", className)} {...props} />
);

export const CardTitle = ({ className, ...props }: React.ComponentProps<"h3">) => (
  <h3 className={cn("text-[15px] font-medium", className)} {...props} />
);

export const CardDescription = ({ className, ...props }: React.ComponentProps<"p">) => (
  <p className={cn("text-xs text-quiet", className)} {...props} />
);

export const CardContent = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div className={cn("p-4", className)} {...props} />
);
