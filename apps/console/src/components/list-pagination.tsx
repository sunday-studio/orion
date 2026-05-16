import { Button } from "@/components/ui/button";

type ListPaginationProps = {
  count: number;
  limit: number;
  offset: number;
  onOffsetChange: (offset: number) => void;
};

export const ListPagination = ({ count, limit, offset, onOffsetChange }: ListPaginationProps) => {
  const pageStart = count === 0 ? 0 : offset + 1;
  const pageEnd = Math.min(offset + limit, count);
  const canGoBack = offset > 0;
  const canGoForward = offset + limit < count;

  if (count <= limit) return null;

  return (
    <div className="flex items-center justify-between gap-3 py-3 text-sm px-3">
      <span className="text-neutral-600 tabular-nums">
        {pageStart} - {pageEnd} of {count}
      </span>
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
  );
};
