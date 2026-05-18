import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import type { ApiAlertChannelResponse } from "@/orion-sdk";

type DeleteWebhookDialogProps = {
  channel: ApiAlertChannelResponse | null;
  isPending: boolean;
  mutationError: string;
  onClose: () => void;
  onDelete: () => void;
};

export const DeleteWebhookDialog = ({
  channel,
  isPending,
  mutationError,
  onClose,
  onDelete,
}: DeleteWebhookDialogProps) => (
  <Dialog open={Boolean(channel)} onOpenChange={(open) => !open && onClose()}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Delete webhook</DialogTitle>
        <DialogDescription>
          Delete {channel?.name ?? "this webhook"} from future alert deliveries. Existing
          notification history stays in the log.
        </DialogDescription>
      </DialogHeader>
      {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
      <DialogFooter>
        <Button variant="ghost" onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button variant="destructive" disabled={!channel?.id || isPending} onClick={onDelete}>
          {isPending ? "Deleting..." : "Delete webhook"}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
);
