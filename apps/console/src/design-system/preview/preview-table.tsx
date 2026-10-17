import type { ColumnDef } from "@tanstack/react-table";
import { useState } from "react";
import {
  DataTable,
  type DataTableFilterField,
  FilterBar,
  StatusBadge,
  filterParam,
  useDataTableUrlState,
} from "@/design-system";
import { Section } from "@/design-system/preview/preview-controls";

type Row = { id: string; name: string; status: string; region: string };

const rows: Row[] = [
  { id: "1", name: "api-https", status: "up", region: "eu" },
  { id: "2", name: "db-primary", status: "down", region: "us" },
  { id: "3", name: "edge-cache", status: "degraded", region: "eu" },
];

const columns: ColumnDef<Row>[] = [
  { accessorKey: "name", header: "Monitor" },
  {
    accessorKey: "status",
    header: "Status",
    cell: ({ row }) => <StatusBadge status={row.original.status} />,
  },
  { accessorKey: "region", header: "Region" },
];

const fields: readonly DataTableFilterField[] = [
  { type: "search", key: "q", placeholder: "Search monitors" },
  {
    type: "select",
    key: "status",
    label: "Status",
    options: [
      { value: "all", label: "All statuses" },
      { value: "up", label: "Up" },
      { value: "down", label: "Down" },
      { value: "degraded", label: "Degraded" },
    ],
  },
];

export const TablePreview = () => {
  const table = useDataTableUrlState({ fields, limit: 10, prefix: "ds" });
  const [clicked, setClicked] = useState("none");
  const query = (filterParam(fields[0], table.values.q) ?? "").toLowerCase();
  const status = filterParam(fields[1], table.values.status);
  const visible = rows.filter(
    (row) => row.name.includes(query) && (!status || row.status === status),
  );

  return (
    <Section title="Data table">
      <FilterBar
        fields={fields}
        values={table.values}
        onChange={table.setFilter}
        onClear={table.clearFilters}
      />
      <DataTable
        columns={columns}
        data={visible}
        getRowId={(row) => row.id}
        sort={table.sort}
        onSortChange={table.setSort}
        onRowClick={(row) => setClicked(row.name)}
        onResetFilters={table.hasFilters ? table.clearFilters : undefined}
        emptyMessage="No monitors match the current filters."
      />
      <p className="text-xs text-quiet">Last row opened: {clicked}</p>
      <DataTable columns={columns} data={[]} isLoading />
    </Section>
  );
};
