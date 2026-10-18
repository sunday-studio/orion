import { parseAsString, useQueryStates } from "nuqs";
import {
  AddFilterButton,
  CheckFilterPanel,
  FilterToken,
  FilterTokenRow,
  PopoverClose,
  PopoverContent,
} from "@/design-system";
import {
  DEFAULT_SINCE,
  DEFAULT_STATUSES,
  severityOptions,
  sinceOptions,
  statusOptions,
} from "../incident-format";

const csv = (value: string) => value.split(",").filter(Boolean);

export const useIncidentFilters = () => {
  const [state, setState] = useQueryStates({
    status: parseAsString.withDefault(DEFAULT_STATUSES.join(",")),
    severity: parseAsString.withDefault(""),
    since: parseAsString.withDefault(DEFAULT_SINCE),
    q: parseAsString.withDefault(""),
  });
  return {
    statuses: csv(state.status),
    severities: csv(state.severity),
    since: state.since,
    q: state.q,
    setStatuses: (next: string[]) => void setState({ status: next.join(",") || "none" }),
    setSeverities: (next: string[]) => void setState({ severity: next.join(",") }),
    setSince: (next: string) => void setState({ since: next }),
    setQuery: (next: string) => void setState({ q: next }),
    clearAll: () => void setState({ status: "", severity: "", since: "", q: "" }),
    isDefault:
      state.status === DEFAULT_STATUSES.join(",") &&
      state.severity === "" &&
      state.since === DEFAULT_SINCE,
  };
};

export type IncidentFilters = ReturnType<typeof useIncidentFilters>;

const labelList = (
  values: readonly string[],
  options: readonly { value: string; label: string }[],
) => values.map((v) => options.find((o) => o.value === v)?.label ?? v).join(", ") || "Any";

/** Applied filters as tokens plus an "add filter" menu (canvas incidents-list). */
export const IncidentFilterRow = ({ filters }: { filters: IncidentFilters }) => {
  const statuses = filters.statuses.filter((s) => s !== "none");
  return (
    <FilterTokenRow onClearAll={filters.clearAll}>
      <FilterToken
        field="Status"
        operator="is one of"
        value={labelList(statuses, statusOptions)}
        onRemove={() => filters.setStatuses(statusOptions.map((o) => o.value))}
      >
        <CheckFilterPanel
          title="Status"
          options={statusOptions}
          selected={statuses}
          onChange={filters.setStatuses}
          onDone={() => undefined}
        />
      </FilterToken>
      {filters.since && (
        <FilterToken
          field="Opened"
          operator="in"
          value={sinceOptions.find((o) => o.value === filters.since)?.label ?? filters.since}
          onRemove={() => filters.setSince("all")}
        >
          <PopoverContent className="w-[220px] p-1.5">
            {sinceOptions.map((option) => (
              <PopoverClose
                key={option.value}
                onClick={() => filters.setSince(option.value)}
                className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
              >
                {option.label}
              </PopoverClose>
            ))}
          </PopoverContent>
        </FilterToken>
      )}
      {filters.severities.length > 0 && (
        <FilterToken
          field="Severity"
          operator="is one of"
          value={labelList(filters.severities, severityOptions)}
          onRemove={() => filters.setSeverities([])}
        >
          <CheckFilterPanel
            title="Severity"
            options={severityOptions}
            selected={filters.severities}
            onChange={filters.setSeverities}
            onDone={() => undefined}
          />
        </FilterToken>
      )}
      <AddFilterButton>
        <PopoverContent className="w-[200px] p-1.5">
          <PopoverClose
            onClick={() => filters.setSeverities(severityOptions.map((o) => o.value))}
            className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
          >
            Severity
          </PopoverClose>
          <PopoverClose
            onClick={() => filters.setSince(filters.since || "7d")}
            className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
          >
            Opened
          </PopoverClose>
        </PopoverContent>
      </AddFilterButton>
    </FilterTokenRow>
  );
};
