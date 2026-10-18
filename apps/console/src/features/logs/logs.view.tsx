import { useState } from "react";
import { Link } from "react-router-dom";
import {
  Button,
  FilterToken,
  PageHeader,
  Skeleton,
  Tabs,
  TabsList,
  TabsTab,
  useToast,
} from "@/design-system";
import { useListApps } from "@/orion-sdk";
import { ExportDialog } from "./components/export-dialog";
import { FacetSidebar } from "./components/facet-sidebar";
import { LiveView } from "./components/live-view";
import { LogDetailDrawer } from "./components/log-detail-drawer";
import { LoadFailed, NoApps, NoMatches, SearchTimedOut } from "./components/log-states";
import { IngestBanner } from "./components/log-states";
import { LogPager, LogTable } from "./components/log-table";
import { RangePicker, SearchBox, SourceSelect, ViewSwitch } from "./components/log-toolbar";
import { PatternsView } from "./components/patterns-view";
import { ViewsMenu } from "./components/views-menu";
import { VolumeChart } from "./components/volume-chart";
import {
  type LogKind,
  type LogQueryState,
  filterParams,
  rangeLabel,
  toQueryString,
  useLogQuery,
} from "./logs-state";
import {
  isSearchTimeout,
  useLogFacets,
  useLogHistogram,
  useLogList,
  useLogPatterns,
} from "./use-log-data";
import { useLogStream } from "./use-log-stream";

const SUBTITLE: Record<LogKind, string> = {
  service: "Everything your apps and agents report",
  status: "What Orion itself saw: checks, incidents, alerts, servers",
  internal: "Internal logs of the Orion backend: API, worker, scheduler, agent gateway",
};
const PLACEHOLDER: Record<LogKind, string> = {
  service: "free text, or field:value, like level:error user.id:u_41",
  status: "level:error server:edge-01 monitor:api-https",
  internal: "component:worker level:warning,error",
};

const FILTER_KEYS: (keyof LogQueryState)[] = [
  "q",
  "source",
  "severity",
  "event",
  "type",
  "server",
  "monitor",
  "component",
  "attr",
];

const SAVED_KEYS = ["app_id", "severity", "event", "q", "from", "to", "attr"];

/** Saved views only accept Core's filter names, so map the screen state onto them. */
const savedFilters = (state: LogQueryState) =>
  Object.fromEntries(
    Object.entries(filterParams(state, "service")).filter(
      (entry): entry is [string, string] => SAVED_KEYS.includes(entry[0]) && Boolean(entry[1]),
    ),
  );

const fromSavedFilters = (filters: Record<string, string>): Partial<LogQueryState> => ({
  tab: "service",
  view: "list",
  q: filters.q ?? "",
  source: filters.app_id ?? "",
  severity: filters.severity ?? "",
  event: filters.event ?? "",
  attr: filters.attr ?? "",
  ...(filters.from?.startsWith("-")
    ? { range: filters.from.slice(1), from: "", to: "" }
    : { range: "custom", from: filters.from ?? "", to: filters.to ?? "" }),
});

const CLEARED = Object.fromEntries(FILTER_KEYS.map((key) => [key, ""])) as Partial<LogQueryState>;

export const LogsPage = () => {
  const { state, set } = useLogQuery();
  const toast = useToast();
  const kind = (
    ["service", "status", "internal"].includes(state.tab) ? state.tab : "service"
  ) as LogKind;
  const view = state.view === "live" || state.view === "patterns" ? state.view : "list";
  const limit = Number(state.limit) || 100;
  const [exporting, setExporting] = useState(false);
  const [paused, setPaused] = useState(false);
  const [fieldKey, setFieldKey] = useState<string>();

  const apps = useListApps();
  const list = useLogList(kind, state, limit, view === "list");
  const facets = useLogFacets(kind, state, fieldKey);
  const histogram = useLogHistogram(kind, state);
  const patterns = useLogPatterns(kind, state, view === "patterns");
  const stream = useLogStream(kind, state, view === "live", paused);

  const filtersActive = FILTER_KEYS.some((key) => state[key] !== "");
  const clear = () => void set({ ...CLEARED });
  const patch = (next: Partial<LogQueryState>) => void set(next);
  const noApps = kind === "service" && apps.isSuccess && (apps.data?.apps ?? []).length === 0;
  const params = filterParams(state, kind);
  const query = toQueryString(params as Record<string, string | undefined>);
  const sourceOptions = facets.groups.find((group) => group.key === "source")?.items ?? [];
  const selectedSource = sourceOptions.find((item) => item.value === state.source);
  const plainOnly =
    kind === "service" &&
    Boolean(state.source) &&
    facets.fields.length === 0 &&
    list.rows.length > 0 &&
    list.rows.every((row) => row.plain);
  const filterSummary = [
    state.q && `search:${state.q}`,
    state.severity && `level:${state.severity}`,
    selectedSource && `source:${selectedSource.label}`,
    rangeLabel(state.range).toLowerCase(),
  ]
    .filter(Boolean)
    .join(", ");

  const switchTab = (tab: string) =>
    void set({ ...CLEARED, tab, view: "list", log: "", limit: state.limit });
  const tokens: { field: string; value: string; clear: Partial<LogQueryState> }[] = [
    selectedSource && { field: "Source", value: selectedSource.label, clear: { source: "" } },
    state.severity && {
      field: kind === "service" ? "Severity" : "Level",
      value: state.severity.split(",").join(", "),
      clear: { severity: "" },
    },
    state.event && { field: "Event", value: state.event, clear: { event: "" } },
    state.type && { field: "Type", value: state.type, clear: { type: "" } },
    state.server && {
      field: "Server",
      value:
        facets.groups.find((g) => g.key === "server")?.items.find((i) => i.value === state.server)
          ?.label ?? state.server,
      clear: { server: "" },
    },
    state.monitor && {
      field: "Monitor",
      value:
        facets.groups.find((g) => g.key === "monitor")?.items.find((i) => i.value === state.monitor)
          ?.label ?? state.monitor,
      clear: { monitor: "" },
    },
    state.component && { field: "Component", value: state.component, clear: { component: "" } },
    state.attr && { field: "Field", value: state.attr.split(",").join(", "), clear: { attr: "" } },
  ].filter(Boolean) as { field: string; value: string; clear: Partial<LogQueryState> }[];

  const actions = (
    <>
      {kind === "service" && view === "list" && (
        <Button
          size="sm"
          variant="secondary"
          onClick={() => {
            void navigator.clipboard.writeText(window.location.href);
            toast.add({ title: "Link copied" });
          }}
        >
          Copy link
        </Button>
      )}
      <Button size="sm" variant="secondary" onClick={() => setExporting(true)}>
        Export
      </Button>
      <ViewsMenu
        current={savedFilters(state)}
        onApply={(filters) => void set({ ...CLEARED, ...fromSavedFilters(filters), log: "" })}
      />
      {kind === "service" && view === "list" && (
        <Button size="sm" variant="secondary" onClick={() => patch({ view: "live" })}>
          <i className="mr-1.5 inline-block size-1.5 rounded-full bg-[#2a9f69]" />
          Live tail
        </Button>
      )}
    </>
  );

  const body = () => {
    if (view === "live") {
      return (
        <LiveView
          rows={stream.rows}
          kind={kind}
          connected={stream.connected}
          paused={paused}
          onPause={() => setPaused((value) => !value)}
          onClear={stream.clear}
          onOpen={(row) => patch({ log: row.id })}
        />
      );
    }
    if (view === "patterns") {
      if (isSearchTimeout(patterns.error)) return <SearchTimedOut onRetry={patterns.refetch} />;
      if (patterns.isError) return <LoadFailed onRetry={patterns.refetch} />;
      return (
        <PatternsView
          patterns={patterns.patterns}
          pending={patterns.isPending}
          onOpen={(pattern) =>
            patch({ view: "list", q: (pattern.pattern ?? "").replace(/<[^>]+>/g, " ").trim() })
          }
        />
      );
    }
    if (list.isError) {
      return isSearchTimeout(list.error) ? (
        <SearchTimedOut onRetry={list.refetch} />
      ) : (
        <LoadFailed onRetry={list.refetch} />
      );
    }
    const first = list.page * limit + 1;
    return (
      <div className="flex flex-col gap-4">
        {kind !== "internal" && (
          <VolumeChart
            title={kind === "status" ? "Events per 15 minutes" : "Volume"}
            data={histogram.data}
            pending={histogram.isPending}
            summary={
              facets.total === undefined ? undefined : `${facets.total.toLocaleString()} lines`
            }
            onZoom={(from, to) => patch({ range: "custom", from, to })}
          />
        )}
        <div className="flex items-start gap-6">
          <FacetSidebar
            groups={facets.groups}
            fields={facets.fields}
            fieldValues={facets.values}
            state={state}
            plainOnly={plainOnly}
            activeField={fieldKey}
            onPickField={setFieldKey}
            onChange={patch}
          />
          <div className="min-w-0 flex-1">
            {list.isPending ? (
              <Skeleton className="h-60 w-full rounded-xl" />
            ) : list.rows.length === 0 ? (
              <NoMatches
                range={rangeLabel(state.range).toLowerCase()}
                onWiden={
                  state.range === "24h"
                    ? undefined
                    : () => patch({ range: "24h", from: "", to: "" })
                }
                onClear={clear}
              />
            ) : (
              <LogTable
                rows={list.rows}
                kind={kind}
                plain={plainOnly}
                fetching={list.isPlaceholderData}
                onOpen={(row) => patch({ log: row.id })}
                footer={
                  <LogPager
                    from={first}
                    to={first + list.rows.length - 1}
                    total={kind === "status" ? list.total : facets.total}
                    inferred={list.rows.some((row) => row.inferred)}
                    hasNewer={list.hasNewer}
                    hasOlder={list.hasOlder}
                    limit={String(limit)}
                    onNewer={list.newer}
                    onOlder={list.older}
                    onLimit={(next) => patch({ limit: next })}
                  />
                }
              />
            )}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="w-full pb-10">
      <PageHeader
        title="Logs"
        description={
          view === "live"
            ? "Live tail"
            : view === "patterns"
              ? `Patterns, ${rangeLabel(state.range).toLowerCase()}`
              : SUBTITLE[kind]
        }
        actions={
          noApps ? (
            <Link to="/apps/new">
              <Button size="sm">Connect an app</Button>
            </Link>
          ) : (
            actions
          )
        }
      />
      <div className="flex justify-center pb-4 pt-3">
        <Tabs value={kind} onValueChange={switchTab}>
          <TabsList>
            <TabsTab value="service">Service logs</TabsTab>
            <TabsTab value="status">Status logs</TabsTab>
            <TabsTab value="internal">Internal logs</TabsTab>
          </TabsList>
        </Tabs>
      </div>
      {noApps ? (
        <NoApps />
      ) : (
        <div className="flex flex-col gap-4">
          {kind === "service" && <IngestBanner />}
          <div className="flex flex-wrap items-center gap-3">
            <SearchBox
              value={state.q}
              placeholder={PLACEHOLDER[kind]}
              onChange={(q) => patch({ q })}
            />
            {kind === "service" && view === "list" && (
              <SourceSelect
                value={state.source}
                options={sourceOptions.map((item) => ({ value: item.value, label: item.label }))}
                onChange={(source) => patch({ source })}
              />
            )}
            <RangePicker
              range={state.range}
              from={state.from}
              to={state.to}
              live={view === "live"}
              onChange={(next) =>
                patch({ range: next.range, from: next.from ?? "", to: next.to ?? "" })
              }
            />
            {kind !== "service" && (
              <ViewSwitch value={view} onChange={(next) => patch({ view: next })} />
            )}
          </div>
          {kind === "service" && view !== "list" && (
            <ViewSwitch value={view} onChange={(next) => patch({ view: next })} />
          )}
          {tokens.length > 0 && view === "list" && (
            <div className="flex flex-wrap items-center gap-2">
              {tokens.map((token) => (
                <FilterToken
                  key={token.field}
                  field={token.field}
                  operator="is"
                  value={token.value}
                  onRemove={() => patch(token.clear)}
                />
              ))}
              {filtersActive && (
                <button
                  type="button"
                  className="ml-auto text-[13px] text-quiet hover:text-ink"
                  onClick={clear}
                >
                  Clear all
                </button>
              )}
            </div>
          )}
          {body()}
        </div>
      )}
      <LogDetailDrawer
        id={state.log}
        kind={kind}
        statusRow={list.rows.find((row) => row.id === state.log)}
        onClose={() => patch({ log: "" })}
        onFilter={patch}
      />
      <ExportDialog
        open={exporting}
        kind={kind}
        query={query}
        filterSummary={filterSummary}
        onOpenChange={setExporting}
      />
    </div>
  );
};
