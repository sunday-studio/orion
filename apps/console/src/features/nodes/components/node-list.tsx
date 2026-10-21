import { Search } from "lucide-react";
import { parseAsString, useQueryStates } from "nuqs";
import { useMemo } from "react";
import { Link, useNavigate } from "react-router-dom";
import {
  Button,
  EmptyState,
  GridRow,
  GridTable,
  Input,
  ListPagination,
  Segmented,
  Select,
  Skeleton,
  Sparkline,
  StatusBadge,
  useClientPagination,
} from "@/design-system";
import { formatWhen, useNow } from "@/features/overview/overview-time";
import { useGetNodes } from "@/orion-sdk";
import {
  type ServerRow,
  byAttention,
  needsAttention,
  percent,
  platformLabel,
  serverState,
  stateDot,
  statusText,
} from "../server-format";

const COLUMNS = "minmax(0,1.1fr) 110px 120px 90px 60px 60px";

const badgeStatus = (row: ServerRow) => {
  const state = serverState(row);
  return state === "maintenance" ? "maintenance" : state;
};

export const NodeList = () => {
  const navigate = useNavigate();
  const [{ q, view, platform }, setQuery] = useQueryStates({
    q: parseAsString.withDefault(""),
    view: parseAsString.withDefault("all"),
    platform: parseAsString.withDefault(""),
  });
  const now = useNow(60_000);
  const nodesQuery = useGetNodes({ limit: 200, include_system: false });
  const all = useMemo(
    () => [...((nodesQuery.data?.nodes ?? []) as ServerRow[])].sort(byAttention),
    [nodesQuery.data],
  );
  const attention = all.filter(needsAttention).length;
  const platforms = [...new Set(all.map(platformLabel))].sort();

  const text = q.trim().toLowerCase();
  const rows = all.filter(
    (node) =>
      (view !== "attention" || needsAttention(node)) &&
      (!platform || platformLabel(node) === platform) &&
      (!text || (node.name ?? "").toLowerCase().includes(text)),
  );
  const { pageItems, pagination } = useClientPagination(rows);

  if (nodesQuery.isLoading) return <Skeleton className="mt-3 h-40 w-full rounded-xl" />;
  if (nodesQuery.isError) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load servers"
        description="Retry after Core is reachable."
        action={
          <Button size="sm" variant="secondary" onClick={() => void nodesQuery.refetch()}>
            Retry
          </Button>
        }
      />
    );
  }

  if (all.length === 0) {
    return (
      <div className="mt-3 flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <b className="text-[15px] font-medium">No servers yet</b>
        <p className="text-[13px] text-mid">
          Install the node to get CPU, memory, disk and process monitors.
        </p>
        <Link to="/servers/new" className="mt-3">
          <Button size="sm">Add your first server</Button>
        </Link>
      </div>
    );
  }

  return (
    <div className="mt-3 flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-[260px]">
          <Search
            aria-hidden
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-faint"
          />
          <Input
            aria-label="Search servers"
            placeholder="Search servers"
            className="pl-9"
            value={q}
            onChange={(event) => void setQuery({ q: event.target.value })}
          />
        </div>
        <Segmented
          role="group"
          aria-label="Server view"
          options={[
            { value: "all", label: `All ${all.length}` },
            { value: "attention", label: `Needs attention ${attention}` },
          ]}
          value={view}
          onValueChange={(next) => void setQuery({ view: next })}
        />
        <span className="flex-1" />
        <Select
          aria-label="Platform"
          label="Platform"
          className="min-w-[130px]"
          options={[
            { value: "", label: "Any" },
            ...platforms.map((item) => ({ value: item, label: item })),
          ]}
          value={platform}
          onValueChange={(next) => void setQuery({ platform: next })}
        />
      </div>
      {rows.length === 0 ? (
        <EmptyState
          title="No servers match"
          description="Clear the search or filters to see every server."
          action={
            <Button
              size="sm"
              variant="secondary"
              onClick={() => void setQuery({ q: "", view: "all", platform: "" })}
            >
              Clear filters
            </Button>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>Server</span>
              <span>Platform</span>
              <span>Status</span>
              <span>CPU</span>
              <span>Memory</span>
              <span>Report</span>
            </GridRow>
            {pageItems.map((node) => (
              <GridRow
                key={node.id}
                flipId={node.id}
                columns={COLUMNS}
                className="cursor-pointer"
                onClick={() => navigate(`/servers/${node.id}`)}
              >
                <Link
                  to={`/servers/${node.id}`}
                  className="truncate font-medium outline-none hover:underline focus-visible:underline"
                  onClick={(event) => event.stopPropagation()}
                >
                  {node.name ?? node.id}
                </Link>
                <span className="font-mono text-xs text-faint">{platformLabel(node)}</span>
                <span>
                  <StatusBadge status={badgeStatus(node)}>{statusText(node, now)}</StatusBadge>
                </span>
                <span>
                  {node.cpu_series && node.cpu_series.length > 1 ? (
                    <Sparkline values={node.cpu_series} color={stateDot[serverState(node)]} />
                  ) : (
                    <span className="text-faint">-</span>
                  )}
                </span>
                <span className="text-faint">{percent(node.memory_percent)}</span>
                <span className="text-faint">{formatWhen(node.last_seen, now)}</span>
              </GridRow>
            ))}
          </GridTable>
          <ListPagination {...pagination} />
        </div>
      )}
    </div>
  );
};
