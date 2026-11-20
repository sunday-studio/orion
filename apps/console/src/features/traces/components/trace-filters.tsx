import { useState } from "react";
import { Input, Select } from "@/design-system";
import { RangePicker, SearchBox } from "@/features/logs/components/log-toolbar";
import type { ApiTraceFacetsResponse } from "@/orion-sdk";
import { normalizeMinDuration } from "../trace-format";
import {
  MAX_TRACE_ATTRS,
  TRACE_RANGES,
  type TraceFilters,
  addAttr,
  attrDraft,
  statusCounts,
  traceRangeLabel,
} from "../traces-state";

type Option = { value: string; label: string };

/** Options plus the current value when it is not among them (a link set it, or it fell off the top 50). */
const withCurrent = (options: Option[], value: string, label?: string): Option[] =>
  value && !options.some((option) => option.value === value)
    ? [...options, { value, label: label ?? value }]
    : options;

const MIN_DURATIONS: Option[] = [
  { value: "", label: "Any duration" },
  { value: "50ms", label: "At least 50ms" },
  { value: "100ms", label: "At least 100ms" },
  { value: "250ms", label: "At least 250ms" },
  { value: "500ms", label: "At least 500ms" },
  { value: "1s", label: "At least 1s" },
  { value: "2s", label: "At least 2s" },
  { value: "5s", label: "At least 5s" },
];

const ATTR_HINTS = {
  invalid: "Use key:value, for example http.status_code:500",
  full: `At most ${MAX_TRACE_ATTRS} attributes; remove one to add another`,
} as const;

/**
 * key:value box that adds one more attribute to the filter. It commits on Enter or blur, clears
 * itself, and says so when the text is not a pair. The pairs added so far are the tokens under the toolbar.
 */
const AttributeBox = ({
  value,
  onChange,
}: {
  value: string[];
  onChange: (value: string[]) => void;
}) => {
  const [draft, setDraft] = useState("");
  const state = attrDraft(value, draft);
  const hint = state === "invalid" || state === "full" ? ATTR_HINTS[state] : undefined;
  const commit = () => {
    if (state === "valid") {
      onChange(addAttr(value, draft));
      setDraft("");
    }
  };
  return (
    <div className="grid gap-1">
      <Input
        aria-label="Add attribute filter, as key:value"
        aria-invalid={hint ? true : undefined}
        aria-describedby={hint ? "trace-attr-hint" : undefined}
        placeholder={value.length > 0 ? "add another  key:value" : "attribute  key:value"}
        className="w-52"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => event.key === "Enter" && commit()}
      />
      {hint && (
        <span id="trace-attr-hint" role="status" className="text-xs text-(--tone-down-fg)">
          {hint}
        </span>
      )}
    </div>
  );
};

export const TraceToolbar = ({
  filters,
  facets,
  apps,
  onChange,
}: {
  filters: TraceFilters;
  facets?: ApiTraceFacetsResponse;
  apps: Option[];
  onChange: (patch: Partial<TraceFilters>) => void;
}) => {
  const counts = statusCounts(facets?.status);
  const services: Option[] = [
    { value: "", label: "All services" },
    ...(facets?.services ?? []).flatMap((item) =>
      item.value
        ? [{ value: item.value, label: `${item.value} (${(item.count ?? 0).toLocaleString()})` }]
        : [],
    ),
  ];
  const minDuration = normalizeMinDuration(filters.minDuration);
  return (
    <div role="search" aria-label="Filter traces" className="flex flex-wrap items-start gap-3">
      <div className="min-w-[220px] flex-1">
        <SearchBox
          label="Search root span names"
          value={filters.q}
          placeholder="root span name, like POST /checkout"
          onChange={(q) => onChange({ q })}
        />
      </div>
      <Select
        aria-label="Service"
        className="min-w-[150px]"
        options={withCurrent(services, filters.service)}
        value={filters.service}
        onValueChange={(service) => onChange({ service })}
      />
      <Select
        aria-label="App"
        className="min-w-[130px]"
        options={withCurrent([{ value: "", label: "All apps" }, ...apps], filters.app)}
        value={filters.app}
        onValueChange={(app) => onChange({ app })}
      />
      <Select
        aria-label="Status"
        className="min-w-[130px]"
        options={[
          { value: "", label: "Any status" },
          { value: "error", label: `Errors (${counts.error.toLocaleString()})` },
          { value: "ok", label: `OK (${counts.ok.toLocaleString()})` },
        ]}
        value={filters.status === "ok" || filters.status === "error" ? filters.status : ""}
        onValueChange={(status) => onChange({ status })}
      />
      <Select
        aria-label="Minimum duration"
        className="min-w-[140px]"
        options={withCurrent(MIN_DURATIONS, minDuration, `At least ${minDuration}`)}
        value={minDuration}
        onValueChange={(next) => onChange({ minDuration: next })}
      />
      <AttributeBox value={filters.attr} onChange={(attr) => onChange({ attr })} />
      <RangePicker
        range={filters.range}
        from={filters.from}
        to={filters.to}
        ranges={TRACE_RANGES}
        labelOf={traceRangeLabel}
        onChange={(next) =>
          onChange({ range: next.range, from: next.from ?? "", to: next.to ?? "" })
        }
      />
    </div>
  );
};
