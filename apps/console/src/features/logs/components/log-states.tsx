import { Link } from "react-router-dom";
import { Button, Callout, EmptyState, StatusBadge } from "@/design-system";
import { useGetLogIngestStatus } from "@/orion-sdk";
import { levelLabel, levelTone } from "../log-model";

export const LevelBadge = ({ level }: { level?: string }) => (
  <StatusBadge status={levelTone(level)}>{levelLabel(level)}</StatusBadge>
);

const LAG = (seconds?: number) => {
  const minutes = Math.max(Math.round((seconds ?? 0) / 60), 1);
  return minutes === 1 ? "about a minute" : `about ${minutes} minutes`;
};

/** "Ingest is behind" banner; renders only while Core reports a backlog. */
export const IngestBanner = () => {
  const status = useGetLogIngestStatus({ query: { refetchInterval: 30_000 } });
  const data = status.data;
  if (data?.storage_full) {
    return (
      <Callout tone="down" lead="Storage is full.">
        New logs are being dropped. Raise an app's size cap or delete old logs in Settings.
      </Callout>
    );
  }
  if (!data?.behind) return null;
  return (
    <Callout tone="warn" lead="Ingest is behind.">
      New logs may take {LAG(data.lag_seconds)} to appear.
      {data.rate_limited ? " Apps are being asked to slow down (429)." : ""}
    </Callout>
  );
};

export const SearchTimedOut = ({ onRetry }: { onRetry: () => void }) => (
  <EmptyState
    className="min-h-52 rounded-xl border-solid bg-card"
    title="The search timed out"
    description="Core took longer than 10 seconds. Narrow the time range or add a filter."
    action={<Button onClick={onRetry}>Retry</Button>}
  />
);

export const LoadFailed = ({ onRetry }: { onRetry: () => void }) => (
  <EmptyState
    tone="error"
    className="min-h-52 rounded-xl bg-card"
    title="Unable to load logs"
    action={
      <Button variant="secondary" onClick={onRetry}>
        Retry
      </Button>
    }
  />
);

export const NoMatches = ({
  range,
  onWiden,
  onClear,
}: {
  range: string;
  onWiden?: () => void;
  onClear: () => void;
}) => (
  <EmptyState
    className="min-h-52 rounded-xl border-solid bg-card"
    title="No logs match"
    description={`Nothing in ${range}. Try a wider range or fewer filters.`}
    action={
      <span className="flex gap-2">
        {onWiden && (
          <Button size="sm" variant="secondary" onClick={onWiden}>
            Last 24 hours
          </Button>
        )}
        <Button size="sm" variant="secondary" onClick={onClear}>
          Clear filters
        </Button>
      </span>
    }
  />
);

export const NoApps = () => (
  <EmptyState
    className="min-h-52 rounded-xl border-solid bg-card"
    title="No logs yet"
    description="Connect an app and its logs appear here within seconds. Any app works: curl, OpenTelemetry, or a plain log line."
    action={
      <Link to="/apps/new">
        <Button>Connect your first app</Button>
      </Link>
    }
  />
);
