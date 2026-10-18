import { Button } from "@/design-system/components/button";
import { Select } from "@/design-system/components/select";
import { cn } from "@/utils/cn";

export type ListPaginationProps = {
  /** Total number of rows matching the query (from Core's `count` or `pagination.total_items`). */
  count: number;
  limit: number;
  offset: number;
  onOffsetChange: (offset: number) => void;
  /** When set, a "Rows per page" select is shown. */
  onLimitChange?: (limit: number) => void;
  pageSizes?: readonly number[];
  className?: string;
};

const DEFAULT_SIZES = [10, 20, 25, 50, 100] as const;

export const ListPagination = ({
  count,
  limit,
  offset,
  onOffsetChange,
  onLimitChange,
  pageSizes = DEFAULT_SIZES,
  className,
}: ListPaginationProps) => {
  const pageStart = count === 0 ? 0 : offset + 1;
  const pageEnd = Math.min(offset + limit, count);
  const canGoBack = offset > 0;
  const canGoForward = offset + limit < count;
  const sizes = [...new Set([...pageSizes, limit])].sort((a, b) => a - b);

  if (count === 0) return null;

  return (
    <div
      className={cn(
        "flex flex-wrap items-center justify-between gap-3 border-t border-line-soft px-3.5 py-2 text-[12.5px] text-quiet",
        className,
      )}
    >
      <span className="tabular-nums">
        Showing {pageStart} to {pageEnd} of {count}
      </span>
      <div className="flex items-center gap-1.5">
        {onLimitChange && (
          <>
            <Select
              aria-label="Rows per page"
              className="h-6 min-w-20 rounded-md px-2 text-xs"
              options={sizes.map((size) => ({ value: String(size), label: `${size} rows` }))}
              value={String(limit)}
              onValueChange={(value) => onLimitChange(Number(value))}
            />
          </>
        )}
        <div className="flex items-center gap-1.5">
          <Button
            variant="secondary"
            size="sm"
            disabled={!canGoBack}
            onClick={() => onOffsetChange(Math.max(0, offset - limit))}
          >
            Previous
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={!canGoForward}
            onClick={() => onOffsetChange(offset + limit)}
          >
            Next
          </Button>
        </div>
      </div>
    </div>
  );
};
