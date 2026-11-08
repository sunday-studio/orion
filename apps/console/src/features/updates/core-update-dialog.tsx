import {
  Button,
  CodeBlock,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  useToast,
} from "@/design-system";
import { formatDate } from "@/utils/date";
import { coreUpdateCommand } from "./update-model";
import { type ApiUpdateSettings, useChangeUpdateSettings } from "./use-updates";

const BACKUP_GUIDE =
  "https://github.com/sunday-studio/orion/blob/main/docs/deployment/core-docker.md#backups";

/** How to move Core to the newest release: notes, the Compose command, skip or remind later. */
export const CoreUpdateDialog = ({
  open,
  onOpenChange,
  updates,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  updates: ApiUpdateSettings;
}) => {
  const toast = useToast();
  const change = useChangeUpdateSettings();
  const latest = updates.latest_version ?? "";
  const command = coreUpdateCommand(latest);
  const security = updates.state === "security";
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[540px]">
        <DialogHeader>
          <DialogTitle>Update Core to {latest}</DialogTitle>
          <DialogDescription>
            You are on {updates.current_version}.
            {updates.released_at && ` Released ${formatDate(updates.released_at, "d MMM")}.`}{" "}
            {updates.release_url && (
              <a href={updates.release_url} target="_blank" rel="noreferrer" className="underline">
                Release notes
              </a>
            )}
          </DialogDescription>
        </DialogHeader>
        {security && (
          <p className="text-[13px] text-[var(--tone-down-fg)]">
            This release fixes a security issue, so it cannot be skipped.
          </p>
        )}
        {(updates.highlights?.length ?? 0) > 0 && (
          <ul className="grid gap-1 pl-4 text-[13px] [list-style:disc]">
            {updates.highlights?.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        )}
        <div className="grid gap-1.5">
          <span className="text-[12.5px] font-medium">Docker Compose</span>
          <CodeBlock className="whitespace-pre-wrap break-all">{command}</CodeBlock>
        </div>
        <p className="text-[12.5px] text-quiet">
          Back up <code className="font-mono">orion.db</code> first.{" "}
          <a href={BACKUP_GUIDE} target="_blank" rel="noreferrer" className="underline">
            Backup guide
          </a>
        </p>
        <DialogFooter>
          {!security && (
            <Button
              variant="secondary"
              disabled={change.isPending}
              onClick={() =>
                change.mutate(
                  { data: { skipped_version: latest } },
                  {
                    onSuccess: () => {
                      toast.add({ title: `Skipped ${latest}` });
                      onOpenChange(false);
                    },
                  },
                )
              }
            >
              Skip this version
            </Button>
          )}
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Remind me later
          </Button>
          <Button
            onClick={() => {
              void navigator.clipboard.writeText(command);
              toast.add({ title: "Command copied" });
            }}
          >
            Copy command
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
