import { Select as SelectPrimitive } from "@base-ui/react/select";
import { CheckIcon, ChevronDownIcon } from "lucide-react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export type SelectOption = { value: string; label: string };

export type SelectProps = {
  options: readonly SelectOption[];
  value: string;
  onValueChange: (value: string) => void;
  /** Visible prefix inside the trigger, for example "Status". */
  label?: string;
  "aria-label"?: string;
  className?: string;
  disabled?: boolean;
};

/** Single-choice select over a typed option list. */
export const Select = ({
  options,
  value,
  onValueChange,
  label,
  className,
  disabled,
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
        className={cn(
          controlBase,
          "flex h-9 min-w-40 items-center justify-between gap-2 rounded-lg border border-input-border bg-card px-3 text-[13px] text-ink",
          className,
        )}
      >
        <span className="truncate">
          {label ? `${label}: ` : ""}
          {selected?.label ?? value}
        </span>
        <SelectPrimitive.Icon>
          <ChevronDownIcon className="size-4 text-quiet" />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Positioner alignItemWithTrigger={false} sideOffset={6} className="z-50">
          <SelectPrimitive.Popup className="max-h-[var(--available-height)] min-w-[var(--anchor-width)] overflow-y-auto rounded-[10px] border border-hairline bg-card p-1 text-[13px] shadow-overlay outline-none">
            <SelectPrimitive.List>
              {options.map((option) => (
                <SelectPrimitive.Item
                  key={option.value}
                  value={option.value}
                  className="relative flex cursor-default select-none items-center rounded-md py-1.5 pl-2 pr-8 outline-none data-[highlighted]:bg-sub"
                >
                  <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
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
