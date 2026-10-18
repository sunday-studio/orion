import { useState } from "react";
import { EmptyState, Segmented, Skeleton, StatusBadge, Tooltip } from "@/design-system";
import { usePreviewStatusPage } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { formatDate } from "@/utils/date";
import { publicStatusLabel, publicStatusTone, statusDot } from "../status-format";

const headline = (status?: string) =>
  status === "operational" || status === "up"
    ? "All systems operational"
    : status === "maintenance"
      ? "Scheduled maintenance in progress"
      : status
        ? "Some systems are experiencing issues"
        : "Status unavailable";

type Tick = { date?: string; status?: string; uptime_display?: string };

/** Day ticks for one component; hovering a day shows its date, status and uptime. */
const UptimeBar = ({ history }: { history?: Tick[] }) => {
  const ticks: Tick[] =
    history && history.length > 0 ? history : Array.from({ length: 90 }, () => ({}));
  return (
    <div className="flex h-[18px] gap-[2px]">
      {ticks.map((tick, index) => (
        <Tooltip
          key={tick.date ?? index}
          content={
            <span className="grid gap-0.5">
              {tick.date && <b className="font-medium">{formatDate(tick.date, "MMM d, yyyy")}</b>}
              <span>
                {tick.status ? publicStatusLabel(tick.status) : "No data"}
                {tick.uptime_display ? ` · ${tick.uptime_display}` : ""}
              </span>
            </span>
          }
        >
          <i
            className="min-w-[2px] flex-1 rounded-[2px] opacity-85 transition-[opacity,transform] duration-150 hover:scale-y-110 hover:opacity-100"
            style={{ background: tick.status ? statusDot(tick.status) : "#2a9f69" }}
          />
        </Tooltip>
      ))}
    </div>
  );
};

export const PreviewTab = ({
  pageId,
  url,
  published,
}: {
  pageId: string;
  url: string;
  published: boolean;
}) => {
  const [device, setDevice] = useState("desktop");
  const preview = usePreviewStatusPage(pageId);
  if (preview.isPending) return <Skeleton className="h-64 w-full rounded-xl" />;
  const data = preview.data?.preview;
  if (preview.isError || !data)
    return <EmptyState tone="error" title="Unable to load the preview" />;
  const components = (data.sections ?? []).flatMap((section) => section.components ?? []);
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <span className="text-[13px] text-quiet">
          {published
            ? "Live preview. This is what visitors see."
            : "Draft preview, not public. This is what visitors will see."}
        </span>
        <Segmented
          aria-label="Device"
          role="group"
          surface="page"
          options={[
            { value: "desktop", label: "Desktop" },
            { value: "mobile", label: "Mobile" },
          ]}
          value={device}
          onValueChange={setDevice}
        />
      </div>
      <div
        className={cn(
          "overflow-hidden rounded-xl border border-hairline shadow-[0_1px_2px_rgba(0,0,0,0.05)]",
          device === "mobile" && "mx-auto w-[375px] max-w-full",
        )}
      >
        <div className="border-b border-hairline bg-field px-3.5 py-2 font-mono text-xs text-faint">
          {url}
        </div>
        <div className="grid gap-3.5 bg-card px-8 py-7 max-sm:px-5">
          <h2 className="text-[28px] font-medium leading-tight tracking-[-0.02em]">
            {headline(data.overall_status)}
          </h2>
          {components.map((component) => (
            <div key={component.id} className="grid gap-[5px]">
              <div className="flex items-center justify-between gap-3 text-[13.5px]">
                <b className="min-w-0 truncate font-medium">{component.name}</b>
                <StatusBadge status={publicStatusTone(component.status)}>
                  {component.status_display ?? publicStatusLabel(component.status)}
                </StatusBadge>
              </div>
              <UptimeBar history={component.uptime_history} />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};
