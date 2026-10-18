import type * as React from "react";
import { cn } from "@/utils/cn";

/** Unstyled-by-default table primitives with design-language rows (38px, sticky 30px header). */
export const Table = ({
  className,
  dividers = false,
  ...props
}: React.ComponentProps<"table"> & { dividers?: boolean }) => (
  <table
    data-slot="ds-table"
    className={cn(
      "w-full border-collapse text-[13px] tabular-nums",
      dividers &&
        "[&_td:not(:last-child)]:border-r [&_th:not(:last-child)]:border-r [&_td]:border-line-soft [&_th]:border-line-soft",
      className,
    )}
    {...props}
  />
);

export const TableHeader = ({ className, ...props }: React.ComponentProps<"thead">) => (
  <thead className={cn("sticky top-0 z-10 bg-sub", className)} {...props} />
);

export const TableBody = (props: React.ComponentProps<"tbody">) => <tbody {...props} />;

export const TableRow = ({ className, ...props }: React.ComponentProps<"tr">) => (
  <tr className={cn("border-b border-hairline hover:bg-sub", className)} {...props} />
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
