import { useEffect, useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Segmented,
  Select,
} from "@/design-system";

/** Scheduled maintenance windows need Core support (F4 batch B); "Start now" works today. */
export const SCHEDULED_MAINTENANCE_API_AVAILABLE = false;

const DURATIONS = [
  [3600, "1 hour"],
  [14400, "4 hours"],
  [86400, "24 hours"],
  [604800, "7 days"],
] as const;

export const MaintenanceDialog = ({
  open,
  pending,
  onOpenChange,
  onStart,
}: {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onStart: (durationSeconds: number) => void;
}) => {
  const [mode, setMode] = useState("now");
  const [duration, setDuration] = useState("86400");
  const [reason, setReason] = useState("");
  const [pauseMonitors, setPauseMonitors] = useState(true);
  const [notice, setNotice] = useState(false);

  useEffect(() => {
    if (!open) return;
    setMode("now");
    setDuration("86400");
    setReason("");
    setNotice(false);
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5">
        <form
          className="flex min-w-0 flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (mode === "now") onStart(Number(duration));
            else if (!SCHEDULED_MAINTENANCE_API_AVAILABLE) setNotice(true);
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Maintenance window</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] leading-[1.5] text-mid">
            Checks keep running, but alerts and incidents are held. Status pages show Maintenance.
          </p>
          <Segmented
            aria-label="When"
            options={[
              { value: "now", label: "Start now" },
              { value: "schedule", label: "Schedule" },
            ]}
            value={mode}
            onValueChange={setMode}
          />
          {mode === "now" ? (
            <div className="flex flex-col gap-1.5 text-[13px] font-medium">
              Duration
              <Select
                aria-label="Duration"
                className="w-full font-normal"
                options={DURATIONS.map(([value, label]) => ({ value: String(value), label }))}
                value={duration}
                onValueChange={setDuration}
              />
            </div>
          ) : (
            <>
              <div className="grid grid-cols-2 gap-3">
                <label className="flex flex-col gap-1.5 text-[13px] font-medium">
                  Starts
                  <Input className="font-normal" type="datetime-local" />
                </label>
                <label className="flex flex-col gap-1.5 text-[13px] font-medium">
                  Ends
                  <Input className="font-normal" type="datetime-local" />
                </label>
              </div>
              <label className="flex flex-col gap-1.5 text-[13px] font-medium">
                Reason shown on the status page
                <Input
                  className="font-normal"
                  value={reason}
                  placeholder="Kernel upgrade"
                  onChange={(event) => setReason(event.target.value)}
                />
              </label>
              <label className="flex items-center gap-2 text-[13px]">
                <Checkbox
                  checked={pauseMonitors}
                  onCheckedChange={(next) => setPauseMonitors(Boolean(next))}
                />
                Pause monitors on this server
              </label>
            </>
          )}
          {notice && (
            <Callout tone="warn">
              Scheduled windows need a Core update that is not available yet. Use Start now.
            </Callout>
          )}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {mode === "now" ? "Start maintenance" : "Schedule maintenance"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
