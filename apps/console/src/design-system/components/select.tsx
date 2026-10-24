import { Select as SelectPrimitive } from "@base-ui/react/select";
import { CheckIcon } from "lucide-react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

/** Trigger look shared by every select-like control (popover pickers included). */
export const selectTriggerClass = `${controlBase} flex h-[30px] min-w-[120px] items-center justify-between gap-2 rounded-md border border-input-border bg-gradient-to-b from-(--control-from) to-(--control-to) px-2.5 text-[13px] font-medium text-ink shadow-[inset_0_1px_0_rgba(255,255,255,0.95),0_1px_2px_rgba(24,24,27,0.06)] dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.05),0_1px_2px_rgba(0,0,0,0.5)]`;

/** The small chevron at the end of a select trigger. */
export const SelectChevron = () => (
  <span
    aria-hidden
    className="block size-1.5 -translate-y-px rotate-45 border-b-[1.5px] border-r-[1.5px] border-faint"
  />
);

type SelectOption = { value: string; label: string };

export type SelectProps = {
  options: readonly SelectOption[];
  value: string;
  onValueChange: (value: string) => void;
  /** Visible prefix inside the trigger, for example "Status". */
  label?: string;
  "aria-label"?: string;
  className?: string;
  disabled?: boolean;
  /** Shown in the trigger while no option is chosen. */
  placeholder?: string;
};

/** Single-choice select over a typed option list. */
export const Select = ({
  options,
  value,
  onValueChange,
  label,
  className,
  disabled,
  placeholder,
  "aria-label": ariaLabel,
}: SelectProps) => {
  const selected = options.find((option) => option.value === value);

  return (
    <SelectPrimitive.Root
      value={value}
      disabled={disabled}
      onValueChange={(next) => {
        if (next !== null) onValueChange(String(next));
      }}
    >
      <SelectPrimitive.Trigger
        aria-label={ariaLabel ?? label}
        data-slot="ds-select"
        className={cn(selectTriggerClass, className)}
      >
        <span className="min-w-0 flex-1 truncate text-left">
          {label ? `${label}: ` : ""}
          {selected ? (
            selected.label
          ) : (
            <span className="font-normal text-faint">{placeholder ?? value}</span>
          )}
        </span>
        <SelectPrimitive.Icon className="shrink-0">
          <span
            aria-hidden
            className="block size-1.5 -translate-y-px rotate-45 border-b-[1.5px] border-r-[1.5px] border-faint"
          />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Positioner alignItemWithTrigger={false} sideOffset={6} className="z-50">
          <SelectPrimitive.Popup className="max-h-[var(--available-height)] min-w-[var(--anchor-width)] max-w-[min(32rem,var(--available-width))] overflow-y-auto rounded-[10px] border border-hairline bg-card p-1 text-[13px] shadow-overlay outline-none">
            <SelectPrimitive.List>
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={option.value}
                  className="relative flex cursor-default select-none items-center rounded-md py-1.5 pl-2 pr-8 outline-none data-[highlighted]:bg-sub"
                >
                  <SelectPrimitive.ItemText className="truncate">
                    {option.label}
                  </SelectPrimitive.ItemText>
                  <SelectPrimitive.ItemIndicator className="absolute right-2">
                    <CheckIcon className="size-4" />
                  </SelectPrimitive.ItemIndicator>
                </SelectPrimitive.Item>
              ))}
            </SelectPrimitive.List>
          </SelectPrimitive.Popup>
        </SelectPrimitive.Positioner>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
};
