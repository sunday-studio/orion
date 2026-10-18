import { EmptyState, GridRow, GridTable, Skeleton, Sparkline, StatusBadge } from "@/design-system";
import type { ApiLogPattern } from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { levelTone } from "../log-model";
import { LevelBadge } from "./log-states";

const COLUMNS = "minmax(0,1fr) 90px 90px 110px 130px 56px";

/** Highlights <placeholder> tokens the way the artboard italicises them. */
const PatternText = ({ text }: { text: string }) => (
  <span className="truncate">
    {text.split(/(<[^>]+>)/g).map((part, index) =>
      part.startsWith("<") ? (
        <i key={index} className="font-normal text-quiet">
          {part}
        </i>
      ) : (
        <b key={index} className="font-medium">
          {part}
        </b>
      ),
    )}
  </span>
);

const trendColor = (level?: string) =>
  levelTone(level) === "down" ? "#e5484d" : levelTone(level) === "degraded" ? "#e5484d" : "#2a9f69";

export const PatternsView = ({
  patterns,
  pending,
  onOpen,
}: {
  patterns: ApiLogPattern[];
  pending: boolean;
  onOpen: (pattern: ApiLogPattern) => void;
}) => {
  if (pending) return <Skeleton className="h-48 w-full rounded-xl" />;
  if (patterns.length === 0)
    return (
      <EmptyState
        className="min-h-40 rounded-xl border-solid bg-card"
        title="No patterns yet"
        description="Patterns appear once there are enough similar lines in this range."
      />
    );
  return (
    <div className="grid gap-3">
      <p className="text-[13px] text-quiet">
        Orion groups similar messages, so a flood of one error shows as one row. Open a pattern to
        see its events.
      </p>
      <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <GridTable>
          <GridRow columns={COLUMNS} header>
            <span>Pattern</span>
            <span>Level</span>
            <span>Events</span>
            <span>Trend</span>
            <span>Last seen</span>
            <span />
          </GridRow>
          {patterns.map((pattern) => (
            <GridRow
              key={pattern.pattern}
              columns={COLUMNS}
              className="cursor-pointer"
              onClick={() => onOpen(pattern)}
            >
              <PatternText text={pattern.pattern ?? ""} />
              <LevelBadge level={pattern.severity} />
              <span className="tabular-nums text-faint">
                {(pattern.count ?? 0).toLocaleString()}
              </span>
              <Sparkline
                values={pattern.trend ?? []}
                color={trendColor(pattern.severity)}
                width={72}
                height={16}
              />
              <span className="text-faint">{formatDate(pattern.last_seen, "MMM d HH:mm")}</span>
              {pattern.is_new ? <StatusBadge status="down">New</StatusBadge> : <span />}
            </GridRow>
          ))}
        </GridTable>
      </div>
    </div>
  );
};
