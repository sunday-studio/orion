import * as React from "react";

import {cn} from "@/utils/cn";

function Table({className, ...props}: React.ComponentProps<"table">) {
  return (
    <div className="relative w-full overflow-x-auto">
      <table
        data-slot="table"
        data-testid="data-table"
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  );
}

function TableHeader({className, ...props}: React.ComponentProps<"thead">) {
  return (
    <thead
      data-testid="data-table-header"
      className={cn("[&_tr]:border-b", className)}
      {...props}
    />
  );
}

function TableBody({className, ...props}: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-testid="data-table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
}

function TableFooter({className, ...props}: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-testid="data-table-footer"
      className={cn("border-t border-neutral-200  mt-4 [&>tr]:last:border-b-0", className)}
      {...props}
    />
  );
}

function TableRow({className, ...props}: React.ComponentProps<"tr">) {
  return (
    <tr
      className={cn(
        "border-b border-neutral-200 font-normal transition-colors hover:bg-neutral-100 data-[state=selected]:bg-neutral-100",
        className
      )}
      data-testid="data-table-row"
      {...props}
    />
  );
}

function TableHead({className, ...props}: React.ComponentProps<"th">) {
  return (
    <th
      data-testid="data-table-head"
      className={cn(
        " h-10 pr-2 text-left align-middle whitespace-nowrap font-medium text-neutral-900 [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  );
}

function TableCell({className, ...props}: React.ComponentProps<"td">) {
  return (
    <td
      data-testid="data-table-cell"
      className={cn(
        "p-2 align-middle whitespace-nowrap text-neutral-700 [&:has([role=checkbox])]:pr-0 [&>[role=checkbox]]:translate-y-[2px]",
        className
      )}
      {...props}
    />
  );
}

export {Table, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell};
