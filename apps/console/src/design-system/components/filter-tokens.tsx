import { Plus, X } from "lucide-react";
import type * as React from "react";
import { Popover, PopoverContent, PopoverTrigger } from "@/design-system/components/popover";
import { Button } from "@/design-system/components/button";
import { Checkbox } from "@/design-system/components/checkbox";
import { cn } from "@/utils/cn";

export type FilterOption = { value: string; label: string; group?: string };

/** One applied filter: "Status | is one of | Open, Acknowledged | x" (canvas `.ft2`). */
export const FilterToken = ({
  field,
  operator,
  value,
  onRemove,
  children,
}: {
  field: string;
  operator: string;
  value: string;
  onRemove?: () => void;
  /** Popover content that edits the token; the token body is its trigger. */
  children?: React.ReactNode;
}) => (
  <Popover>
    <div className="inline-flex h-7 items-stretch overflow-hidden rounded-[7px] border border-input-border bg-card text-[12.5px] shadow-[0_1px_1px_rgba(0,0,0,0.04)]">
      <PopoverTrigger
        aria-label={`Edit ${field} filter`}
        className="flex items-stretch outline-none focus-visible:ring-2 focus-visible:ring-focus"
      >
        <i className="flex items-center border-r border-hairline bg-sub px-[9px] not-italic text-mid">
          {field}
        </i>
        <i className="flex items-center border-r border-hairline px-[9px] not-italic text-faint">
          {operator}
        </i>
        <b className="flex items-center px-[9px] font-medium">{value}</b>
      </PopoverTrigger>
      {onRemove && (
        <button
          type="button"
          aria-label={`Remove ${field} filter`}
          onClick={onRemove}
          className="flex items-center border-l border-hairline px-[7px] text-faint outline-none hover:text-ink focus-visible:ring-2 focus-visible:ring-focus"
        >
          <X className="size-3" />
        </button>
      )}
    </div>
    {children}
  </Popover>
);

export const AddFilterButton = ({ children }: { children: React.ReactNode }) => (
  <Popover>
    <PopoverTrigger className="inline-flex h-7 items-center gap-[5px] rounded-[7px] border border-dashed border-input-border px-[9px] text-[12.5px] text-quiet outline-none hover:text-ink focus-visible:ring-2 focus-visible:ring-focus">
      <Plus className="size-3.5" />
      Filter
    </PopoverTrigger>
    {children}
  </Popover>
);

/** Row of tokens with "Clear all" on the right. */
export const FilterTokenRow = ({
  children,
  onClearAll,
  className,
}: {
  children: React.ReactNode;
  onClearAll?: () => void;
  className?: string;
}) => (
  <div
    role="search"
    aria-label="Filters"
    className={cn("flex flex-wrap items-center gap-2", className)}
  >
    {children}
    {onClearAll && (
      <button
        type="button"
        onClick={onClearAll}
        className="ml-auto text-[12.5px] text-mid hover:text-ink"
      >
        Clear all
      </button>
    )}
  </div>
);

/** Popover body for "is one of" filters: grouped checkboxes with a Done button. */
export const CheckFilterPanel = ({
  title,
  options,
  selected,
  onChange,
  onDone,
}: {
  title: string;
  options: readonly FilterOption[];
  selected: readonly string[];
  onChange: (next: string[]) => void;
  onDone: () => void;
}) => {
  const groups = Array.from(new Set(options.map((option) => option.group ?? "")));
  return (
    <PopoverContent className="w-[260px] p-0">
      <div className="flex items-center justify-between border-b border-hairline px-3 py-2 text-[12.5px] text-quiet">
        <span>← {title}</span>
        <Button size="sm" onClick={onDone}>
          Done
        </Button>
      </div>
      <div className="px-3 py-2.5">
        <div className="mb-2 flex h-8 items-center rounded-lg border border-input-border px-2.5 text-[12.5px]">
          is one of
        </div>
        {groups.map((group) => (
          <div key={group}>
            {group && (
              <div className="pb-1 pt-2 font-mono text-[10.5px] uppercase tracking-[0.08em] text-faint">
                {group}
              </div>
            )}
            {options
              .filter((option) => (option.group ?? "") === group)
              .map((option) => (
                <label key={option.value} className="flex items-center gap-2.5 py-1.5 text-[13px]">
                  <Checkbox
                    checked={selected.includes(option.value)}
                    onCheckedChange={(checked) =>
                      onChange(
                        checked
                          ? [...selected, option.value]
                          : selected.filter((value) => value !== option.value),
                      )
                    }
                  />
                  {option.label}
                </label>
              ))}
          </div>
        ))}
      </div>
    </PopoverContent>
  );
};

/** Floating action bar anchored to the bottom of the viewport (bulk actions). */
export const FloatingBar = ({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) => (
  <div
    role="toolbar"
    className={cn(
      "fixed bottom-12 left-1/2 z-40 flex -translate-x-1/2 items-center gap-2 rounded-xl border border-hairline bg-card px-3.5 py-2.5 text-[13px] shadow-overlay",
      className,
    )}
  >
    {children}
  </div>
);
