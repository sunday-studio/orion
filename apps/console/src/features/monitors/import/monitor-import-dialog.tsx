import { useQueryClient } from "@tanstack/react-query";
import { useMemo, useRef, useState } from "react";
import {
  AnimatedHeight,
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  RaisedCard,
  StatusBadge,
  Textarea,
  useToast,
} from "@/design-system";
import {
  createCoreMonitor,
  getGetMonitorSummaryQueryKey,
  getGetMonitorsQueryKey,
  useGetMonitors,
} from "@/orion-sdk";
import { EXAMPLE, type ImportRow, parseImport } from "./import-model";

type Outcome = { state: "created" | "failed" | "skipped"; message?: string };

/** Import many website and TCP monitors from JSON, with a preview before anything is created. */
export const MonitorImportDialog = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const file = useRef<HTMLInputElement>(null);
  const existing = useGetMonitors({ limit: 200 }, { query: { enabled: open } });
  const [text, setText] = useState("");
  const [skipExisting, setSkipExisting] = useState(true);
  const [running, setRunning] = useState(false);
  const [outcomes, setOutcomes] = useState<Record<number, Outcome>>({});
  const parsed = useMemo(() => (text.trim() ? parseImport(text) : undefined), [text]);
  const names = useMemo(
    () => new Set((existing.data?.monitors ?? []).map((m) => (m.name ?? "").toLowerCase())),
    [existing.data],
  );
  const rows = parsed?.rows ?? [];
  const isDuplicate = (row: ImportRow) => skipExisting && names.has(row.name.toLowerCase());
  const ready = rows.filter((row) => row.request && !isDuplicate(row));
  const finished = Object.keys(outcomes).length > 0 && !running;

  const reset = () => {
    setText("");
    setOutcomes({});
    setRunning(false);
  };

  const run = async () => {
    setRunning(true);
    const next: Record<number, Outcome> = {};
    for (const row of rows) {
      if (!row.request) continue;
      if (isDuplicate(row)) {
        next[row.index] = { state: "skipped", message: "Already exists" };
        continue;
      }
      try {
        await createCoreMonitor(row.request);
        next[row.index] = { state: "created" };
      } catch (failure) {
        next[row.index] = { state: "failed", message: (failure as Error).message };
      }
      setOutcomes({ ...next });
    }
    setOutcomes(next);
    setRunning(false);
    void queryClient.invalidateQueries({ queryKey: getGetMonitorsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetMonitorSummaryQueryKey() });
    const created = Object.values(next).filter((o) => o.state === "created").length;
    toast.add({ title: `Imported ${created} ${created === 1 ? "monitor" : "monitors"}` });
  };

  const badge = (row: ImportRow) => {
    const outcome = outcomes[row.index];
    if (outcome?.state === "created") return <StatusBadge status="up">Created</StatusBadge>;
    if (outcome?.state === "failed") return <StatusBadge status="down">Failed</StatusBadge>;
    if (outcome?.state === "skipped" || (row.request && isDuplicate(row)))
      return <StatusBadge status="stale">Already exists</StatusBadge>;
    if (row.error) return <StatusBadge status="down">Invalid</StatusBadge>;
    return (
      <StatusBadge status="unknown" shape="none">
        Ready
      </StatusBadge>
    );
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (!next) reset();
      }}
    >
      <DialogContent className="max-w-[680px] gap-3.5 rounded-2xl p-5">
        <AnimatedHeight>
          <div className="grid gap-3.5 pb-1.5">
            <DialogHeader>
              <DialogTitle className="text-[16px]">Import monitors</DialogTitle>
              <DialogDescription>
                Paste JSON or choose a file. Each website or TCP target becomes a monitor.
              </DialogDescription>
            </DialogHeader>
            <Textarea
              aria-label="Monitors JSON"
              rows={rows.length > 0 ? 4 : 9}
              className="font-mono text-[12.5px]"
              placeholder={EXAMPLE}
              value={text}
              disabled={running}
              onChange={(event) => {
                setText(event.target.value);
                setOutcomes({});
              }}
            />
            <div className="flex flex-wrap items-center gap-2">
              <input
                ref={file}
                type="file"
                accept="application/json,.json"
                className="hidden"
                onChange={(event) => {
                  const chosen = event.target.files?.[0];
                  event.target.value = "";
                  if (chosen)
                    void chosen.text().then((content) => {
                      setText(content);
                      setOutcomes({});
                    });
                }}
              />
              <Button size="sm" variant="secondary" onClick={() => file.current?.click()}>
                Choose file
              </Button>
              <Button
                size="sm"
                variant="secondary"
                onClick={() => {
                  setText(EXAMPLE);
                  setOutcomes({});
                }}
              >
                Use the example
              </Button>
              <span className="text-xs text-quiet">
                A list of URLs also works: ["https://a.com", "https://b.com"]
              </span>
            </div>
            {parsed?.fatal && <Callout tone="down">{parsed.fatal}</Callout>}
            {rows.length > 0 && (
              <>
                <RaisedCard className="max-h-[260px] overflow-y-auto">
                  {rows.map((row) => (
                    <div
                      key={row.index}
                      className="grid min-h-[40px] grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)_130px] items-center gap-3 border-t border-line-soft px-3.5 py-1.5 text-[13px] first:border-t-0"
                    >
                      <b className="truncate font-medium">{row.name || `Row ${row.index + 1}`}</b>
                      <span
                        className="truncate text-quiet"
                        title={outcomes[row.index]?.message ?? row.error}
                      >
                        {outcomes[row.index]?.message ??
                          row.error ??
                          `${row.kind.toUpperCase()} ${row.target}, every ${row.intervalSeconds}s`}
                      </span>
                      <span className="flex justify-end">{badge(row)}</span>
                    </div>
                  ))}
                </RaisedCard>
                <label className="flex items-center gap-2 text-[13px]">
                  <Checkbox
                    checked={skipExisting}
                    disabled={running}
                    onCheckedChange={(next) => setSkipExisting(Boolean(next))}
                  />
                  Skip monitors that already exist (same name)
                </label>
              </>
            )}
            <DialogFooter>
              <Button variant="secondary" onClick={() => onOpenChange(false)}>
                {finished ? "Close" : "Cancel"}
              </Button>
              {!finished && (
                <Button disabled={ready.length === 0 || running} onClick={() => void run()}>
                  {running
                    ? "Importing..."
                    : `Import ${ready.length} ${ready.length === 1 ? "monitor" : "monitors"}`}
                </Button>
              )}
            </DialogFooter>
          </div>
        </AnimatedHeight>
      </DialogContent>
    </Dialog>
  );
};
