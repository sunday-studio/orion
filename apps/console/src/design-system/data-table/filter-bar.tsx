import { Search } from "lucide-react";
import { Button } from "@/design-system/components/button";
import { Input } from "@/design-system/components/input";
import { Select } from "@/design-system/components/select";
import {
  type DataTableFilterField,
  type DataTableFilterValues,
  filterEmptyValue,
  isFilterActive,
} from "@/design-system/data-table/schema";

export type FilterBarProps = {
  fields: readonly DataTableFilterField[];
  values: DataTableFilterValues;
  onChange: (key: string, value: string) => void;
  onClear: () => void;
};

/** Schema-driven filter row. Pair with useDataTableUrlState so filters live in the URL. */
export const FilterBar = ({ fields, values, onChange, onClear }: FilterBarProps) => {
  const hasFilters = fields.some((field) => isFilterActive(field, values[field.key]));

  return (
    <div role="search" className="flex flex-wrap items-center gap-2">
      {fields.map((field) => {
        const value = values[field.key] ?? filterEmptyValue(field);
        if (field.type === "select") {
          return (
            <Select
              key={field.key}
              label={field.label}
              options={field.options}
              value={value}
              onValueChange={(next) => onChange(field.key, next)}
            />
          );
        }
        const input = (
          <Input
            aria-label={field.placeholder ?? field.key}
            placeholder={field.placeholder}
            value={value}
            className={field.type === "search" ? "pl-9" : "w-48"}
            onChange={(event) => onChange(field.key, event.target.value)}
          />
        );
        return field.type === "search" ? (
          <div key={field.key} className="relative w-full max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint" />
            {input}
          </div>
        ) : (
          <div key={field.key}>{input}</div>
        );
      })}
      {hasFilters && (
        <Button size="sm" variant="ghost" onClick={onClear}>
          Clear
        </Button>
      )}
    </div>
  );
};
