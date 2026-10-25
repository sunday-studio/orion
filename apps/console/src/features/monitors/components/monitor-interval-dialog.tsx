import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Select,
} from "@/design-system";
import { FAIL_AFTER, INTERVALS } from "../node-timing";

/** Picks a check frequency for the selected Node monitors. */
export const MonitorIntervalDialog = ({
  open,
  count,
  pending,
  onOpenChange,
  onApply,
}: {
  open: boolean;
  count: number;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onApply: (interval: number, failAfter?: number) => void;
}) => {
  const [interval, setInterval] = useState("60");
  const [failAfter, setFailAfter] = useState("keep");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[440px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">
            Change how often {count} {count === 1 ? "monitor runs" : "monitors run"}
          </DialogTitle>
          <DialogDescription>
            Applies to monitors that run on a server through its Node and are managed by Orion. The
            Node picks it up at its next sync, about a minute.
          </DialogDescription>
        </DialogHeader>
        <label className="grid gap-1.5 text-[12.5px] font-medium">
          Check
          <Select
            aria-label="Check frequency"
            className="w-full"
            options={INTERVALS}
            value={interval}
            onValueChange={setInterval}
          />
        </label>
        <label className="grid gap-1.5 text-[12.5px] font-medium">
          Down after
          <Select
            aria-label="Failed checks before down"
            className="w-full"
            options={[{ value: "keep", label: "Keep as is" }, ...FAIL_AFTER]}
            value={failAfter}
            onValueChange={setFailAfter}
          />
        </label>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={pending}
            onClick={() =>
              onApply(Number(interval), failAfter === "keep" ? undefined : Number(failAfter))
            }
          >
            {pending ? "Applying..." : "Apply"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
