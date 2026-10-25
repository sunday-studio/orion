import type * as React from "react";
import { useFlip } from "@/design-system/use-flip";
import { cn } from "@/utils/cn";

/** Unstyled-by-default table primitives with design-language rows (38px, sticky 30px header). */
export const Table = ({ className, ...props }: React.ComponentProps<"table">) => (
  <table
    data-slot="ds-table"
    className={cn("w-full border-collapse text-[13px] tabular-nums", className)}
    {...props}
  />
);

export const TableHeader = ({ className, ...props }: React.ComponentProps<"thead">) => (
  <thead className={cn("sticky top-0 z-10 bg-sub", className)} {...props} />
);

export const TableBody = (props: React.ComponentProps<"tbody">) => {
  const ref = useFlip<HTMLTableSectionElement>();
  return <tbody ref={ref} {...props} />;
};

export const TableRow = ({ className, ...props }: React.ComponentProps<"tr">) => (
  <tr
    className={cn(
      "border-b border-hairline transition-colors duration-150 hover:bg-sub",
      className,
    )}
    {...props}
  />
);

export const TableHead = ({ className, ...props }: React.ComponentProps<"th">) => (
  <th
    className={cn("h-[30px] px-3 text-left text-[11.5px] font-normal text-quiet", className)}
    {...props}
  />
);

export const TableCell = ({ className, ...props }: React.ComponentProps<"td">) => (
  <td className={cn("h-[38px] truncate px-3 align-middle", className)} {...props} />
);
