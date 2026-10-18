import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/design-system";

export const DeleteMonitorDialog = ({
  open,
  name,
  pending,
  error,
  onOpenChange,
  onConfirm,
}: {
  open: boolean;
  name: string;
  pending: boolean;
  error: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-w-[400px] gap-3 rounded-2xl p-5">
      <DialogHeader>
        <DialogTitle className="text-[15px] font-medium">Delete monitor</DialogTitle>
      </DialogHeader>
      <p className="text-[13px] leading-[1.5] text-mid">
        {name} and its 90 days of check history will be deleted. Open incidents stay.
      </p>
      {error && <Callout tone="warn">Unable to delete the monitor. Try again.</Callout>}
      <DialogFooter>
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant="destructive" disabled={pending} onClick={onConfirm}>
          Delete monitor
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);
