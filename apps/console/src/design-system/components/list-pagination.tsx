import { Button } from "@/design-system/components/button";
import { Select } from "@/design-system/components/select";
import { cn } from "@/utils/cn";

type ListPaginationProps = {
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
        "sticky bottom-0 z-10 flex min-h-9 flex-wrap items-center justify-between gap-3 bg-page/95 px-3 text-sm backdrop-blur",
        className,
      )}
    >
      <span className="text-quiet tabular-nums">
        {pageStart} - {pageEnd} of {count}
      </span>
      <div className="flex items-center gap-3">
        {onLimitChange && (
          <div className="flex items-center gap-2 text-quiet">
            <span className="hidden sm:inline">Rows per page</span>
            <Select
              aria-label="Rows per page"
              className="w-[72px]"
              options={sizes.map((size) => ({ value: String(size), label: String(size) }))}
              value={String(limit)}
              onValueChange={(value) => onLimitChange(Number(value))}
            />
          </div>
        )}
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            disabled={!canGoBack}
            onClick={() => onOffsetChange(Math.max(0, offset - limit))}
          >
            Previous
          </Button>
          <Button
            variant="ghost"
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
