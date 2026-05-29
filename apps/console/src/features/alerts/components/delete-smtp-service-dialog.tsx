import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ApiAlertSMTPServiceResponse } from "@/orion-sdk";

type DeleteSMTPServiceDialogProps = {
  isPending: boolean;
  mutationError: string;
  service: ApiAlertSMTPServiceResponse | null;
  onClose: () => void;
  onDelete: () => void;
};

export const DeleteSMTPServiceDialog = ({
  isPending,
  mutationError,
  service,
  onClose,
  onDelete,
}: DeleteSMTPServiceDialogProps) => (
  <Dialog open={Boolean(service)} onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Delete SMTP service</DialogTitle>
        <DialogDescription>
          Delete {service?.name ?? "this SMTP service"} from future email alert deliveries. Existing
          notification history stays in the log.
        </DialogDescription>
      </DialogHeader>
      {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button variant="destructive" disabled={!service?.id || isPending} onClick={onDelete}>
          {isPending ? "Deleting..." : "Delete SMTP service"}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);
