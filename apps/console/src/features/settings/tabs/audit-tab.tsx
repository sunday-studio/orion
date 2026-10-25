import { keepPreviousData } from "@tanstack/react-query";
import { parseAsString, useQueryStates } from "nuqs";
import { useState } from "react";
import {
  Button,
  Callout,
  EmptyState,
  FilterToken,
  GridRow,
  GridTable,
  Select,
  Skeleton,
} from "@/design-system";
import { type ApiAuditEntry, useListAuditLog } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { AuditExport } from "./audit-export";

const COLUMNS = "110px 160px minmax(0,1fr) 160px";

const when = (iso?: string) => {
  if (!iso) return "-";
  const date = new Date(iso);
  const days = Math.floor((Date.now() - date.getTime()) / 86_400_000);
  if (days < 1 && date.getDate() === new Date().getDate()) return formatDate(iso, "HH:mm");
  return days < 2 ? "Yesterday" : formatDate(iso, "MMM d");
};

export const AuditTab = ({
  exportOpen,
  onExportOpen,
}: {
  exportOpen: boolean;
  onExportOpen: (open: boolean) => void;
}) => {
  const [{ actor }, setQuery] = useQueryStates({ actor: parseAsString.withDefault("") });
  const [extra, setExtra] = useState<ApiAuditEntry[]>([]);
  const [cursor, setCursor] = useState<string>();
  const query = useListAuditLog(
    { actor: actor || undefined, limit: 25 },
    { query: { placeholderData: keepPreviousData } },
  );
  const more = useListAuditLog(
    { actor: actor || undefined, limit: 25, cursor },
    { query: { enabled: Boolean(cursor) } },
  );
  const base = query.data?.entries ?? [];
  const entries = [...base, ...extra, ...(cursor ? (more.data?.entries ?? []) : [])];
  const next = cursor ? more.data?.next_cursor : query.data?.next_cursor;
  const actors = query.data?.actors ?? [];

  const showMore = () => {
    if (!next) return;
    if (cursor) setExtra((current) => [...current, ...(more.data?.entries ?? [])]);
    setCursor(next);
  };
  const reset = (value: string) => {
    setExtra([]);
    setCursor(undefined);
    void setQuery({ actor: value });
  };

  return (
    <div className="flex flex-col gap-3.5">
      <div className="flex flex-wrap items-center gap-2">
        <FilterToken
          field="Actor"
          operator="is"
          value={actor || "Anyone"}
          onRemove={actor ? () => reset("") : undefined}
        />
        <Select
          aria-label="Actor"
          className="min-w-[150px]"
          options={[
            { value: "", label: "Anyone" },
            ...actors.map((item) => ({
              value: item.actor ?? "",
              label: `${item.actor} (${item.count})`,
            })),
          ]}
          value={actor}
          onValueChange={reset}
        />
        {actor && (
          <button
            type="button"
            className="ml-auto text-[13px] text-quiet hover:text-ink"
            onClick={() => reset("")}
          >
            Clear all
          </button>
        )}
      </div>
      {query.isPending ? (
        <Skeleton className="h-60 w-full rounded-xl" />
      ) : query.isError ? (
        <Callout tone="down">Unable to load the audit log.</Callout>
      ) : entries.length === 0 ? (
        <EmptyState
          className="rounded-xl border-solid bg-card"
          title="No audit entries"
          description="Changes made in the console or through API tokens show up here."
        />
      ) : (
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>Time</span>
              <span>Actor</span>
              <span>Action</span>
              <span>Target</span>
            </GridRow>
            {entries.map((entry) => (
              <GridRow key={entry.id} columns={COLUMNS}>
                <span className="text-faint">{when(entry.at)}</span>
                <b className="font-medium">{entry.actor}</b>
                <span>{entry.action}</span>
                <span className="text-faint">{entry.target}</span>
              </GridRow>
            ))}
          </GridTable>
          {next && (
            <div className="border-t border-line-soft py-2 text-center">
              <Button size="sm" variant="secondary" disabled={more.isFetching} onClick={showMore}>
                Show more
              </Button>
            </div>
          )}
        </div>
      )}
      <AuditExport open={exportOpen} onOpenChange={onExportOpen} actor={actor} />
    </div>
  );
};
