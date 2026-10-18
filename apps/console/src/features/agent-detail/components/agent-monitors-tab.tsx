import { parseAsString, useQueryState } from "nuqs";
import { Link } from "react-router-dom";
import {
  AddFilterButton,
  CheckFilterPanel,
  EmptyState,
  FilterToken,
  FilterTokenRow,
  GridFooter,
  GridRow,
  GridTable,
  ListPagination,
  PopoverClose,
  PopoverContent,
  Skeleton,
  useClientPagination,
} from "@/design-system";
import { useGetAgentMonitors } from "@/orion-sdk";
import { monitorHealth, monitorPriority } from "../agent-detail.domain";
import { MONITOR_COLUMNS, MonitorCells } from "./monitor-rows";

const STATUS_OPTIONS = [
  { value: "down", label: "Down" },
  { value: "degraded", label: "Degraded" },
  { value: "stale", label: "Stale" },
  { value: "unknown", label: "Unknown" },
  { value: "up", label: "Up" },
] as const;

const DEFAULT = "down,degraded,stale,unknown";

const labelFor = (values: readonly string[]) =>
  values.map((value) => STATUS_OPTIONS.find((o) => o.value === value)?.label ?? value).join(", ") ||
  "Any";

export const AgentMonitorsTab = ({ agentId }: { agentId: string }) => {
  const [statusParam, setStatusParam] = useQueryState(
    "mstatus",
    parseAsString.withDefault(DEFAULT),
  );
  const selected = statusParam.split(",").filter(Boolean);
  const query = useGetAgentMonitors(agentId, { limit: 200 });
  const all = [...(query.data?.monitors ?? [])].sort(
    (a, b) => monitorPriority(a) - monitorPriority(b),
  );
  const shown = all.filter((monitor) => selected.includes(monitorHealth(monitor)));
  const hidden = all.length - shown.length;
  const { pageItems, pagination } = useClientPagination(shown);
  const isDefault = statusParam === DEFAULT;

  if (query.isLoading) return <Skeleton className="h-40 w-full rounded-xl" />;
  if (query.isError) return <EmptyState tone="error" title="Unable to load monitors" />;

  return (
    <div className="flex flex-col gap-3">
      <FilterTokenRow onClearAll={() => void setStatusParam(DEFAULT)}>
        {selected.length > 0 && (
          <FilterToken
            field="Status"
            operator="is one of"
            value={labelFor(selected)}
            onRemove={() => void setStatusParam(STATUS_OPTIONS.map((o) => o.value).join(","))}
          >
            <CheckFilterPanel
              title="Status"
              options={STATUS_OPTIONS}
              selected={selected}
              onChange={(next) => void setStatusParam(next.join(","))}
              onDone={() => undefined}
            />
          </FilterToken>
        )}
        <AddFilterButton>
          <PopoverContent className="w-[200px] p-1.5">
            <PopoverClose
              onClick={() => void setStatusParam(selected.length ? statusParam : DEFAULT)}
              className="block w-full rounded-md px-2 py-1.5 text-left text-[13px] hover:bg-tray"
            >
              Status
            </PopoverClose>
          </PopoverContent>
        </AddFilterButton>
      </FilterTokenRow>
      {shown.length === 0 ? (
        <EmptyState
          title={isDefault ? "No unhealthy monitors" : "No monitors match"}
          description={
            hidden > 0
              ? `${hidden} monitors are hidden by the status filter.`
              : "No monitors are registered on this server."
          }
        />
      ) : (
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={MONITOR_COLUMNS} header>
              <span />
              <span>Monitor</span>
              <span>Type</span>
              <span>Status</span>
              <span>Detail</span>
            </GridRow>
            {pageItems.map((monitor) => (
              <GridRow key={monitor.id} columns={MONITOR_COLUMNS}>
                <MonitorCells monitor={monitor} />
              </GridRow>
            ))}
            {hidden > 0 && (
              <GridFooter>
                {hidden} {hidden === 1 ? "monitor is" : "monitors are"} hidden.{" "}
                <button
                  type="button"
                  className="underline"
                  onClick={() => void setStatusParam(STATUS_OPTIONS.map((o) => o.value).join(","))}
                >
                  Show them
                </button>{" "}
                ·{" "}
                <Link className="underline" to={`/monitors?target=server:${agentId}`}>
                  Open in Monitors
                </Link>
              </GridFooter>
            )}
          </GridTable>
          <ListPagination {...pagination} />
        </div>
      )}
    </div>
  );
};
