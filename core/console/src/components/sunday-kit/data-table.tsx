import {
  type ColumnDef,
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import ArrowUpIcon from "@/assets/arrow-up.svg?react";

import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/sunday-kit/table-components";
import {Pagination} from "./pagination";
import {useMemo} from "react";
import {cn} from "@/utils/cn";

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
}

export function DataTable<TData, TValue>({columns, data}: DataTableProps<TData, TValue>) {
  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    enableSortingRemoval: true,
    initialState: {
      pagination: {
        pageIndex: 0,
        pageSize: 5,
      },
    },
  });

  const pagination = table.getState().pagination;

  const paginationProps = useMemo(() => {
    return {
      pageIndex: pagination.pageIndex,
      pageSize: pagination.pageSize,
      total: table.getRowCount(),
      canPreviousPage: table.getCanPreviousPage(),
      canNextPage: table.getCanNextPage(),
      pageCount: table.getPageCount(),
      gotoPage: table.setPageIndex,
      nextPage: table.nextPage,
      previousPage: table.previousPage,
    };
  }, [pagination, table]);

  return (
    <div className="overflow-hidden rounded-sm bg-transparent ring-neutral-300">
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id} className="hover:bg-transparent">
              {headerGroup.headers.map((header) => {
                const isSorted = header.column.getIsSorted();
                return (
                  <TableHead key={header.id} onClick={header.column.getToggleSortingHandler()}>
                    {header.isPlaceholder ? null : (
                      <button
                        type="button"
                        data-testid="column-header-button"
                        onClick={header.column.getToggleSortingHandler()}
                        className="hover:bg-neutral-200/60 px-1 rounded-sm py-0.5 flex items-center gap-1 group"
                      >
                        {flexRender(header.column.columnDef.header, header.getContext())}
                        <ArrowUpIcon
                          data-testid="column-header-arrow"
                          className={cn(
                            "w-3 h-3 shrink-0 transition-all duration-200",
                            isSorted === false && " text-neutral-50 group-hover:text-neutral-300",
                            isSorted !== false && "text-amber-600",
                            isSorted === "asc" && "rotate-180"
                          )}
                        />
                      </button>
                    )}
                  </TableHead>
                );
              })}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows?.length ? (
            table.getRowModel().rows.map((row) => (
              <TableRow key={row.id} data-state={row.getIsSelected() && "selected"}>
                {row.getVisibleCells().map((cell) => (
                  <TableCell key={cell.id}>
                    {flexRender(cell.column.columnDef.cell, cell.getContext())}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell colSpan={columns.length} className="h-24 text-center">
                No results.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
        <TableFooter>
          <TableRow className="hover:bg-transparent">
            <TableCell colSpan={columns.length}>
              <Pagination {...paginationProps} />
            </TableCell>
          </TableRow>
        </TableFooter>
      </Table>
    </div>
  );
}
