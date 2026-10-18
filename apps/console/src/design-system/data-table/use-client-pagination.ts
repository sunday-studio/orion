import { useState } from "react";
import type { ListPaginationProps } from "@/design-system/components/list-pagination";

/** Client-side paging for tables built from a full in-memory list (grid tables). */
export const useClientPagination = <T>(items: readonly T[], defaultPageSize = 20) => {
  const [state, setState] = useState({ pageIndex: 0, pageSize: defaultPageSize });
  const lastPage = Math.max(Math.ceil(items.length / state.pageSize) - 1, 0);
  const pageIndex = Math.min(state.pageIndex, lastPage);
  const start = pageIndex * state.pageSize;
  const pagination: ListPaginationProps = {
    count: items.length,
    limit: state.pageSize,
    offset: start,
    onOffsetChange: (offset) =>
      setState((current) => ({ ...current, pageIndex: Math.floor(offset / current.pageSize) })),
    onLimitChange: (pageSize) => setState({ pageIndex: 0, pageSize }),
  };
  return { pageItems: items.slice(start, start + state.pageSize), pagination };
};
