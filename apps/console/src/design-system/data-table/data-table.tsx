import {
  type ColumnDef,
  type Row,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { useMemo } from "react";
import { Button } from "@/design-system/components/button";
import { EmptyState } from "@/design-system/components/empty-state";
import { Skeleton } from "@/design-system/components/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/design-system/data-table/table";
import { cn } from "@/utils/cn";

export type DataTableProps<TData> = {
  columns: ColumnDef<TData>[];
  data: TData[];
  getRowId?: (row: TData, index: number) => string;
  isLoading?: boolean;
  /** Shown when the query failed; ends in a Retry button (design rule 5). */
  error?: { message: string; onRetry: () => void };
  emptyMessage?: string;
  /** Called when filters are active so the empty state can offer a reset. */
  onResetFilters?: () => void;
  onRowClick?: (row: TData) => void;
  /** "columnId.asc" or "columnId.desc"; sorts the visible page client-side. */
  sort?: string;
  onSortChange?: (sort: string) => void;
  dense?: boolean;
};

const parseSort = (sort: string): SortingState => {
  const [id, direction] = sort.split(".");
  return id ? [{ id, desc: direction === "desc" }] : [];
};

const serializeSort = (sorting: SortingState) =>
  sorting[0] ? `${sorting[0].id}.${sorting[0].desc ? "desc" : "asc"}` : "";

const SKELETON_ROWS = 6;

/** Schema-driven table: columns are TanStack column defs. Loading, empty and error states built in. */
export function DataTable<TData>({
  columns,
  data,
  getRowId,
  isLoading = false,
  error,
  emptyMessage = "No rows found.",
  onResetFilters,
  onRowClick,
  sort = "",
  onSortChange,
  dense = false,
}: DataTableProps<TData>) {
  const sorting = useMemo(() => parseSort(sort), [sort]);
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getRowId,
    state: { sorting },
    enableSorting: Boolean(onSortChange),
    onSortingChange: (updater) => {
      if (!onSortChange) return;
      onSortChange(serializeSort(typeof updater === "function" ? updater(sorting) : updater));
    },
  });

  if (error) {
    return (
      <EmptyState
        tone="error"
        title={error.message}
        action={
          <Button size="sm" variant="secondary" onClick={error.onRetry}>
            Retry
          </Button>
        }
      />
    );
  }

  if (!isLoading && data.length === 0) {
    return (
      <EmptyState
        title={emptyMessage}
        action={
          onResetFilters ? (
            <Button size="sm" variant="secondary" onClick={onResetFilters}>
              Reset filters
            </Button>
          ) : undefined
        }
      />
    );
  }

  const cellHeight = dense ? "h-[30px]" : undefined;

  return (
    <Table aria-busy={isLoading}>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id} className="hover:bg-transparent">
            {group.headers.map((header) => {
              const sorted = header.column.getIsSorted();
              return (
                <TableHead
                  key={header.id}
                  aria-sort={
                    sorted === "asc" ? "ascending" : sorted === "desc" ? "descending" : undefined
                  }
                >
                  {header.isPlaceholder ? null : header.column.getCanSort() ? (
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 hover:text-ink"
                      onClick={header.column.getToggleSortingHandler()}
                    >
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      <span aria-hidden>
                        {sorted === "asc" ? "↑" : sorted === "desc" ? "↓" : ""}
                      </span>
                    </button>
                  ) : (
                    flexRender(header.column.columnDef.header, header.getContext())
                  )}
                </TableHead>
              );
            })}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {isLoading
          ? Array.from({ length: SKELETON_ROWS }, (_, index) => (
              <TableRow key={index} className="hover:bg-transparent">
                {columns.map((_column, columnIndex) => (
                  <TableCell key={columnIndex} className={cellHeight}>
                    <Skeleton className="h-3 w-3/4" />
                  </TableCell>
                ))}
              </TableRow>
            ))
          : table.getRowModel().rows.map((row: Row<TData>) => (
              <TableRow
                key={row.id}
                className={cn(onRowClick && "cursor-pointer")}
                tabIndex={onRowClick ? 0 : undefined}
                onClick={onRowClick ? () => onRowClick(row.original) : undefined}
                onKeyDown={
                  onRowClick
                    ? (event) => {
                        if (event.target === event.currentTarget && event.key === "Enter") {
                          onRowClick(row.original);
                        }
                      }
                    : undefined
                }
              >
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id} className={cellHeight}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))}
      </TableBody>
    </Table>
  );
}
