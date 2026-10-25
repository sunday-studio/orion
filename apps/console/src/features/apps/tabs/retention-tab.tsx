import { useState } from "react";
import { Button, ConfirmDialog, FieldPanel, Input, Select, useToast } from "@/design-system";
import { type ApiApp, usePurgeAppLogs, useUpdateApp } from "@/orion-sdk";
import { Labeled } from "@/features/settings/components-ui";
import { gigabytes, storageUnit } from "../app-format";

const KEEP = [7, 14, 30, 90].map((days) => ({ value: String(days), label: `${days} days` }));
const DEBUG = [
  { value: "1", label: "100% (all)" },
  { value: "0.1", label: "10% (sampled)" },
  { value: "0.01", label: "1% (sampled)" },
  { value: "0", label: "None" },
];

const normalizeRate = (rate?: number) => (rate === undefined ? 1 : rate > 1 ? rate / 100 : rate);

export const RetentionTab = ({ app, onSaved }: { app: ApiApp; onSaved: () => void }) => {
  const toast = useToast();
  const [keep, setKeep] = useState(String(app.retention_days ?? 14));
  const [cap, setCap] = useState(
    app.max_storage_mb ? String(Math.round((app.max_storage_mb / 1024) * 10) / 10) : "",
  );
  const [rate, setRate] = useState(String(app.rate_limit_per_second ?? 0));
  const [debug, setDebug] = useState(String(normalizeRate(app.debug_sample_rate)));
  const [purging, setPurging] = useState(false);
  const id = app.id ?? "";
  const save = useUpdateApp({
    mutation: {
      onSuccess: () => {
        onSaved();
        toast.add({ title: "Changes saved" });
      },
      onError: (failure) => toast.add({ title: (failure as Error).message || "Unable to save" }),
    },
  });
  const purge = usePurgeAppLogs({
    mutation: {
      onSuccess: (result) => {
        onSaved();
        toast.add({ title: `Deleted ${(result.deleted ?? 0).toLocaleString()} logs` });
      },
      onError: () => toast.add({ title: "Unable to delete logs" }),
    },
  });
  const initialCap = app.max_storage_mb
    ? String(Math.round((app.max_storage_mb / 1024) * 10) / 10)
    : "";
  const dirty =
    keep !== String(app.retention_days ?? 14) ||
    cap !== initialCap ||
    rate !== String(app.rate_limit_per_second ?? 0) ||
    debug !== String(normalizeRate(app.debug_sample_rate));
  const percent = Math.min(Math.round(app.storage_percent ?? 0), 100);
  const capNumber = Number(cap);

  return (
    <div className="flex flex-col gap-3.5">
      <FieldPanel title="Retention" aside="How long logs stay searchable">
        <div className="grid grid-cols-2 gap-4">
          <Labeled label="Keep logs for">
            <Select
              aria-label="Keep logs for"
              className="w-full"
              options={KEEP}
              value={keep}
              onValueChange={setKeep}
            />
          </Labeled>
          <Labeled label="Size cap for this app">
            <Input
              inputMode="decimal"
              placeholder="No cap"
              value={cap ? `${cap}` : ""}
              onChange={(event) => setCap(event.target.value.replace(/[^0-9.]/g, ""))}
            />
            <small className="text-xs text-quiet">In GB. Leave empty for no cap.</small>
          </Labeled>
        </div>
        <div className="mt-3 rounded-[10px] border border-hairline bg-card px-4 py-3 shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <div className="flex justify-between text-[13px]">
            <span>
              Using {gigabytes(app.storage_bytes)}
              {app.storage_cap_bytes
                ? ` of ${gigabytes(app.storage_cap_bytes)} ${storageUnit(app.storage_cap_bytes)}`
                : ` ${storageUnit(app.storage_bytes)}`}
            </span>
            {app.storage_cap_bytes ? <span>{percent}%</span> : null}
          </div>
          <div className="mt-2 h-2 overflow-hidden rounded-full bg-sub">
            <div
              className="h-full rounded-full bg-[#2a9f69] transition-[width]"
              style={{ width: `${app.storage_cap_bytes ? percent : 0}%` }}
            />
          </div>
        </div>
      </FieldPanel>
      <FieldPanel title="Limits" aside="Protect Core from a noisy app">
        <div className="grid grid-cols-2 gap-4">
          <Labeled
            label="Rate limit, lines per second"
            hint="Over the limit, Core answers 429 and the app should retry later."
          >
            <Input
              inputMode="numeric"
              value={rate}
              onChange={(event) => setRate(event.target.value.replace(/\D/g, ""))}
            />
          </Labeled>
          <Labeled label="Keep debug lines">
            <Select
              aria-label="Keep debug lines"
              className="w-full"
              options={DEBUG}
              value={debug}
              onValueChange={setDebug}
            />
          </Labeled>
        </div>
      </FieldPanel>
      <div className="flex justify-end gap-2">
        <Button
          disabled={save.isPending || !dirty}
          onClick={() =>
            save.mutate({
              id,
              data: {
                retention_days: Number(keep),
                max_storage_mb: capNumber > 0 ? Math.round(capNumber * 1024) : 0,
                rate_limit_per_second: Number(rate) || 0,
                debug_sample_rate: Number(debug),
              },
            })
          }
        >
          Save
        </Button>
        <Button variant="destructive" onClick={() => setPurging(true)}>
          Delete all logs for this app
        </Button>
      </div>
      <ConfirmDialog
        open={purging}
        onOpenChange={setPurging}
        title="Delete all logs"
        description={`Every stored log line for ${app.name} is deleted. This cannot be undone.`}
        confirmLabel="Delete all logs"
        onConfirm={() => purge.mutate({ id })}
      />
    </div>
  );
};
