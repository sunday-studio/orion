import { EmptyState } from "@/design-system";
import {
  type ColumnDef,
  type Row,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import * as React from "react";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  type TableVariant,
} from "@/components/ui/table";
import { cn } from "@/utils/cn";

type DataTableProps<TData> = {
  columns: ColumnDef<TData>[];
  data: TData[];
  emptyMessage?: string;
  getRowId?: (row: TData, index: number, parent?: Row<TData>) => string;
  isLoading?: boolean;
  loadingMessage?: string;
  onRowClick?: (row: TData) => void;
  /** URL-friendly sort, "columnId.asc" or "columnId.desc". Sorts the visible page client-side. */
  sort?: string;
  onSortChange?: (sort: string) => void;
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

const parseSort = (sort: string): SortingState => {
  const [id, direction] = sort.split(".");
  return id ? [{ id, desc: direction === "desc" }] : [];
};

const serializeSort = (sorting: SortingState) =>
  sorting[0] ? `${sorting[0].id}.${sorting[0].desc ? "desc" : "asc"}` : "";

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
  onSortChange,
  sort = "",
  tableClassName,
  tableProps,
  variant = "default",
}: DataTableProps<TData>) {
  const sorting = React.useMemo(() => parseSort(sort), [sort]);
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId,
    state: { sorting },
    onSortingChange: (updater) => {
      if (!onSortChange) return;
      const next = typeof updater === "function" ? updater(sorting) : updater;
      onSortChange(serializeSort(next));
    },
    enableSorting: Boolean(onSortChange),
  });

  if (isLoading) {
    return <div className="py-3 text-sm text-neutral-600">{loadingMessage}</div>;
  }

  if (data.length === 0) {
    return <EmptyState title={emptyMessage} />;
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
                {header.isPlaceholder ? null : header.column.getCanSort() ? (
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 hover:text-neutral-900"
                    onClick={header.column.getToggleSortingHandler()}
                  >
                    {flexRender(header.column.columnDef.header, header.getContext())}
                    <span aria-hidden className="text-neutral-400">
                      {{ asc: "↑", desc: "↓" }[header.column.getIsSorted() as string] ?? ""}
                    </span>
                  </button>
                ) : (
                  flexRender(header.column.columnDef.header, header.getContext())
                )}
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
