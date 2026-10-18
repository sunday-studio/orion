import { useState } from "react";
import { Button, Drawer, DrawerContent, DrawerTitle, StatusBadge, useToast } from "@/design-system";
import type { ApiMonitorReportResponse } from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const pretty = (payload?: string) => {
  if (!payload) return "";
  try {
    return JSON.stringify(JSON.parse(payload), null, 2);
  } catch {
    return payload;
  }
};

type Props = {
  monitorName: string;
  report?: ApiMonitorReportResponse;
  hasPrev: boolean;
  hasNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  onClose: () => void;
};

/** Check detail: time, result, duration, source and the raw payload (canvas report-drawer). */
export const MonitorCheckDrawer = ({
  monitorName,
  report,
  hasPrev,
  hasNext,
  onPrev,
  onNext,
  onClose,
}: Props) => {
  const toast = useToast();
  const [lastReport, setLastReport] = useState(report);
  if (report && report !== lastReport) setLastReport(report);
  const shown = report ?? lastReport;
  const copy = async (text: string, label: string) => {
    await navigator.clipboard.writeText(text);
    toast.add({ title: `${label} copied` });
  };
  const json = pretty(shown?.payload);
  const passed = shown?.passed ?? shown?.health === "up";
  const rows: [string, string][] = [
    ["Monitor", monitorName],
    [
      "Duration",
      typeof shown?.duration_ms === "number" ? `${shown.duration_ms.toLocaleString()} ms` : "-",
    ],
    ["Reported by", shown?.reported_by ?? "-"],
  ];
  return (
    <Drawer
      open={Boolean(report)}
      onOpenChange={(open) => !open && onClose()}
      swipeDirection="right"
    >
      <DrawerContent className="w-[440px] gap-3 p-5">
        <div className="flex items-center gap-2 pr-9">
          <DrawerTitle className="pr-0 text-[15px]">
            Check {formatDate(shown?.collected_at ?? shown?.created_at, "HH:mm:ss")}
          </DrawerTitle>
          <StatusBadge status={passed ? "up" : "down"} shape={passed ? undefined : "square"}>
            {passed ? "Pass" : "Fail"}
          </StatusBadge>
          <span className="flex-1" />
          <Button size="sm" variant="secondary" disabled={!hasPrev} onClick={onPrev}>
            Prev
          </Button>
          <Button size="sm" variant="secondary" disabled={!hasNext} onClick={onNext}>
            Next
          </Button>
        </div>
        <div className="grid grid-cols-[110px_1fr] gap-x-3 gap-y-1.5 border-t border-line-soft pt-3 text-[13px] [&>:nth-child(odd)]:text-quiet">
          {rows.map(([label, value]) => (
            <div key={label} className="contents">
              <span>{label}</span>
              <span className="min-w-0 truncate">{value}</span>
            </div>
          ))}
        </div>
        <h3 className="text-[13px] font-medium">Raw payload</h3>
        <pre className="m-0 max-h-[320px] overflow-auto rounded-lg bg-[#142a23] px-3 py-3 font-mono text-[12.5px] leading-[1.6] text-[#e6f1ec]">
          {json || "No payload recorded."}
        </pre>
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="secondary"
            disabled={!json}
            onClick={() => void copy(json, "JSON")}
          >
            Copy JSON
          </Button>
        </div>
      </DrawerContent>
    </Drawer>
  );
};
