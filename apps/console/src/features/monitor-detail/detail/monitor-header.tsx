import { Link } from "react-router-dom";
import { Button, StatusBadge } from "@/design-system";
import type { ApiMonitorDetailSummary, ApiMonitorResponse } from "@/orion-sdk";
import { everyLabel } from "@/features/monitors/monitor-format";

type Props = {
  monitor: ApiMonitorResponse;
  summary?: ApiMonitorDetailSummary;
  paused: boolean;
  pending: boolean;
  testing: boolean;
  onTest: () => void;
  onPause: () => void;
  onResume: () => void;
  breadcrumb?: boolean;
};

export const MonitorHeader = ({
  monitor,
  summary,
  paused,
  pending,
  testing,
  onTest,
  onPause,
  onResume,
  breadcrumb = true,
}: Props) => {
  const status = paused ? "paused" : (summary?.status ?? monitor.health ?? "unknown");
  const label = paused ? "Paused" : (summary?.status_text ?? status);
  const target = summary?.target ?? monitor.target;
  const every = everyLabel(summary?.interval_seconds ?? monitor.interval_seconds);
  return (
    <div className="flex flex-wrap items-start justify-between gap-x-3 gap-y-2.5 pb-2 pt-7">
      <div className="min-w-0">
        {breadcrumb && (
          <div className="text-[13.5px] leading-[1.4] text-quiet">
            <Link to="/monitors" className="hover:text-ink">
              Monitors
            </Link>{" "}
            / {monitor.name}
          </div>
        )}
        <h1 className="text-[28px] font-medium leading-10 tracking-[-0.025em]">{monitor.name}</h1>
        <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[13.5px] text-quiet">
          <StatusBadge status={status} shape={status === "down" ? "square" : undefined}>
            {label}
          </StatusBadge>
          <span>
            {[target, paused ? undefined : every !== "-" ? `checked every ${every}` : undefined]
              .filter(Boolean)
              .join(" · ")}
          </span>
        </div>
      </div>
      <div className="flex items-center gap-1.5">
        <Button size="sm" variant="secondary" disabled={pending} onClick={onTest}>
          {testing ? "Testing..." : "Test now"}
        </Button>
        {paused ? (
          <Button size="sm" disabled={pending} onClick={onResume}>
            Resume
          </Button>
        ) : (
          <>
            <Button size="sm" variant="secondary" disabled={pending} onClick={onPause}>
              Pause
            </Button>
            <Link to={`/monitors/${monitor.id}/edit`}>
              <Button size="sm" variant="secondary">
                Configure
              </Button>
            </Link>
          </>
        )}
      </div>
    </div>
  );
};
