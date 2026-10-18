import { Search } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Input, Popover, PopoverContent, PopoverTrigger, Segmented, Select } from "@/design-system";
import { cn } from "@/utils/cn";
import { RANGES, rangeLabel } from "../logs-state";

const pill =
  "inline-flex h-9 items-center justify-between gap-2 rounded-lg border border-input-border bg-card px-3 text-[13px] hover:bg-sub";

export const SearchBox = ({
  value,
  placeholder,
  onChange,
}: {
  value: string;
  placeholder: string;
  onChange: (value: string) => void;
}) => {
  const ref = useRef<HTMLInputElement>(null);
  const [draft, setDraft] = useState(value);
  useEffect(() => setDraft(value), [value]);
  useEffect(() => {
    const timer = window.setTimeout(() => draft !== value && onChange(draft), 300);
    return () => window.clearTimeout(timer);
  }, [draft, value, onChange]);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement;
      if (event.key === "/" && !/INPUT|TEXTAREA|SELECT/.test(target.tagName)) {
        event.preventDefault();
        ref.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);
  return (
    <div className="relative min-w-0 flex-1">
      <Search
        aria-hidden
        className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint"
      />
      <Input
        ref={ref}
        aria-label="Search logs"
        placeholder={placeholder}
        className="h-9 pl-9 pr-8"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
      />
      <kbd className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-faint">
        /
      </kbd>
    </div>
  );
};

const toLocalInput = (iso: string) => (iso ? iso.slice(0, 16) : "");

export const RangePicker = ({
  range,
  from,
  to,
  live,
  onChange,
}: {
  range: string;
  from: string;
  to: string;
  live?: boolean;
  onChange: (patch: { range: string; from?: string; to?: string }) => void;
}) => {
  const [custom, setCustom] = useState({ from: toLocalInput(from), to: toLocalInput(to) });
  if (live) {
    return <span className={cn(pill, "min-w-[150px] cursor-default")}>Now</span>;
  }
  return (
    <Popover>
      <PopoverTrigger className={cn(pill, "min-w-[150px]")}>
        {rangeLabel(range)}
        <span aria-hidden className="text-faint">
          ⌄
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[230px] p-1.5">
        <p className="px-2.5 py-1.5 text-xs text-quiet">Time range</p>
        {RANGES.map((item) => (
          <button
            key={item.value}
            type="button"
            className={cn(
              "block w-full rounded-md px-2.5 py-2 text-left text-[13px] hover:bg-sub",
              range === item.value && "bg-sub",
            )}
            onClick={() => onChange({ range: item.value, from: "", to: "" })}
          >
            {item.label}
          </button>
        ))}
        <div className="mt-1.5 border-t border-line-soft px-2.5 pb-1.5 pt-2.5">
          <p className="mb-1.5 text-xs text-quiet">Custom</p>
          <div className="grid gap-1.5">
            <Input
              type="datetime-local"
              aria-label="From"
              value={custom.from}
              onChange={(event) => setCustom((c) => ({ ...c, from: event.target.value }))}
              onBlur={() =>
                custom.from &&
                onChange({
                  range: "custom",
                  from: new Date(custom.from).toISOString(),
                  to: custom.to ? new Date(custom.to).toISOString() : "",
                })
              }
            />
            <Input
              type="datetime-local"
              aria-label="To"
              value={custom.to}
              onChange={(event) => setCustom((c) => ({ ...c, to: event.target.value }))}
              onBlur={() =>
                custom.from &&
                onChange({
                  range: "custom",
                  from: new Date(custom.from).toISOString(),
                  to: custom.to ? new Date(custom.to).toISOString() : "",
                })
              }
            />
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
};

export const SourceSelect = ({
  value,
  options,
  onChange,
}: {
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
}) => (
  <Select
    aria-label="Source"
    className="min-w-[150px]"
    options={[{ value: "", label: "All sources" }, ...options]}
    value={value}
    onValueChange={onChange}
  />
);

export const ViewSwitch = ({
  value,
  onChange,
}: {
  value: string;
  onChange: (value: string) => void;
}) => (
  <Segmented
    aria-label="View"
    role="group"
    options={[
      { value: "list", label: "List" },
      { value: "live", label: "Live" },
      { value: "patterns", label: "Patterns" },
    ]}
    value={value}
    onValueChange={onChange}
  />
);
