import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import {
  type DataTableFilterField,
  type DataTableFilterValues,
  filterEmptyValue,
  isFilterActive,
} from "@/components/shared/data-table-schema";

type DataTableToolbarProps = {
  fields: readonly DataTableFilterField[];
  values: DataTableFilterValues;
  onChange: (key: string, value: string) => void;
  onClear: () => void;
};

export const DataTableToolbar = ({ fields, values, onChange, onClear }: DataTableToolbarProps) => {
  const hasFilters = fields.some((field) => isFilterActive(field, values[field.key]));

  return (
    <div className="flex flex-wrap items-center gap-2">
      {fields.map((field) => {
        const value = values[field.key] ?? filterEmptyValue(field);

        if (field.type === "select") {
          const selected = field.options.find((option) => option.value === value);
          return (
            <Select
              key={field.key}
              value={value}
              onValueChange={(next) => onChange(field.key, next)}
            >
              <SelectTrigger className="min-w-44 text-xs">
                <span data-slot="select-value">
                  {field.label}: {selected?.label ?? value}
                </span>
              </SelectTrigger>
              <SelectContent>
                {field.options.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          );
        }

        if (field.type === "search") {
          return (
            <div key={field.key} className="relative w-full max-w-sm">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-neutral-400" />
              <Input
                value={value}
                onChange={(event) => onChange(field.key, event.target.value)}
                placeholder={field.placeholder}
                className="pl-9"
              />
            </div>
          );
        }

        return (
          <Input
            key={field.key}
            value={value}
            onChange={(event) => onChange(field.key, event.target.value)}
            placeholder={field.placeholder}
            className="w-full max-w-48"
          />
        );
      })}
      {hasFilters && (
        <Button variant="ghost" size="sm" onClick={onClear}>
          Clear
        </Button>
      )}
    </div>
  );
};
