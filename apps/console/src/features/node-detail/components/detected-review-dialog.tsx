import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  RaisedCard,
  Select,
} from "@/design-system";
import type { ServiceNodeDiscoveryItem } from "@/orion-sdk";
import { FAIL_AFTER, INTERVALS } from "@/features/monitors/node-timing";
import { acceptWhat } from "../detected-model";

/** Review before accepting: exactly what will be created. */
export const DetectedReviewDialog = ({
  open,
  items,
  skippedLogs,
  pending,
  interval,
  failAfter,
  onInterval,
  onFailAfter,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  items: ServiceNodeDiscoveryItem[];
  skippedLogs: ServiceNodeDiscoveryItem[];
  pending: boolean;
  interval: string;
  failAfter: string;
  onInterval: (value: string) => void;
  onFailAfter: (value: string) => void;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) => {
  const storage = items.filter((item) => item.kind === "storage");
  const monitors = items.filter((item) => item.kind !== "storage");
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[640px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">
            {monitors.length > 0
              ? `Add ${monitors.length} ${monitors.length === 1 ? "monitor" : "monitors"}`
              : "Track storage"}
            {monitors.length > 0 && storage.length > 0
              ? ` and track ${storage.length} ${storage.length === 1 ? "directory" : "directories"}`
              : ""}
            ?
          </DialogTitle>
          <DialogDescription>
            Orion creates the monitors now. The Node picks them up in about a minute.
            {storage.length > 0 && " Directories are only tracked for size, with no alerts."}
          </DialogDescription>
        </DialogHeader>
        {monitors.length > 0 && (
          <div className="grid grid-cols-2 gap-3">
            <label className="grid gap-1.5 text-[12.5px] font-medium">
              Check
              <Select
                aria-label="Check frequency"
                className="w-full"
                options={INTERVALS}
                value={interval}
                onValueChange={onInterval}
              />
            </label>
            <label className="grid gap-1.5 text-[12.5px] font-medium">
              Down after
              <Select
                aria-label="Failed checks before down"
                className="w-full"
                options={FAIL_AFTER}
                value={failAfter}
                onValueChange={onFailAfter}
              />
            </label>
          </div>
        )}
        <RaisedCard className="max-h-[320px] overflow-y-auto">
          {items.map((item) => {
            const words = acceptWhat(item, failAfter);
            return (
              <div
                key={item.id}
                className="grid min-h-[42px] grid-cols-[170px_minmax(0,1fr)_150px] items-center gap-3 border-t border-line-soft px-3.5 py-2 text-[13px] first:border-t-0"
              >
                <b className="truncate font-medium">{item.name}</b>
                <span className="text-mid">{words.what}</span>
                <span className="text-quiet">{words.when}</span>
              </div>
            );
          })}
        </RaisedCard>
        {skippedLogs.length > 0 && (
          <p className="text-[13px] text-quiet">
            Skipped: {skippedLogs.map((item) => item.name).join(", ")} needs a log key.
          </p>
        )}
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button disabled={pending || items.length === 0} onClick={onConfirm}>
            {pending
              ? "Adding..."
              : monitors.length > 0
                ? `Add ${monitors.length} ${monitors.length === 1 ? "monitor" : "monitors"}`
                : "Track storage"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
