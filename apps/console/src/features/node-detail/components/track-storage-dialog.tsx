import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
} from "@/design-system";

/** Asks for a directory to track; the node measures its size and reports it. */
export const TrackStorageDialog = ({
  open,
  pending,
  onOpenChange,
  onTrack,
}: {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onTrack: (path: string) => void;
}) => {
  const [path, setPath] = useState("");
  const valid = path.trim().startsWith("/");
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (!next) setPath("");
      }}
    >
      <DialogContent className="max-w-[440px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle className="text-[16px]">Track a directory</DialogTitle>
          <DialogDescription>
            The node measures its size and reports it in about a minute.
          </DialogDescription>
        </DialogHeader>
        <label className="grid gap-1.5">
          <span className="text-[12.5px] font-medium">Directory</span>
          <Input
            autoFocus
            placeholder="/var/lib/postgresql"
            value={path}
            aria-invalid={path !== "" && !valid}
            onChange={(event) => setPath(event.target.value)}
          />
          {path !== "" && !valid && (
            <small className="text-xs text-[var(--tone-down-fg)]">Use an absolute path.</small>
          )}
        </label>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            disabled={!valid || pending}
            onClick={() => {
              onTrack(path.trim());
              setPath("");
              onOpenChange(false);
            }}
          >
            Track
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
