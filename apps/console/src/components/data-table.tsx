import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type Row,
} from "@tanstack/react-table";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
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
};

export function DataTable<TData>({
  columns,
  data,
  emptyMessage = "No rows found.",
  getRowId,
  isLoading = false,
  loadingMessage = "Loading...",
  onRowClick,
  rowClassName,
  tableClassName,
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
    <Table className={tableClassName}>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header) => (
              <TableHead key={header.id}>
                {header.isPlaceholder
                  ? null
                  : flexRender(header.column.columnDef.header, header.getContext())}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow
            key={row.id}
            className={cn(
              onRowClick && "cursor-pointer",
              typeof rowClassName === "function" ? rowClassName(row) : rowClassName,
            )}
            onClick={onRowClick ? () => onRowClick(row.original) : undefined}
          >
            {row.getVisibleCells().map((cell) => (
              <TableCell key={cell.id}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
