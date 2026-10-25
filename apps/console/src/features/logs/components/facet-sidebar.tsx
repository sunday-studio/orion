import { Checkbox } from "@/design-system";
import type { ApiLogFieldStat } from "@/orion-sdk";
import type { FacetGroup, FacetItem } from "../use-log-data";
import { type LogQueryState, inCsv, toggleCsv } from "../logs-state";

const compact = (n: number) =>
  n >= 10_000 ? `${Math.round(n / 1000)}k` : n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

const caption = "text-[11px] font-medium uppercase tracking-[0.08em] text-faint";

/** Which state key each facet group writes to. */
const KEY: Record<string, keyof LogQueryState> = {
  severity: "severity",
  env: "env",
  source: "source",
  event: "event",
  type: "type",
  server: "server",
  monitor: "monitor",
  component: "component",
};

const Row = ({
  item,
  checked,
  onToggle,
}: {
  item: FacetItem;
  checked: boolean;
  onToggle: () => void;
}) => (
  <label
    className={`flex items-center gap-2 py-[5px] text-[13px] ${
      item.disabled && !checked ? "cursor-not-allowed opacity-45" : "cursor-pointer"
    }`}
  >
    <Checkbox checked={checked} disabled={item.disabled && !checked} onCheckedChange={onToggle} />
    <span className="min-w-0 flex-1 truncate">{item.label}</span>
    <span className="text-xs tabular-nums text-faint">{compact(item.count)}</span>
  </label>
);

export const FacetSidebar = ({
  groups,
  fields,
  fieldValues,
  state,
  plainOnly,
  onChange,
  onPickField,
  activeField,
  onTextParser,
  showAll,
  onShowAll,
}: {
  groups: FacetGroup[];
  fields: ApiLogFieldStat[];
  fieldValues: FacetItem[];
  state: LogQueryState;
  plainOnly: boolean;
  onChange: (patch: Partial<LogQueryState>) => void;
  onPickField: (key: string | undefined) => void;
  activeField?: string;
  onTextParser?: () => void;
  showAll: boolean;
  onShowAll: (showAll: boolean) => void;
}) => {
  return (
    <aside className="flex w-[190px] shrink-0 flex-col gap-5">
      {groups
        .filter((group) => group.items.length > 0)
        .map((group) => {
          const key = KEY[group.key];
          const shown = showAll ? group.items : group.items.slice(0, 4);
          const hidden =
            Math.max(group.items.length - shown.length, 0) + (showAll ? 0 : (group.more ?? 0));
          return (
            <section key={group.key}>
              <h3 className={`${caption} mb-1.5`}>{group.title}</h3>
              {shown.map((item) => (
                <Row
                  key={item.value}
                  item={item}
                  checked={inCsv(String(state[key] ?? ""), item.value)}
                  onToggle={() =>
                    onChange({
                      [key]:
                        key === "severity"
                          ? toggleCsv(state.severity, item.value)
                          : state[key] === item.value
                            ? ""
                            : item.value,
                    } as Partial<LogQueryState>)
                  }
                />
              ))}
              {(hidden > 0 || showAll) && group.items.length + (group.more ?? 0) > 4 && (
                <button
                  type="button"
                  className="py-1 text-[13px] text-quiet hover:text-ink"
                  onClick={() => onShowAll(!showAll)}
                >
                  {showAll ? "Show fewer" : `Show ${hidden} more`}
                </button>
              )}
            </section>
          );
        })}
      {fields.length > 0 && !plainOnly && (
        <section>
          <h3 className={`${caption} mb-1.5`}>Fields from the field catalog</h3>
          {fields.slice(0, 8).map((field) => (
            <div key={field.key}>
              <button
                type="button"
                className="flex w-full items-center gap-2 py-[5px] text-left text-[13px] hover:text-ink"
                onClick={() => onPickField(activeField === field.key ? undefined : field.key)}
              >
                <span className="min-w-0 flex-1 truncate">{field.key}</span>
                <code className="rounded bg-sub px-1 text-[10px] text-faint">
                  {field.type === "number" ? "num" : "str"}
                </code>
                <span className="text-xs tabular-nums text-faint">
                  {compact(field.cardinality ?? 0)}
                </span>
              </button>
              {activeField === field.key &&
                fieldValues.map((value) => (
                  <Row
                    key={value.value}
                    item={value}
                    checked={state.attr === `${field.key}:${value.value}`}
                    onToggle={() =>
                      onChange({
                        attr:
                          state.attr === `${field.key}:${value.value}`
                            ? ""
                            : `${field.key}:${value.value}`,
                      })
                    }
                  />
                ))}
            </div>
          ))}
        </section>
      )}
      {plainOnly && (
        <section>
          <h3 className={`${caption} mb-1.5`}>Fields</h3>
          <p className="text-[13px] leading-[1.5] text-quiet">
            This app sends plain text, so there are no fields to filter on. Severity was read from
            the text.
          </p>
          {onTextParser && (
            <button type="button" className="mt-1 text-[13px] underline" onClick={onTextParser}>
              Set up a text parser
            </button>
          )}
        </section>
      )}
    </aside>
  );
};
