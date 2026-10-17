export type DataTableFilterOption = { value: string; label: string };

type DataTableFilterBase = {
  /** URL query key and filter value key. */
  key: string;
  placeholder?: string;
};

export type DataTableSearchField = DataTableFilterBase & { type: "search" };
export type DataTableInputField = DataTableFilterBase & { type: "input" };
export type DataTableSelectField = DataTableFilterBase & {
  type: "select";
  /** Prefix shown in the trigger, for example "Source". */
  label: string;
  options: readonly DataTableFilterOption[];
  /** Value that means "no filter". Defaults to "all". */
  allValue?: string;
};

export type DataTableFilterField =
  | DataTableSearchField
  | DataTableInputField
  | DataTableSelectField;

export type DataTableFilterValues = Record<string, string>;

export const filterEmptyValue = (field: DataTableFilterField) =>
  field.type === "select" ? (field.allValue ?? "all") : "";

export const isFilterActive = (field: DataTableFilterField, value: string | undefined) => {
  const current = (value ?? filterEmptyValue(field)).trim();
  return current !== filterEmptyValue(field);
};

/** Returns the API param value for a filter, or undefined when the filter is inactive. */
export const filterParam = (field: DataTableFilterField, value: string | undefined) =>
  isFilterActive(field, value) ? (value ?? "").trim() : undefined;
