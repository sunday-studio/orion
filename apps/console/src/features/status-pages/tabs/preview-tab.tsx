import { useState } from "react";
import {
  ChartTooltip,
  EmptyState,
  Segmented,
  Skeleton,
  StatusBadge,
  usePlotHover,
} from "@/design-system";
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
  const { index, bind } = usePlotHover(ticks.length);
  const active = index === null ? undefined : ticks[index];
  const x = index === null ? 0 : ((index + 0.5) / ticks.length) * 100;
  return (
    <div className="relative min-w-0">
      <div {...bind} className="flex h-[34px] gap-[2px]">
        {ticks.map((tick, i) => (
          <i
            key={tick.date ?? i}
            className={cn(
              "min-w-0 flex-1 rounded-[2px] transition-[opacity,transform] duration-150",
              i === index ? "scale-y-110 opacity-100" : "opacity-85",
            )}
            style={{ background: tick.status ? statusDot(tick.status) : "#2a9f69" }}
          />
        ))}
      </div>
      {active && (
        <ChartTooltip x={x} className="!top-auto bottom-[calc(100%+6px)]">
          {[
            active.date ? formatDate(active.date, "MMM d") : undefined,
            active.status ? publicStatusLabel(active.status) : "No data",
            active.uptime_display,
          ]
            .filter(Boolean)
            .join(" · ")}
        </ChartTooltip>
      )}
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
  const sections = data.sections ?? [];
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
        <div
          className={cn(
            "grid grid-cols-[minmax(0,1fr)] gap-3.5 bg-card py-7",
            device === "mobile" ? "px-5" : "px-8",
          )}
        >
          <h2 className="text-[28px] font-medium leading-tight tracking-[-0.02em]">
            {headline(data.overall_status)}
          </h2>
          {sections.map((section) => (
            <section key={section.id} className="grid min-w-0 gap-2">
              <h3 className="text-[13px] font-medium text-quiet">{section.name}</h3>
              <div className="overflow-hidden rounded-xl border border-hairline">
                {(section.components ?? []).map((component) => (
                  <div
                    key={component.id}
                    className="flex min-w-0 flex-col gap-3 border-t border-hairline p-3.5 first:border-t-0"
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <i
                        className="size-2 shrink-0 rounded-full"
                        style={{ background: statusDot(component.status) }}
                      />
                      <span className="min-w-0 flex-1 break-words text-[13.5px] font-medium">
                        {component.name}
                        {(component.status_reason || component.description) && (
                          <small className="block text-xs font-normal text-quiet">
                            {component.status_reason || component.description}
                          </small>
                        )}
                      </span>
                      <span className="shrink-0 text-[13px] text-quiet">
                        {component.uptime?.uptime_display ?? ""}
                      </span>
                      <StatusBadge className="shrink-0" status={publicStatusTone(component.status)}>
                        {component.status_display ?? publicStatusLabel(component.status)}
                      </StatusBadge>
                    </div>
                    <UptimeBar history={component.uptime_history} />
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
};
