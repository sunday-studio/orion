import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ApiAlertEmailDestinationResponse } from "@/orion-sdk";

type DeleteEmailDestinationDialogProps = {
  destination: ApiAlertEmailDestinationResponse | null;
  isPending: boolean;
  mutationError: string;
  onClose: () => void;
  onDelete: () => void;
};

export const DeleteEmailDestinationDialog = ({
  destination,
  isPending,
  mutationError,
  onClose,
  onDelete,
}: DeleteEmailDestinationDialogProps) => (
  <Dialog open={Boolean(destination)} onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Delete email destination</DialogTitle>
        <DialogDescription>
          Delete {destination?.name ?? "this email destination"} from future alert deliveries.
          Existing notification history stays in the log.
        </DialogDescription>
      </DialogHeader>
      {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button variant="destructive" disabled={!destination?.id || isPending} onClick={onDelete}>
          {isPending ? "Deleting..." : "Delete email destination"}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);
