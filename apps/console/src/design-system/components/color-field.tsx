import { CheckIcon } from "lucide-react";
import { Input } from "@/design-system/components/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/design-system/components/popover";
import { cn } from "@/utils/cn";

const PRESETS = [
  "#2a9f69",
  "#0d9488",
  "#2563eb",
  "#6366f1",
  "#9333ea",
  "#db2777",
  "#e5484d",
  "#f59e0b",
  "#18181b",
  "#71717a",
];

const HEX = /^#[0-9a-f]{6}$/i;

/** Hex colour field with a swatch that opens presets and the system picker. */
export const ColorField = ({
  value,
  onChange,
  placeholder = "#2a9f69",
  "aria-label": ariaLabel = "Colour",
}: {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  "aria-label"?: string;
}) => {
  const valid = HEX.test(value);
  return (
    <div className="flex items-center gap-2">
      <Popover>
        <PopoverTrigger
          aria-label={`${ariaLabel} picker`}
          className="size-[30px] shrink-0 rounded-md border border-input-border shadow-[inset_0_0_0_2px_var(--color-card)] outline-none focus-visible:ring-2 focus-visible:ring-focus"
          style={{ background: valid ? value : undefined }}
        />
        <PopoverContent align="start" className="w-[232px] p-3">
          <div className="grid grid-cols-5 gap-2">
            {PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                aria-label={preset}
                className="flex size-8 items-center justify-center rounded-md border border-black/10 text-white outline-none transition-transform hover:scale-105 focus-visible:ring-2 focus-visible:ring-focus"
                style={{ background: preset }}
                onClick={() => onChange(preset)}
              >
                {value.toLowerCase() === preset && <CheckIcon className="size-4" />}
              </button>
            ))}
          </div>
          <label className="mt-3 flex items-center justify-between gap-3 text-[12.5px] text-quiet">
            Custom
            <input
              type="color"
              aria-label="Pick a custom colour"
              className="h-8 w-20 cursor-pointer rounded-md border border-input-border bg-card p-0.5"
              value={valid ? value : "#2a9f69"}
              onChange={(event) => onChange(event.target.value)}
            />
          </label>
        </PopoverContent>
      </Popover>
      <Input
        aria-label={ariaLabel}
        aria-invalid={value !== "" && !valid}
        className={cn("font-mono")}
        placeholder={placeholder}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
};
