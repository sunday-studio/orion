import { useEffect, useMemo, useState } from "react";
import { useGetAgents, type GetAgentsParams } from "@/orion-sdk";
import { AgentFilters, type AttentionFilterValue } from "./agent-filters";
import { AgentRow } from "./agent-row";
import { Separator } from "@/components/ui/separator";
import { Fragment } from "react/jsx-runtime";
import { ListPagination } from "./list-pagination";

const SERVER_LIMIT = 20;

export const AgentList = () => {
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("all");
  const [maintenanceOnly, setMaintenanceOnly] = useState(false);
  const [staleOnly, setStaleOnly] = useState(false);
  const [hasIncidents, setHasIncidents] = useState(false);
  const [offset, setOffset] = useState(0);

  const params = useMemo<GetAgentsParams>(
    () => ({
      limit: SERVER_LIMIT,
      offset,
      search: search.trim() || undefined,
      status: status === "all" ? undefined : status,
      maintenance: maintenanceOnly ? "true" : undefined,
      stale_only: staleOnly || undefined,
      has_incidents: hasIncidents || undefined,
    }),
    [hasIncidents, maintenanceOnly, offset, search, staleOnly, status],
  );

  const agentsResponse = useGetAgents(params);
  const agents = agentsResponse.data?.agents ?? [];
  const count = agentsResponse.data?.count ?? agents.length;
  const hasFilters =
    search.trim() !== "" || status !== "all" || maintenanceOnly || staleOnly || hasIncidents;
  const selectedAttentionFilters = useMemo<AttentionFilterValue[]>(
    () =>
      [
        maintenanceOnly ? "maintenance" : undefined,
        staleOnly ? "stale" : undefined,
        hasIncidents ? "incidents" : undefined,
      ].filter((value): value is AttentionFilterValue => value !== undefined),
    [hasIncidents, maintenanceOnly, staleOnly],
  );

  const setAttentionFilters = (values: AttentionFilterValue[]) => {
    const selected = new Set(values);
    setMaintenanceOnly(selected.has("maintenance"));
    setStaleOnly(selected.has("stale"));
    setHasIncidents(selected.has("incidents"));
  };

  const clearFilters = () => {
    setSearch("");
    setStatus("all");
    setMaintenanceOnly(false);
    setStaleOnly(false);
    setHasIncidents(false);
    setOffset(0);
  };

  useEffect(() => {
    setOffset(0);
  }, [hasIncidents, maintenanceOnly, search, staleOnly, status]);

  return (
    <div className="space-y-3">
      <AgentFilters
        search={search}
        status={status}
        selectedAttentionFilters={selectedAttentionFilters}
        hasFilters={hasFilters}
        onSearchChange={setSearch}
        onStatusChange={setStatus}
        onAttentionFiltersChange={setAttentionFilters}
        onClear={clearFilters}
      />
      {agentsResponse.isLoading && (
        <div className="py-3 text-sm text-neutral-600">Loading servers...</div>
      )}
      {agentsResponse.error && <div className="py-3 text-sm">Unable to load servers.</div>}
      {!agentsResponse.isLoading && !agentsResponse.error && agents.length === 0 && (
        <div className="py-3 text-sm text-neutral-600">No servers match these filters.</div>
      )}
      {agents.map((agent, index) => (
        <Fragment key={agent.id}>
          <AgentRow key={agent.id} agent={agent} />
          {index < agents.length - 1 && <Separator />}{" "}
        </Fragment>
      ))}
      <ListPagination
        count={count}
        limit={SERVER_LIMIT}
        offset={offset}
        onOffsetChange={setOffset}
      />
    </div>
  );
};
