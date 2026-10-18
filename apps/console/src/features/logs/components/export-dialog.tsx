import { useState } from "react";
import {
  AnimatedHeight,
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Segmented,
} from "@/design-system";
import { downloadExport, useCreateExport, useGetExport } from "@/orion-sdk";
import type { LogKind } from "../logs-state";

const size = (bytes?: number) =>
  (bytes ?? 0) >= 1_048_576
    ? `${((bytes ?? 0) / 1_048_576).toFixed(1)} MB`
    : `${Math.max(Math.round((bytes ?? 0) / 1024), 1)} KB`;

/** Two-step export: choose format and scope, then download once the background job is done. */
export const ExportDialog = ({
  open,
  kind,
  query,
  filterSummary,
  onOpenChange,
}: {
  open: boolean;
  kind: LogKind;
  query: string;
  filterSummary: string;
  onOpenChange: (open: boolean) => void;
}) => {
  const [format, setFormat] = useState("csv");
  const [raw, setRaw] = useState(true);
  const [jobId, setJobId] = useState("");
  const [error, setError] = useState<string>();
  const create = useCreateExport({
    mutation: {
      onSuccess: (result) => setJobId(result.export?.id ?? ""),
      onError: (failure) => setError((failure as Error).message || "Unable to start the export."),
    },
  });
  const job = useGetExport(jobId, {
    query: {
      enabled: Boolean(jobId),
      refetchInterval: (state) => {
        const status = state.state.data?.export?.status;
        return status === "done" || status === "failed" ? false : 1500;
      },
    },
  });
  const result = job.data?.export;
  const done = result?.status === "done";
  const failed = result?.status === "failed";
  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setJobId("");
      setError(undefined);
    }
  };
  const download = async () => {
    const data = await downloadExport(jobId);
    const blob = new Blob([data as BlobPart], {
      type: format === "csv" ? "text/csv" : "application/x-ndjson",
    });
    const link = document.createElement("a");
    link.href = URL.createObjectURL(blob);
    link.download = `orion-logs.${format === "csv" ? "csv" : "jsonl"}`;
    link.click();
    URL.revokeObjectURL(link.href);
  };

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-w-[420px] rounded-2xl p-5">
        <AnimatedHeight>
          <div className="grid gap-3.5 pb-1.5">
            <DialogHeader>
              <DialogTitle className="text-[16px]">
                {jobId ? "Export ready" : "Export events"}
              </DialogTitle>
            </DialogHeader>
            {!jobId ? (
              <>
                <div className="grid gap-1.5">
                  <span className="text-[13px] font-medium">Format</span>
                  <Segmented
                    aria-label="Format"
                    surface="card"
                    options={[
                      { value: "csv", label: "CSV" },
                      { value: "jsonl", label: "JSON lines" },
                    ]}
                    value={format}
                    onValueChange={setFormat}
                  />
                </div>
                <div className="grid gap-1.5">
                  <span className="text-[13px] font-medium">Range</span>
                  <Callout tone="warn" className="font-medium">
                    Your current filter: {filterSummary}
                  </Callout>
                </div>
                <label className="flex items-center gap-2 text-[13px]">
                  <Checkbox checked={raw} onCheckedChange={(next) => setRaw(Boolean(next))} />
                  Include the raw event as a column
                </label>
                <p className="text-[13px] leading-[1.5] text-quiet">
                  Exports are capped at 100,000 rows. Narrow the range for larger sets.
                </p>
                {error && <Callout tone="down">{error}</Callout>}
              </>
            ) : failed ? (
              <Callout tone="down">{result?.error || "The export failed. Try again."}</Callout>
            ) : (
              <>
                <Callout tone={done ? "up" : "warn"}>
                  {done ? (
                    <>
                      <b className="font-semibold">{(result?.rows ?? 0).toLocaleString()} events</b>{" "}
                      exported, {size(result?.bytes)}
                      {result?.truncated ? " (capped at 100,000 rows)" : ""}
                    </>
                  ) : (
                    "Preparing your export..."
                  )}
                </Callout>
                <p className="text-[13px] leading-[1.5] text-quiet">
                  The link works for 24 hours. Exports run in the background, so you can close this.
                </p>
              </>
            )}
            <DialogFooter>
              <Button variant="secondary" onClick={() => close(false)}>
                {jobId ? "Close" : "Cancel"}
              </Button>
              {!jobId ? (
                <Button
                  disabled={create.isPending}
                  onClick={() => {
                    setError(undefined);
                    create.mutate({
                      data: {
                        kind: kind === "service" ? "logs" : kind,
                        format: format as "csv" | "jsonl",
                        include_raw: raw,
                        query,
                      },
                    });
                  }}
                >
                  Export
                </Button>
              ) : (
                <Button disabled={!done} onClick={() => void download()}>
                  Download {format === "csv" ? "CSV" : "JSON lines"}
                </Button>
              )}
            </DialogFooter>
          </div>
        </AnimatedHeight>
      </DialogContent>
    </Dialog>
  );
};
