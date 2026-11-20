import { Link } from "react-router-dom";
import { Button, EmptyState } from "@/design-system";
import { errorStatus } from "../use-trace-data";

const card = "min-h-52 rounded-xl bg-card";

/** The one error state of both trace screens: timeout, traces off, or a plain failure. */
export const TraceLoadError = ({
  error,
  what = "traces",
  onRetry,
}: {
  error: unknown;
  what?: string;
  onRetry: () => void;
}) => {
  const status = errorStatus(error);
  if (status === 504) {
    return (
      <EmptyState
        className={`${card} border-solid`}
        title="The search timed out"
        description="Core took longer than 10 seconds. Narrow the time range or add a filter."
        action={<Button onClick={onRetry}>Retry</Button>}
      />
    );
  }
  if (status === 503) {
    return (
      <EmptyState
        tone="error"
        className={card}
        title="Traces are not available"
        description="Core has traces turned off, or the trace database is out of space."
        action={
          <Button variant="secondary" onClick={onRetry}>
            Retry
          </Button>
        }
      />
    );
  }
  return (
    <EmptyState
      tone="error"
      className={card}
      title={`Unable to load ${what}`}
      description={(error as Error | null)?.message}
      action={
        <Button variant="secondary" onClick={onRetry}>
          Retry
        </Button>
      }
    />
  );
};

export const NoTraces = ({
  range,
  filtered,
  onWiden,
  onClear,
}: {
  range: string;
  filtered: boolean;
  onWiden?: () => void;
  onClear: () => void;
}) => (
  <EmptyState
    className={`${card} border-solid`}
    title={filtered ? "No traces match" : "No traces yet"}
    description={
      filtered
        ? `Nothing in ${range}. Try a wider range or fewer filters.`
        : `Nothing in ${range}. Point an OpenTelemetry traces exporter at an app's ingest key and traces appear here within seconds.`
    }
    action={
      <span className="flex flex-wrap justify-center gap-2">
        {onWiden && (
          <Button size="sm" variant="secondary" onClick={onWiden}>
            Last 24 hours
          </Button>
        )}
        {filtered ? (
          <Button size="sm" variant="secondary" onClick={onClear}>
            Clear filters
          </Button>
        ) : (
          <Link to="/logs/apps">
            <Button size="sm" variant="secondary">
              Open Apps
            </Button>
          </Link>
        )}
      </span>
    }
  />
);
