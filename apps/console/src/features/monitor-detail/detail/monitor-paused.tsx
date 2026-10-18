import { Button, Callout, Dot, FieldPanel, RaisedCard, StatBox } from "@/design-system";
import type { ApiMonitorDetailSummary } from "@/orion-sdk";
import { TIME_FORMAT, formatDate } from "@/utils/date";

export const MonitorPaused = ({
  summary,
  pending,
  core = true,
  onResume,
}: {
  summary?: ApiMonitorDetailSummary;
  pending: boolean;
  core?: boolean;
  onResume: () => void;
}) => {
  const at = formatDate(summary?.paused_at, TIME_FORMAT);
  return (
    <div className="mt-2 flex flex-col gap-3">
      <Callout
        tone="down"
        className="flex items-center justify-between gap-3 rounded-[10px] bg-sub px-4 py-3 text-sm font-medium text-ink"
      >
        <span className="flex items-center gap-2.5">
          <Dot color="#a1a1aa" />
          This monitor is paused. It sends no checks and opens no incidents.
        </span>
        {core && (
          <Button size="sm" disabled={pending} onClick={onResume}>
            Resume now
          </Button>
        )}
      </Callout>
      <div className="grid grid-cols-3 gap-3">
        <StatBox
          label="Last result"
          value={
            summary?.last_result === "fail"
              ? "Fail"
              : summary?.last_result === "pass"
                ? "Pass"
                : "-"
          }
        />
        <StatBox label="Paused at" value={at} />
        <StatBox label="Resumes" value="Manually" />
      </div>
      <FieldPanel title="History while paused" aside="No checks were run">
        <RaisedCard>
          <div className="grid grid-cols-[12px_auto_1fr] items-center gap-3 px-3.5 py-2.5 text-[13px]">
            <Dot color="#a1a1aa" />
            <b className="font-medium">{at} to now</b>
            <span className="text-right text-faint">
              {summary?.paused_by ? `Paused by ${summary.paused_by}` : "Paused"}
            </span>
          </div>
        </RaisedCard>
      </FieldPanel>
    </div>
  );
};
