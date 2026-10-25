import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { useCallback, useMemo } from "react";
import { usePageSize } from "@/design-system/data-table/use-page-size";
import {
  type DataTableFilterField,
  type DataTableFilterValues,
  filterEmptyValue,
  isFilterActive,
} from "@/design-system/data-table/schema";

type UseDataTableUrlStateOptions = {
  fields: readonly DataTableFilterField[];
  limit: number;
  /** Prefix for URL keys so several tables can share one page, for example "service". */
  prefix?: string;
};

const prefixed = (prefix: string | undefined, key: string) =>
  prefix ? `${prefix}${key.charAt(0).toUpperCase()}${key.slice(1)}` : key;

/**
 * URL-backed (nuqs) filter, sort and page state for a schema-driven data table.
 * `fields` must be a stable, module-level constant.
 */
export function useDataTableUrlState({
  fields,
  limit: defaultLimit,
  prefix,
}: UseDataTableUrlStateOptions) {
  const pageKey = prefixed(prefix, "page");
  const { limit, setLimit } = usePageSize(defaultLimit, {
    key: prefixed(prefix, "pageSize"),
    pageKey,
  });
  const sortKey = prefixed(prefix, "sort");

  const parsers = useMemo(() => {
    const entries: Record<string, ReturnType<typeof parseAsString.withDefault>> = {};
    for (const field of fields) {
      entries[prefixed(prefix, field.key)] = parseAsString.withDefault(filterEmptyValue(field));
    }
    return { ...entries, [sortKey]: parseAsString.withDefault("") };
  }, [fields, prefix, sortKey]);
  const [state, setState] = useQueryStates({
    ...parsers,
    [pageKey]: parseAsInteger.withDefault(1),
  });

  const values: DataTableFilterValues = {};
  for (const field of fields) {
    values[field.key] = String(state[prefixed(prefix, field.key)] ?? filterEmptyValue(field));
  }

  const page = Math.max(Number(state[pageKey]) || 1, 1);
  const offset = (page - 1) * limit;
  const sort = String(state[sortKey] ?? "");
  const hasFilters = fields.some((field) => isFilterActive(field, values[field.key]));

  const setFilter = useCallback(
    (key: string, value: string) => {
      void setState({ [prefixed(prefix, key)]: value, [pageKey]: 1 });
    },
    [pageKey, prefix, setState],
  );

  const clearFilters = useCallback(() => {
    const reset: Record<string, string | number> = { [pageKey]: 1 };
    for (const field of fields) reset[prefixed(prefix, field.key)] = filterEmptyValue(field);
    void setState(reset);
  }, [fields, pageKey, prefix, setState]);

  const setOffset = useCallback(
    (nextOffset: number) => {
      void setState({ [pageKey]: Math.floor(nextOffset / limit) + 1 });
    },
    [limit, pageKey, setState],
  );

  const setSort = useCallback(
    (next: string) => {
      void setState({ [sortKey]: next, [pageKey]: 1 });
    },
    [pageKey, setState, sortKey],
  );

  return {
    values,
    page,
    offset,
    limit,
    setLimit,
    sort,
    hasFilters,
    setFilter,
    clearFilters,
    setOffset,
    setSort,
  };
}
