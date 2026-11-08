import { Box, BoxHeader, Skeleton } from "@/design-system";
import { useNow } from "@/features/overview/overview-time";
import { useGetService, useGetServiceLogCounts } from "@/orion-sdk";
import { errorsPerMinute, niceMax } from "../timeline-model";

const W = 900;
const H = 72;
const LEFT = 40;
const DAY = 86_400_000;

/** App errors per minute for the last day, from the apps behind the monitor's service. Drawn under the response chart. */
export const MonitorErrorsLane = ({ serviceId }: { serviceId: string }) => {
  const now = useNow(60_000);
  const service = useGetService(serviceId);
  const apps = service.data?.service?.apps ?? [];
  const counts = useGetServiceLogCounts(serviceId, { from: "-24h", step_seconds: 900 });
  if (apps.length === 0) return null;
  const points = errorsPerMinute(counts.data?.buckets ?? [], 900);
  const peak = niceMax(Math.max(0, ...points.map((point) => point.v)));
  const x = (t: number) => LEFT + ((t - (now - DAY)) / DAY) * (W - LEFT);
  const y = (v: number) => H - (Math.min(v, peak) / peak) * H;
  return (
    <Box>
      <BoxHeader
        title="App errors per minute"
        aside={<span className="text-xs text-quiet">Last 24 hours</span>}
      />
      <div className="px-3 py-2.5">
        {counts.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : points.some((point) => point.v > 0) ? (
          <svg
            viewBox={`0 0 ${W} ${H + 4}`}
            role="img"
            aria-label="App errors per minute over the last day"
            className="block w-full"
          >
            <line x1={LEFT} x2={W} y1={H} y2={H} className="stroke-hairline" />
            <text x={LEFT - 6} y={10} textAnchor="end" className="fill-faint" fontSize="10">
              {peak}
            </text>
            <text x={LEFT - 6} y={H} textAnchor="end" className="fill-faint" fontSize="10">
              0
            </text>
            <g fill="#e5484d" opacity=".85">
              {points.map((point) => (
                <rect
                  key={point.t}
                  x={x(point.t) - 2}
                  y={y(point.v)}
                  width={4}
                  height={Math.max(H - y(point.v), point.v > 0 ? 1 : 0)}
                />
              ))}
            </g>
          </svg>
        ) : (
          <p className="text-[13px] text-faint">
            No error logs from the linked apps in the last day.
          </p>
        )}
      </div>
    </Box>
  );
};
