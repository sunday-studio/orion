import { Button, GridRow, GridTable, Select } from "@/design-system";
import { cn } from "@/utils/cn";
import { formatDate } from "@/utils/date";
import { type LogRow, sourceColor } from "../log-model";
import type { LogKind } from "../logs-state";
import { LevelBadge } from "./log-states";

export const columnsFor = (kind: LogKind, plain: boolean) =>
  kind === "status"
    ? "92px 92px 110px 130px minmax(0,1fr)"
    : plain
      ? "112px 92px minmax(0,1fr)"
      : "112px 92px 140px minmax(0,1fr)";

const headers = (kind: LogKind, plain: boolean) =>
  kind === "status"
    ? ["Time", "Level", "Server", "Monitor", "Event"]
    : plain
      ? ["Time", "Severity", "Message"]
      : [
          "Time",
          kind === "internal" ? "Level" : "Severity",
          kind === "internal" ? "Component" : "Source",
          "Message",
        ];

export const LogRowCells = ({
  row,
  kind,
  plain,
}: {
  row: LogRow;
  kind: LogKind;
  plain: boolean;
}) => {
  const time = (
    <span className="tabular-nums text-faint">
      {formatDate(row.at, kind === "service" ? "HH:mm:ss.SSS" : "HH:mm:ss")}
    </span>
  );
  const message = (
    <span className="truncate">
      <b className="font-medium">{row.title}</b>
      {row.detail && <span className="ml-1.5 text-faint">{row.detail}</span>}
    </span>
  );
  if (kind === "status") {
    return (
      <>
        {time}
        <LevelBadge level={row.level} />
        <span className="text-faint">{row.label ?? "-"}</span>
        <span className="text-faint">{row.monitor ?? "-"}</span>
        {message}
      </>
    );
  }
  return (
    <>
      {time}
      <LevelBadge level={row.level} />
      {!plain && (
        <span className="flex items-center gap-1.5 truncate">
          {kind === "service" && row.label && (
            <i
              className="size-2 shrink-0 rounded-[2px]"
              style={{ background: sourceColor(row.label) }}
            />
          )}
          <span className={cn("truncate", kind === "internal" && "text-faint")}>
            {row.label ?? "-"}
          </span>
        </span>
      )}
      {message}
    </>
  );
};

export const LogTable = ({
  rows,
  kind,
  plain,
  fetching,
  onOpen,
  footer,
}: {
  rows: LogRow[];
  kind: LogKind;
  plain: boolean;
  fetching?: boolean;
  onOpen: (row: LogRow) => void;
  footer: React.ReactNode;
}) => {
  const columns = columnsFor(kind, plain);
  return (
    <div
      className={cn(
        "ds-settled min-w-0 flex-1 overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
        fetching && "ds-fetching",
      )}
    >
      <GridTable>
        <GridRow columns={columns} header>
          {headers(kind, plain).map((label) => (
            <span key={label}>{label}</span>
          ))}
        </GridRow>
        {rows.map((row) => (
          <GridRow
            key={row.id}
            flipId={row.id}
            columns={columns}
            className="cursor-pointer"
            onClick={() => onOpen(row)}
          >
            <LogRowCells row={row} kind={kind} plain={plain} />
          </GridRow>
        ))}
      </GridTable>
      {footer}
    </div>
  );
};

const ROWS = ["25", "50", "100", "200"];

export const LogPager = ({
  from,
  to,
  total,
  inferred,
  hasNewer,
  hasOlder,
  limit,
  onNewer,
  onOlder,
  onLimit,
}: {
  from: number;
  to: number;
  total?: number;
  inferred?: boolean;
  hasNewer: boolean;
  hasOlder: boolean;
  limit: string;
  onNewer: () => void;
  onOlder: () => void;
  onLimit: (limit: string) => void;
}) => (
  <div className="flex items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[12.5px] text-quiet">
    <span>
      Showing {from.toLocaleString()} to {to.toLocaleString()}
      {total !== undefined ? ` of ${total.toLocaleString()}` : ""}
    </span>
    {inferred && (
      <code className="rounded border border-hairline px-1.5 py-0.5 text-[11px]">
        severity inferred
      </code>
    )}
    <span className="flex-1" />
    <Button size="sm" variant="secondary" disabled={!hasNewer} onClick={onNewer}>
      Newer
    </Button>
    <Button size="sm" variant="secondary" disabled={!hasOlder} onClick={onOlder}>
      Older
    </Button>
    <Select
      aria-label="Rows per page"
      className="w-[110px]"
      options={ROWS.map((value) => ({ value, label: `${value} rows` }))}
      value={limit}
      onValueChange={onLimit}
    />
  </div>
);
