import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  StatusBadge,
} from "@/design-system";
import { useState } from "react";
import type { ApiMonitorTestRunResponse } from "@/orion-sdk";

const caption = "font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]";

export const MonitorTestResultDialog = ({
  result: latest,
  running,
  error,
  onRunAgain,
  onClose,
}: {
  result?: ApiMonitorTestRunResponse;
  running?: boolean;
  /** Core's message when the last run was rejected (rate limit, bad target, ...). */
  error?: string;
  onRunAgain: () => void;
  onClose: () => void;
}) => {
  // Keep showing the previous result while a rerun is pending or has failed.
  const [kept, setKept] = useState<ApiMonitorTestRunResponse>();
  if (latest && latest !== kept) setKept(latest);
  const result = latest ?? kept;
  const open = Boolean(latest) || running === true || Boolean(error && kept);
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (next) return;
        setKept(undefined);
        onClose();
      }}
    >
      <DialogContent className="max-w-[520px] gap-3.5 rounded-2xl p-5">
        <DialogHeader className="flex-row items-center justify-between pr-8">
          <DialogTitle className="text-[15px] font-medium">Test result</DialogTitle>
          <StatusBadge
            status={result?.passed ? "up" : "down"}
            shape={result?.passed ? undefined : "square"}
          >
            {result?.passed ? "Passed" : "Failed"}
          </StatusBadge>
        </DialogHeader>
        <div className="grid grid-cols-2 gap-2">
          <div className="rounded-[10px] bg-[#142a23] px-4 py-3.5 text-white">
            <span className={`${caption} text-[#80ffaa]`}>Expected</span>
            <b className="mt-1 block text-[22px] font-medium tracking-[-0.02em]">
              {result?.expected?.title ?? "-"}
            </b>
            <small className="text-xs opacity-75">{result?.expected?.detail}</small>
          </div>
          <div className="rounded-[10px] border border-line-soft bg-card px-4 py-3.5">
            <span className={`${caption} text-faint`}>Actual</span>
            <b
              className={`mt-1 block text-[22px] font-medium tracking-[-0.02em] ${
                result?.passed ? "" : "text-[#be123c]"
              }`}
            >
              {result?.actual?.title ?? "-"}
            </b>
            <small className="text-xs opacity-75">
              {result?.actual?.detail ??
                (typeof result?.duration_ms === "number" ? `${result.duration_ms} ms` : "")}
            </small>
          </div>
        </div>
        {result?.error && (
          <pre className="m-0 overflow-x-auto rounded-lg bg-[#142a23] px-3 py-3 font-mono text-[12.5px] text-[#e6f1ec]">
            {result.error}
          </pre>
        )}
        <p className="text-[13px] leading-[1.5] text-quiet">
          {(result?.notes ?? ["Test runs do not create check history or alerts."]).join(" ")}
        </p>
        {error && <Callout tone="warn">{error}</Callout>}
        {error && <Callout tone="warn">{error}</Callout>}
        <DialogFooter>
          <Button variant="secondary" onClick={onClose}>
            Close
          </Button>
          <Button disabled={running} onClick={onRunAgain}>
            Run again
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
