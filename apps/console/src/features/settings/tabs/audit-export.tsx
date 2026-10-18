import { useState } from "react";
import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Segmented,
} from "@/design-system";
import { downloadExport, useCreateExport, useGetExport } from "@/orion-sdk";

/** Exports the audit log with the current actor filter. */
export const AuditExport = ({
  open,
  onOpenChange,
  actor,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  actor: string;
}) => {
  const [format, setFormat] = useState("csv");
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
      refetchInterval: (state) =>
        ["done", "failed"].includes(state.state.data?.export?.status ?? "") ? false : 1500,
    },
  });
  const done = job.data?.export?.status === "done";
  const close = (next: boolean) => {
    onOpenChange(next);
    if (!next) {
      setJobId("");
      setError(undefined);
    }
  };
  const download = async () => {
    const data = await downloadExport(jobId);
    const link = document.createElement("a");
    link.href = URL.createObjectURL(new Blob([data as BlobPart]));
    link.download = `orion-audit.${format === "csv" ? "csv" : "jsonl"}`;
    link.click();
    URL.revokeObjectURL(link.href);
  };
  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent className="max-w-[420px] rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">
            {jobId ? "Export ready" : "Export audit log"}
          </DialogTitle>
        </DialogHeader>
        {!jobId ? (
          <>
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
            <Callout tone="warn">Current filter: {actor ? `actor:${actor}` : "everyone"}.</Callout>
            {error && <Callout tone="down">{error}</Callout>}
          </>
        ) : (
          <Callout tone={done ? "up" : "warn"}>
            {done
              ? `${(job.data?.export?.rows ?? 0).toLocaleString()} entries exported.`
              : "Preparing your export..."}
          </Callout>
        )}
        <DialogFooter>
          <Button variant="secondary" onClick={() => close(false)}>
            {jobId ? "Close" : "Cancel"}
          </Button>
          {jobId ? (
            <Button disabled={!done} onClick={() => void download()}>
              Download
            </Button>
          ) : (
            <Button
              disabled={create.isPending}
              onClick={() =>
                create.mutate({
                  data: {
                    kind: "audit",
                    format: format as "csv" | "jsonl",
                    query: actor ? `actor=${encodeURIComponent(actor)}` : "",
                  },
                })
              }
            >
              Export
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
