import { parseAsInteger, parseAsString, useQueryStates } from "nuqs";
import { useGetNodes, useGetMonitors } from "@/orion-sdk";
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
  notificationOptions,
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
    notified: parseAsString.withDefault(""),
    server: parseAsString.withDefault(""),
    monitor: parseAsString.withDefault(""),
    review: parseAsString.withDefault(""),
    page: parseAsInteger.withDefault(1),
  });
  return {
    statuses: csv(state.status),
    severities: csv(state.severity),
    since: state.since,
    q: state.q,
    page: Math.max(state.page, 1),
    setPage: (next: number) => void setState({ page: next }),
    notified: csv(state.notified),
    server: state.server,
    monitor: state.monitor,
    review: state.review === "1",
    setNotified: (next: string[]) => void setState({ notified: next.join(","), page: 1 }),
    setServer: (next: string) => void setState({ server: next, page: 1 }),
    setMonitor: (next: string) => void setState({ monitor: next, page: 1 }),
    /** Quick views: Active, Needs review, Resolved. */
    setView: (view: "active" | "review" | "resolved") =>
      void setState({
        status:
          view === "resolved"
            ? "resolved"
            : view === "review"
              ? "open,acknowledged,covered,resolved"
              : DEFAULT_STATUSES.join(","),
        review: view === "review" ? "1" : "",
        since: view === "active" ? DEFAULT_SINCE : "",
        severity: "",
        notified: "",
        server: "",
        monitor: "",
        page: 1,
      }),
    setStatuses: (next: string[]) => void setState({ status: next.join(",") || "none", page: 1 }),
    setSeverities: (next: string[]) => void setState({ severity: next.join(","), page: 1 }),
    setSince: (next: string) => void setState({ since: next, page: 1 }),
    setQuery: (next: string) => void setState({ q: next, page: 1 }),
    clearAll: () =>
      void setState({
        status: "",
        severity: "",
        since: "",
        q: "",
        notified: "",
        server: "",
        monitor: "",
        review: "",
        page: 1,
      }),
    isDefault:
      state.status === DEFAULT_STATUSES.join(",") &&
      state.severity === "" &&
      state.since === DEFAULT_SINCE &&
      state.notified === "" &&
      state.server === "" &&
      state.monitor === "" &&
      state.review === "",
  };
};

export type IncidentFilters = ReturnType<typeof useIncidentFilters>;

const labelList = (
  values: readonly string[],
  options: readonly { value: string; label: string }[],
) => values.map((v) => options.find((o) => o.value === v)?.label ?? v).join(", ") || "Any";

/** Applied filters as tokens plus an "add filter" menu (canvas incidents-list). */
const PickList = ({
  options,
  onPick,
}: {
  options: readonly { value: string; label: string }[];
  onPick: (value: string) => void;
}) => (
  <PopoverContent className="max-h-[260px] w-[220px] overflow-auto p-1.5">
    {options.map((option) => (
      <PopoverClose
        key={option.value}
        onClick={() => onPick(option.value)}
        className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
      >
        {option.label}
      </PopoverClose>
    ))}
  </PopoverContent>
);

export const IncidentFilterRow = ({ filters }: { filters: IncidentFilters }) => {
  const nodes = useGetNodes({ limit: 100 });
  const monitorList = useGetMonitors({ limit: 100 });
  const servers = (nodes.data?.nodes ?? []).map((a) => ({
    value: a.id ?? "",
    label: a.name ?? a.id ?? "Server",
  }));
  const monitors = (monitorList.data?.monitors ?? []).map((m) => ({
    value: m.id ?? "",
    label: m.name ?? m.id ?? "Monitor",
  }));
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
      {filters.notified.length > 0 && (
        <FilterToken
          field="Notification"
          operator="is one of"
          value={labelList(filters.notified, notificationOptions)}
          onRemove={() => filters.setNotified([])}
        >
          <CheckFilterPanel
            title="Notification"
            options={notificationOptions}
            selected={filters.notified}
            onChange={filters.setNotified}
            onDone={() => undefined}
          />
        </FilterToken>
      )}
      {filters.server && (
        <FilterToken
          field="Server"
          operator="is"
          value={servers.find((o) => o.value === filters.server)?.label ?? filters.server}
          onRemove={() => filters.setServer("")}
        >
          <PickList options={servers} onPick={filters.setServer} />
        </FilterToken>
      )}
      {filters.monitor && (
        <FilterToken
          field="Monitor"
          operator="is"
          value={monitors.find((o) => o.value === filters.monitor)?.label ?? filters.monitor}
          onRemove={() => filters.setMonitor("")}
        >
          <PickList options={monitors} onPick={filters.setMonitor} />
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
          <PopoverClose
            onClick={() => filters.setNotified(notificationOptions.map((o) => o.value))}
            className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
          >
            Notification
          </PopoverClose>
          <PopoverClose
            onClick={() => servers[0] && filters.setServer(servers[0].value)}
            className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
          >
            Server
          </PopoverClose>
          <PopoverClose
            onClick={() => monitors[0] && filters.setMonitor(monitors[0].value)}
            className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
          >
            Monitor
          </PopoverClose>
        </PopoverContent>
      </AddFilterButton>
    </FilterTokenRow>
  );
};
