import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  StatusBadge,
} from "@/design-system";

/** Shape of Core's dry-run test result (POST /v1/monitors/test-run, F4 batch B). */
export type MonitorTestResult = {
  passed: boolean;
  expected?: { title?: string; detail?: string };
  actual?: { title?: string; detail?: string };
  duration_ms?: number;
  error?: string;
  notes?: string[];
};

const caption = "font-mono text-[10.5px] font-medium uppercase tracking-[0.1em]";

export const MonitorTestResultDialog = ({
  result,
  running,
  onRunAgain,
  onClose,
}: {
  result?: MonitorTestResult;
  running?: boolean;
  onRunAgain: () => void;
  onClose: () => void;
}) => (
  <Dialog open={Boolean(result)} onOpenChange={(open) => !open && onClose()}>
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
        {(result?.notes ?? []).join(" ")} Test runs do not create check history or alerts.
      </p>
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
