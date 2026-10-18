import { useState } from "react";
import { EmptyState, Segmented, Skeleton, StatusBadge } from "@/design-system";
import { usePreviewStatusPage } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { publicStatusLabel, publicStatusTone, statusDot } from "../status-format";

export const PreviewTab = ({ pageId, url }: { pageId: string; url: string }) => {
  const [device, setDevice] = useState("desktop");
  const preview = usePreviewStatusPage(pageId);
  if (preview.isPending) return <Skeleton className="h-64 w-full rounded-xl" />;
  const data = preview.data?.preview;
  if (preview.isError || !data) {
    return <EmptyState tone="error" title="Unable to load the preview" />;
  }
  return (
    <div className="flex flex-col gap-3">
      <Segmented
        aria-label="Device"
        role="group"
        options={[
          { value: "desktop", label: "Desktop" },
          { value: "mobile", label: "Mobile" },
        ]}
        value={device}
        onValueChange={setDevice}
      />
      <div
        className={cn(
          "overflow-hidden rounded-xl border border-hairline bg-card shadow-card",
          device === "mobile" && "mx-auto w-[375px] max-w-full",
        )}
      >
        <div className="border-b border-hairline bg-field px-4 py-2 text-xs text-faint">{url}</div>
        <div className="grid gap-4 p-6">
          <div>
            <h2 className="text-xl font-medium">{data.page?.title}</h2>
            <div className="mt-2">
              <StatusBadge status={publicStatusTone(data.overall_status)}>
                {data.overall_status_display ?? publicStatusLabel(data.overall_status)}
              </StatusBadge>
            </div>
          </div>
          {(data.sections ?? []).map((section) => (
            <section key={section.id} className="grid gap-2">
              <h3 className="text-[13px] font-medium text-quiet">{section.name}</h3>
              {(section.components ?? []).map((component) => (
                <div
                  key={component.id}
                  className="flex items-center justify-between gap-3 rounded-lg border border-line-soft px-3.5 py-2.5 text-[13px]"
                >
                  <span className="flex min-w-0 items-center gap-2.5">
                    <span
                      className="size-2 shrink-0 rounded-full"
                      style={{ background: statusDot(component.status) }}
                    />
                    <b className="truncate font-medium">{component.name}</b>
                  </span>
                  <span className="text-xs text-faint">
                    {component.status_display ?? publicStatusLabel(component.status)}
                  </span>
                </div>
              ))}
            </section>
          ))}
        </div>
      </div>
    </div>
  );
};
