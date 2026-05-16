import {
  type ColumnDef,
  type Row,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from "@tanstack/react-table";
import type * as React from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  type TableVariant,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

type DataTableProps<TData> = {
  columns: ColumnDef<TData>[];
  data: TData[];
  emptyMessage?: string;
  getRowId?: (row: TData, index: number, parent?: Row<TData>) => string;
  isLoading?: boolean;
  loadingMessage?: string;
  onRowClick?: (row: TData) => void;
  rowClassName?: string | ((row: Row<TData>) => string | undefined);
  tableClassName?: string;
  variant?: TableVariant;
  bodyProps?: React.ComponentProps<typeof TableBody>;
  cellProps?: React.ComponentProps<typeof TableCell>;
  headProps?: React.ComponentProps<typeof TableHead>;
  headerProps?: React.ComponentProps<typeof TableHeader>;
  headerRowProps?: React.ComponentProps<typeof TableRow>;
  rowProps?: React.ComponentProps<typeof TableRow>;
  tableProps?: React.ComponentProps<typeof Table>;
};

export function DataTable<TData>({
  bodyProps,
  cellProps,
  columns,
  data,
  emptyMessage = "No rows found.",
  getRowId,
  headProps,
  headerProps,
  headerRowProps,
  isLoading = false,
  loadingMessage = "Loading...",
  onRowClick,
  rowClassName,
  rowProps,
  tableClassName,
  tableProps,
  variant = "default",
}: DataTableProps<TData>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId,
  });

  if (isLoading) {
    return <div className="py-3 text-sm text-neutral-600">{loadingMessage}</div>;
  }

  if (data.length === 0) {
    return <div className="py-3 text-sm text-neutral-600">{emptyMessage}</div>;
  }

  return (
    <Table
      {...tableProps}
      variant={tableProps?.variant ?? variant}
      className={cn(tableClassName, tableProps?.className)}
    >
      <TableHeader {...headerProps} variant={headerProps?.variant ?? variant}>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow
            {...headerRowProps}
            key={headerGroup.id}
            variant={headerRowProps?.variant ?? variant}
            className={headerRowProps?.className}
          >
            {headerGroup.headers.map((header) => (
              <TableHead
                {...headProps}
                key={header.id}
                variant={headProps?.variant ?? variant}
                className={headProps?.className}
              >
                {header.isPlaceholder
                  ? null
                  : flexRender(header.column.columnDef.header, header.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody {...bodyProps} variant={bodyProps?.variant ?? variant}>
        {table.getRowModel().rows.map((row) => (
          <TableRow
            {...rowProps}
            key={row.id}
            variant={rowProps?.variant ?? variant}
            className={cn(
              onRowClick && "cursor-pointer",
              typeof rowClassName === "function" ? rowClassName(row) : rowClassName,
              rowProps?.className,
            )}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell
                {...cellProps}
                key={cell.id}
                variant={cellProps?.variant ?? variant}
                className={cellProps?.className}
              >
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
