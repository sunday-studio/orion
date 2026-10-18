import { X } from "lucide-react";
import { useState } from "react";
import { cn } from "@/utils/cn";

/** A field holding removable chips; Enter or comma adds the typed value. */
export const ChipInput = ({
  chips,
  fixed = [],
  placeholder = "Add field",
  suggestions = [],
  label,
  onChange,
}: {
  chips: string[];
  /** Chips that are shown but cannot be removed. */
  fixed?: string[];
  placeholder?: string;
  suggestions?: string[];
  label: string;
  onChange: (chips: string[]) => void;
}) => {
  const [draft, setDraft] = useState("");
  const listId = `${label.replace(/\W+/g, "-")}-suggestions`;
  const add = () => {
    const value = draft.trim().replace(/,$/, "");
    if (value && !chips.includes(value) && !fixed.includes(value)) onChange([...chips, value]);
    setDraft("");
  };
  return (
    <div className="flex min-h-[36px] flex-wrap items-center gap-1.5 rounded-lg border border-input-border bg-card px-2 py-1.5 focus-within:ring-2 focus-within:ring-focus">
      {fixed.map((chip) => (
        <span
          key={chip}
          className="inline-flex h-6 items-center rounded-md bg-sub px-2 text-[12.5px]"
        >
          {chip}
        </span>
      ))}
      {chips.map((chip) => (
        <span
          key={chip}
          className="inline-flex h-6 items-center gap-1 rounded-md bg-sub pl-2 pr-1 text-[12.5px]"
        >
          {chip}
          <button
            type="button"
            aria-label={`Remove ${chip}`}
            className="flex size-4 items-center justify-center rounded text-faint hover:text-ink"
            onClick={() => onChange(chips.filter((item) => item !== chip))}
          >
            <X className="size-3" />
          </button>
        </span>
      ))}
      <input
        aria-label={label}
        list={listId}
        className={cn(
          "min-w-[110px] flex-1 bg-transparent text-[13px] outline-none placeholder:text-faint",
        )}
        placeholder={placeholder}
        value={draft}
        onChange={(event) => {
          if (event.target.value.endsWith(",")) {
            setDraft(event.target.value);
            window.setTimeout(add, 0);
          } else setDraft(event.target.value);
        }}
        onBlur={add}
        onKeyDown={(event) => {
          if (event.key === "Enter") {
            event.preventDefault();
            add();
          }
          if (event.key === "Backspace" && draft === "" && chips.length > 0)
            onChange(chips.slice(0, -1));
        }}
      />
      <datalist id={listId}>
        {suggestions
          .filter((item) => !chips.includes(item))
          .map((item) => (
            <option key={item} value={item} />
          ))}
      </datalist>
    </div>
  );
};
