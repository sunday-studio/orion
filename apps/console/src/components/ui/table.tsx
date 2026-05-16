import type * as React from "react";

import { cn } from "@/lib/utils";

export type TableVariant = "default" | "contained";

type TableElementProps<TElement extends keyof React.JSX.IntrinsicElements> =
  React.ComponentProps<TElement> & {
    variant?: TableVariant;
  };

const tableVariantClasses = {
  default: {
    container: "",
    table: "",
    header: "[&_tr]:border-b border-neutral-200/70",
    body: "[&>tr:has(+_tr:hover)]:border-emerald-100 [&>tr:hover]:border-emerald-100 [&>tr:hover]:bg-emerald-50 [&>tr:hover>td]:border-emerald-100",
    footer: "border-t bg-muted/50 font-medium [&>tr]:last:border-b-0 border-neutral-200",
    row: "border-b border-neutral-200/70 transition-colors group",
    head: "border-neutral-200/70 border-b border-l first:border-l-0 h-10 px-3 py-2 text-left align-middle whitespace-nowrap text-neutral-950 [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-[2px] font-medium",
    cell: "py-2 px-3 align-middle text-sm whitespace-nowrap [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-[2px] border-neutral-200/70 border-l first:border-l-0",
  },
  contained: {
    container: "rounded-t-lg bg-neutral-100/70",
    table: "bg-transparent",
    header: "[&_tr]:border-b border-neutral-300/60",
    body: "[&>tr:has(+_tr:hover)]:border-emerald-200 [&>tr:hover]:border-emerald-200 [&>tr:hover]:bg-emerald-50 [&>tr:hover>td]:border-emerald-200",
    footer: "border-t bg-neutral-100 font-medium [&>tr]:last:border-b-0 border-neutral-300/60",
    row: "border-b border-neutral-300/60 transition-colors group last:border-b-0",
    head: "border-neutral-300/60 border-b border-l first:border-l-0 h-10 px-3 py-2 text-left align-middle whitespace-nowrap text-neutral-950 [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-[2px] font-medium",
    cell: "py-2 px-3 align-middle text-sm whitespace-nowrap [&:has([role=checkbox])]:pr-0 *:[[role=checkbox]]:translate-y-[2px] border-neutral-300/60 border-l first:border-l-0",
  },
} satisfies Record<TableVariant, Record<string, string>>;

function Table({ className, variant = "default", ...props }: TableElementProps<"table">) {
  return (
    <div
      data-slot="table-container"
      className={cn("relative w-full overflow-x-auto", tableVariantClasses[variant].container)}
    >
      <table
        data-slot="table"
        className={cn(
          "w-full caption-bottom text-sm",
          tableVariantClasses[variant].table,
          className,
        )}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, variant = "default", ...props }: TableElementProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn(tableVariantClasses[variant].header, className)}
      {...props}
    />
  );
}

function TableBody({ className, variant = "default", ...props }: TableElementProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn(tableVariantClasses[variant].body, className)}
      {...props}
    />
  );
}

function TableFooter({ className, variant = "default", ...props }: TableElementProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(tableVariantClasses[variant].footer, className)}
      {...props}
    />
  );
}

function TableRow({ className, variant = "default", ...props }: TableElementProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(tableVariantClasses[variant].row, className)}
      {...props}
    />
  );
}

function TableHead({ className, variant = "default", ...props }: TableElementProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(tableVariantClasses[variant].head, className)}
      {...props}
    />
  );
}

function TableCell({ className, variant = "default", ...props }: TableElementProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(tableVariantClasses[variant].cell, className)}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export { Table, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell, TableCaption };
