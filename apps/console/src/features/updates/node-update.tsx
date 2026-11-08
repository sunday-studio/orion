import { useQueryClient } from "@tanstack/react-query";
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
import {
  getListNodeDiscoveryRequestsQueryKey,
  useGetNodeDiscovery,
  useListNodeDiscoveryRequests,
  useRequestNodeUpdate,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { nodeCanUpdate, releaseNotesUrl } from "./update-model";
import { useUpdates } from "./use-updates";

const COMMAND = "orion-node update";
const OPEN = new Set(["queued", "delivered"]);

type Choice = "here" | "server";

const Option = ({
  selected,
  disabled,
  onSelect,
  title,
  children,
}: {
  selected: boolean;
  disabled?: boolean;
  onSelect: () => void;
  title: string;
  children: React.ReactNode;
}) => (
  <button
    type="button"
    role="radio"
    aria-checked={selected}
    disabled={disabled}
    onClick={onSelect}
    className={cn(
      "grid grid-cols-[20px_minmax(0,1fr)] gap-2.5 rounded-lg border p-3 text-left outline-none focus-visible:ring-[3px] focus-visible:ring-focus disabled:cursor-not-allowed disabled:opacity-60",
      selected ? "border-brand bg-(--tone-up-bg)" : "border-hairline bg-card",
    )}
  >
    <span
      aria-hidden="true"
      className={cn(
        "mt-px block size-4 rounded-full border-[1.5px]",
        selected
          ? "border-brand bg-[radial-gradient(circle,var(--color-brand)_0_4px,transparent_5px)]"
          : "border-faint",
      )}
    />
    <span className="grid gap-0.5">
      <b className="text-[13px] font-medium">{title}</b>
      <span className="text-[12.5px] leading-normal text-quiet">{children}</span>
    </span>
  </button>
);

/** Pick how to update one Node: from the Console, or by running a command on it. */
const NodeUpdateDialog = ({
  open,
  onOpenChange,
  nodeId,
  name,
  from,
  to,
  canUpdateHere,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  nodeId: string;
  name: string;
  from?: string;
  to: string;
  canUpdateHere: boolean;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [choice, setChoice] = useState<Choice>(canUpdateHere ? "here" : "server");
  const request = useRequestNodeUpdate({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({
          queryKey: getListNodeDiscoveryRequestsQueryKey(nodeId),
        });
        toast.add({ title: `Updating ${name} to ${to}` });
        onOpenChange(false);
      },
      onError: (failure) =>
        toast.add({ title: (failure as Error).message || "Unable to start the update" }),
    },
  });
  const here = choice === "here" && canUpdateHere;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[540px]">
        <DialogHeader>
          <DialogTitle>Update the Node on {name}</DialogTitle>
          <DialogDescription>
            {from} to {to}. Pick how to update it.
          </DialogDescription>
        </DialogHeader>
        <div role="radiogroup" aria-label="How to update" className="grid gap-2">
          <Option
            selected={here}
            disabled={!canUpdateHere}
            onSelect={() => setChoice("here")}
            title="Update from here"
          >
            {canUpdateHere
              ? `Core tells the Node, which downloads ${to} and restarts itself. It rolls back if ${to} does not start.`
              : "This Node cannot update itself. It may be in a container, run a build that is not a release, keep its binary where it cannot write, or have remote_update turned off."}
          </Option>
          <Option
            selected={!here}
            onSelect={() => setChoice("server")}
            title="Run it on the server"
          >
            {here ? (
              "Run the command on the server yourself."
            ) : (
              <CodeBlock className="mt-1.5 text-[12px]">{COMMAND}</CodeBlock>
            )}
          </Option>
        </div>
        <div className="text-right text-[12.5px]">
          <a href={releaseNotesUrl(to)} target="_blank" rel="noreferrer" className="underline">
            Release notes
          </a>
        </div>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          {here ? (
            <Button disabled={request.isPending} onClick={() => request.mutate({ nodeId })}>
              Update Node
            </Button>
          ) : (
            <Button
              onClick={() => {
                void navigator.clipboard.writeText(COMMAND);
                toast.add({ title: "Command copied" });
              }}
            >
              Copy command
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

/** A notice on the server page when the Node runs an older release than Core, and while it updates. */
export const NodeUpdateNotice = ({
  nodeId,
  name,
  nodeVersion,
  className,
}: {
  nodeId: string;
  name: string;
  nodeVersion?: string;
  className?: string;
}) => {
  const updates = useUpdates().data;
  const discovery = useGetNodeDiscovery(nodeId).data?.discovery;
  const requests = useListNodeDiscoveryRequests(nodeId, {
    query: {
      refetchInterval: (query) =>
        (query.state.data?.requests ?? []).some(
          (r) => r.type === "update_node" && OPEN.has(r.status ?? ""),
        )
          ? 10_000
          : false,
    },
  }).data?.requests;
  const [open, setOpen] = useState(false);
  if (!updates || !nodeCanUpdate(nodeVersion, updates)) return null;
  const to = updates.current_version ?? "";
  const last = requests?.find((r) => r.type === "update_node");
  const canUpdateHere = Boolean(discovery?.capabilities?.includes("self_update"));
  const style = cn("flex items-center justify-between gap-3", className);

  if (last && OPEN.has(last.status ?? "")) {
    return (
      <Callout tone="info" lead={`Updating to ${to}`} className={style}>
        <span className="mr-auto">
          {last.status === "queued"
            ? "Waiting for the Node to pick it up. It checks about once a minute."
            : "The Node is installing it and will restart."}
        </span>
      </Callout>
    );
  }
  // A failure from days ago is old news; show it only while it is recent.
  const recent =
    last?.finished_at && Date.now() - new Date(last.finished_at).getTime() < 24 * 3600_000;
  const failed =
    recent && (last?.status === "failed" || last?.status === "unsupported") ? last : undefined;
  return (
    <>
      <Callout
        tone={failed ? "warn" : "info"}
        lead={failed ? "Update failed" : "Node update available"}
        className={style}
      >
        <span className="mr-auto">
          {failed ? (failed.result ?? "The Node did not say why.") : `${nodeVersion} to ${to}`}
        </span>
        <a href={releaseNotesUrl(to)} target="_blank" rel="noreferrer">
          <Button size="sm" variant="secondary">
            Release notes
          </Button>
        </a>
        <Button size="sm" onClick={() => setOpen(true)}>
          {failed ? "Try again" : "Update Node"}
        </Button>
      </Callout>
      <NodeUpdateDialog
        open={open}
        onOpenChange={setOpen}
        nodeId={nodeId}
        name={name}
        from={nodeVersion}
        to={to}
        canUpdateHere={canUpdateHere}
      />
    </>
  );
};
