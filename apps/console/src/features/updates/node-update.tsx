import { useState } from "react";
import {
  Button,
  Callout,
  CodeBlock,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  useToast,
} from "@/design-system";
import { nodeCanUpdate } from "./update-model";
import { useUpdates } from "./use-updates";

const COMMAND = "orion-node update";

/** How to move one Node to the newest release. The Node updates itself, so the command runs on it. */
const NodeUpdateDialog = ({
  open,
  onOpenChange,
  name,
  from,
  to,
  releaseUrl,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  name: string;
  from?: string;
  to?: string;
  releaseUrl?: string;
}) => {
  const toast = useToast();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Update the Node on {name}</DialogTitle>
          <DialogDescription>
            {from} to {to}. Run this on {name}.
          </DialogDescription>
        </DialogHeader>
        <CodeBlock>{COMMAND}</CodeBlock>
        <div className="flex items-center justify-between gap-3 text-[12.5px]">
          <span className="text-quiet">It rolls back by itself if {to} is not healthy.</span>
          {releaseUrl && (
            <a href={releaseUrl} target="_blank" rel="noreferrer" className="underline">
              Release notes
            </a>
          )}
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Close
          </Button>
          <Button
            onClick={() => {
              void navigator.clipboard.writeText(COMMAND);
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

/** A notice on the server page when the Node runs an older release than Core found. */
export const NodeUpdateNotice = ({
  name,
  nodeVersion,
  className,
}: {
  name: string;
  nodeVersion?: string;
  className?: string;
}) => {
  const updates = useUpdates().data;
  const [open, setOpen] = useState(false);
  if (!updates || !nodeCanUpdate(nodeVersion, updates)) return null;
  return (
    <>
      <Callout
        tone="info"
        lead="Node update available"
        className={`flex items-center justify-between gap-3 ${className ?? ""}`}
      >
        <span className="mr-auto">
          {nodeVersion} to {updates.latest_version}
        </span>
        {updates.release_url && (
          <a href={updates.release_url} target="_blank" rel="noreferrer">
            <Button size="sm" variant="secondary">
              Release notes
            </Button>
          </a>
        )}
        <Button size="sm" onClick={() => setOpen(true)}>
          Update Node
        </Button>
      </Callout>
      <NodeUpdateDialog
        open={open}
        onOpenChange={setOpen}
        name={name}
        from={nodeVersion}
        to={updates.latest_version}
        releaseUrl={updates.release_url}
      />
    </>
  );
};
