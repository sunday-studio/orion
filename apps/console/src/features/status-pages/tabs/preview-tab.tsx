import { useState } from "react";
import { EmptyState, Segmented, Skeleton, StatusBadge } from "@/design-system";
import { usePreviewStatusPage } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { publicStatusLabel, publicStatusTone, statusDot } from "../status-format";

const headline = (status?: string) =>
  status === "operational" || status === "up"
    ? "All systems operational"
    : status === "maintenance"
      ? "Scheduled maintenance in progress"
      : status
        ? "Some systems are experiencing issues"
        : "Status unavailable";

/** Day ticks for one component, falling back to an all-green strip when there is no history. */
const UptimeBar = ({ history }: { history?: { status?: string }[] }) => {
  const ticks: { status?: string }[] =
    history && history.length > 0 ? history : Array.from({ length: 90 }, () => ({}));
  return (
    <div aria-hidden className="flex h-[18px] gap-[2px]">
      {ticks.map((tick, index) => (
        <i
          key={index}
          className="min-w-[2px] flex-1 rounded-[2px]"
          style={{ background: tick.status ? statusDot(tick.status) : "#2a9f69", opacity: 0.85 }}
        />
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
