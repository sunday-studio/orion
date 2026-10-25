import { useEffect, useRef, useState } from "react";
import { Button, GridRow, GridTable } from "@/design-system";
import { cn } from "@/utils/cn";
import type { LogKind } from "../logs-state";
import { LogRowCells, columnsFor } from "./log-table";
import type { LiveRow } from "../use-log-stream";

/** Live tail: newest first, brief highlight on arrival, pauses when scrolled away. */
export const LiveView = ({
  rows,
  kind,
  connected,
  paused,
  onPause,
  onClear,
  onOpen,
}: {
  rows: LiveRow[];
  kind: LogKind;
  connected: boolean;
  paused: boolean;
  onPause: () => void;
  onClear: () => void;
  onOpen: (row: LiveRow) => void;
}) => {
  const [now, setNow] = useState(Date.now());
  const columns = columnsFor(kind, false);
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => {
    timer.current = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer.current);
  }, []);
  const recent = rows.filter((row) => now - row.arrivedAt < 5000).length;
  return (
    <div className="grid gap-3">
      <div className="flex items-center gap-2.5 rounded-lg bg-[var(--tone-up-bg)] px-3.5 py-2.5 text-[13px] text-[var(--tone-up-fg)]">
        <i
          className={cn(
            "size-2 rounded-full bg-[#2a9f69]",
            connected && !paused && "animate-pulse",
            !connected && "bg-[#a1a1aa]",
          )}
        />
        <b className="font-semibold">
          {paused ? "Paused" : connected ? "Streaming" : "Reconnecting"}
        </b>
        <span>
          {paused
            ? "New events are not shown while paused."
            : `${recent} new in the last few seconds, matching your filter`}
        </span>
        <span className="flex-1" />
        <Button size="sm" variant="secondary" onClick={onPause}>
          {paused ? "Resume" : "Pause"}
        </Button>
        <Button size="sm" variant="secondary" onClick={onClear}>
          Clear
        </Button>
      </div>
      <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <GridTable>
          <GridRow columns={columns} header>
            <span>Time</span>
            <span>{kind === "status" ? "Level" : "Severity"}</span>
            {kind === "status" ? (
              <>
                <span>Server</span>
                <span>Monitor</span>
                <span>Event</span>
              </>
            ) : (
              <>
                <span>{kind === "internal" ? "Component" : "Source"}</span>
                <span>Message</span>
              </>
            )}
          </GridRow>
          {rows.length === 0 && (
            <p className="px-3.5 py-8 text-center text-[13px] text-quiet">
              Waiting for events that match your filter.
            </p>
          )}
          {rows.map((row) => (
            <GridRow
              key={`${row.id}-${row.arrivedAt}`}
              columns={columns}
              className={cn(
                "cursor-pointer",
                now - row.arrivedAt < 3000 && "bg-[var(--tone-up-bg)]/60",
              )}
              onClick={() => onOpen(row)}
            >
              <LogRowCells row={row} kind={kind} plain={false} />
            </GridRow>
          ))}
        </GridTable>
      </div>
    </div>
  );
};
