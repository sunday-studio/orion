import { Combobox as ComboboxPrimitive } from "@base-ui/react/combobox";
import { CheckIcon } from "lucide-react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export type ComboboxOption = { value: string; label: string };

export type ComboboxProps = {
  options: readonly ComboboxOption[];
  value: string | null;
  onValueChange: (value: string | null) => void;
  placeholder?: string;
  emptyMessage?: string;
  "aria-label": string;
  className?: string;
};

/** Searchable single-choice list. Typing filters options; arrows and Enter select. */
export const Combobox = ({
  options,
  value,
  onValueChange,
  placeholder = "Search",
  emptyMessage = "No matches.",
  "aria-label": ariaLabel,
  className,
}: ComboboxProps) => {
  const selected = options.find((option) => option.value === value) ?? null;

  return (
    <ComboboxPrimitive.Root
      items={options}
      value={selected}
      onValueChange={(next) => onValueChange(next ? (next as ComboboxOption).value : null)}
    >
      <ComboboxPrimitive.Input
        aria-label={ariaLabel}
        placeholder={placeholder}
        className={cn(
          controlBase,
          "h-9 w-full min-w-0 rounded-lg border border-input-border bg-card px-3 text-[13px] text-ink placeholder:text-faint",
          className,
        )}
      />
      <ComboboxPrimitive.Portal>
        <ComboboxPrimitive.Positioner sideOffset={6} className="z-50">
          <ComboboxPrimitive.Popup className="max-h-72 min-w-[var(--anchor-width)] overflow-y-auto rounded-[10px] border border-hairline bg-card p-1 text-[13px] shadow-overlay outline-none">
            <ComboboxPrimitive.Empty className="px-2 py-1.5 text-quiet empty:hidden">
              {emptyMessage}
            </ComboboxPrimitive.Empty>
            <ComboboxPrimitive.List>
              {(item: ComboboxOption) => (
                <ComboboxPrimitive.Item
                  key={item.value}
                  value={item}
                  className="relative flex cursor-default select-none items-center rounded-md py-1.5 pl-2 pr-8 outline-none data-[highlighted]:bg-sub"
                >
                  {item.label}
                  <ComboboxPrimitive.ItemIndicator className="absolute right-2">
                    <CheckIcon className="size-4" />
                  </ComboboxPrimitive.ItemIndicator>
                </ComboboxPrimitive.Item>
              )}
            </ComboboxPrimitive.List>
          </ComboboxPrimitive.Popup>
        </ComboboxPrimitive.Positioner>
      </ComboboxPrimitive.Portal>
    </ComboboxPrimitive.Root>
  );
};
