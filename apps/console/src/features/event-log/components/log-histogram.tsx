import { Skeleton } from "@/design-system";
import { type GetLogHistogramParams, useGetLogHistogram } from "@/orion-sdk";

type LogHistogramProps = { params: GetLogHistogramParams };

const SEVERITY_COLORS: Record<string, string> = {
  fatal: "bg-dot-down",
  error: "bg-dot-down",
  warn: "bg-dot-warn",
  info: "bg-dot-up",
  debug: "bg-dot-idle",
  trace: "bg-dot-idle",
};

/** Volume over the selected window, stacked by severity. */
export const LogHistogram = ({ params }: LogHistogramProps) => {
  const query = useGetLogHistogram(params);
  const buckets = query.data?.buckets ?? [];
  const max = Math.max(1, ...buckets.map((bucket) => bucket.total ?? 0));

  if (query.isLoading) return <Skeleton className="h-16 w-full" />;
  if (query.isError) return <p className="text-xs text-quiet">Volume chart unavailable.</p>;
  if (buckets.length === 0) return null;

  return (
    <figure aria-label="Log volume by severity" className="space-y-1">
      <div className="flex h-16 items-end gap-px">
        {buckets.map((bucket) => (
          <div
            key={bucket.start}
            title={`${bucket.start}: ${bucket.total ?? 0}`}
            className="flex min-w-0 flex-1 flex-col-reverse"
            style={{ height: `${((bucket.total ?? 0) / max) * 100}%` }}
          >
            {Object.entries(bucket.severity ?? {}).map(([severity, count]) => (
              <div
                key={severity}
                className={SEVERITY_COLORS[severity] ?? "bg-dot-idle"}
                style={{ height: `${(count / Math.max(1, bucket.total ?? 1)) * 100}%` }}
              />
            ))}
          </div>
        ))}
      </div>
      <figcaption className="text-xs text-faint">
        As of {new Date().toLocaleTimeString()}, bucket {query.data?.bucket_seconds ?? 0}s
      </figcaption>
    </figure>
  );
};
