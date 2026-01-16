import {cn} from "@/utils/cn";

interface PaginationProps {
  pageIndex: number;
  pageSize: number;
  total: number;
  canPreviousPage: boolean;
  canNextPage: boolean;
  pageCount: number;
  gotoPage: (page: number) => void;
  nextPage: () => void;
  previousPage: () => void;
}

const Button = ({
  children,
  isDisabled,
  onClick,
  isSelected = false,
  testId,
}: {
  children: React.ReactNode;
  isDisabled?: boolean;
  onClick: () => void;
  isSelected?: boolean;
  testId?: string;
}) => {
  return (
    <button
      type="button"
      disabled={isDisabled}
      onClick={onClick}
      data-testid={testId}
      className={cn(
        `px-2 py-1 min-w-7  inset-ring-1 inset-ring-neutral-200
         rounded-md text-sm text-neutral-600 hover:text-neutral-700 
       hover:bg-neutral-100 transition
      `,
        {
          "opacity-50 cursor-not-allowed": isDisabled && !isSelected,
          "opcaity-1 bg-neutral-900 text-neutral-200 inset-ring-neutral-700 inset-ring-2 hover:bg-neutral-900 hover:text-neutral-200":
            isSelected,
        }
      )}
    >
      {children}
    </button>
  );
};

export const Pagination = ({
  pageIndex,
  pageSize,
  total,
  canPreviousPage,
  canNextPage,
  gotoPage,
  pageCount,
  nextPage,
  previousPage,
}: PaginationProps) => {
  return (
    <div className="flex justify-between items-center">
      <p data-testid="pagination-info" className="text-sm text-neutral-500">{`${
        pageIndex * pageSize + 1
      }-${Math.min((pageIndex + 1) * pageSize, total)} of ${total}`}</p>

      <div className="flex items-center gap-2 shrink-0">
        <Button isDisabled={!canPreviousPage} onClick={previousPage} testId="previous-button">
          Previous
        </Button>
        <div className="flex items-center gap-1">
          {Array.from({length: pageCount}).map((_, index) => (
            <Button
              key={`page-button-${index + 1}`}
              isDisabled={index === pageIndex}
              onClick={() => gotoPage(index)}
              isSelected={index === pageIndex}
              testId={`page-button-${index + 1}`}
            >
              {index + 1}
            </Button>
          ))}
        </div>
        <Button isDisabled={!canNextPage} onClick={nextPage} testId="next-button">
          Next
        </Button>
      </div>
    </div>
  );
};
