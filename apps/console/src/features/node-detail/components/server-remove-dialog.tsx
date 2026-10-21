import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Dot,
  Input,
  RaisedCard,
  useToast,
} from "@/design-system";
import type { ServerRow } from "@/features/nodes/server-format";
import {
  getGetNodesQueryKey,
  getGetIncidentsQueryKey,
  getGetOverviewQueryKey,
  useDecommissionNode,
  useGetNodeMonitors,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { monitorTypeLabel } from "./monitor-rows";

const Radio = ({
  checked,
  onSelect,
  children,
}: {
  checked: boolean;
  onSelect: () => void;
  children: React.ReactNode;
}) => (
  <button
    type="button"
    role="radio"
    aria-checked={checked}
    onClick={onSelect}
    className="flex items-center gap-2 text-left text-[13px]"
  >
    <span
      className={cn(
        "flex size-3.5 items-center justify-center rounded-full border",
        checked ? "border-[#2563eb]" : "border-input-border",
      )}
    >
      {checked && <span className="size-2 rounded-full bg-[#2563eb]" />}
    </span>
    {children}
  </button>
);

export const ServerRemoveDialog = ({
  open,
  node,
  onOpenChange,
}: {
  open: boolean;
  node: ServerRow;
  onOpenChange: (open: boolean) => void;
}) => {
  const monitors = useGetNodeMonitors(
    node.id ?? "",
    { limit: 100 },
    { query: { enabled: open } },
  );
  const list = monitors.data?.monitors ?? [];
  const total = monitors.data?.count ?? list.length;
  const [keepHistory, setKeepHistory] = useState(true);
  const [typed, setTyped] = useState("");
  const [error, setError] = useState<string>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const decommission = useDecommissionNode();

  useEffect(() => {
    if (!open) return;
    setKeepHistory(true);
    setTyped("");
    setError(undefined);
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5">
        <form
          className="flex min-w-0 flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            setError(undefined);
            decommission.mutate(
              { id: node.id ?? "", params: { keep_history: keepHistory } },
              {
                onSuccess: () => {
                  void queryClient.invalidateQueries({ queryKey: getGetNodesQueryKey() });
                  void queryClient.invalidateQueries({ queryKey: getGetOverviewQueryKey() });
                  void queryClient.invalidateQueries({ queryKey: getGetIncidentsQueryKey() });
                  toast.add({ title: `${node.name} removed` });
                  onOpenChange(false);
                  navigate("/servers", { replace: true });
                },
                onError: (failure) => {
                  const status = (failure as { status?: number }).status;
                  if (status === 404) {
                    // Already removed elsewhere: the goal is met.
                    void queryClient.invalidateQueries({ queryKey: getGetNodesQueryKey() });
                    onOpenChange(false);
                    navigate("/servers", { replace: true });
                    return;
                  }
                  setError(
                    status === 400
                      ? (failure as Error).message || "This server cannot be removed."
                      : "Unable to remove the server. Try again.",
                  );
                },
              },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Remove {node.name}</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] text-mid">The node is revoked and can no longer report.</p>
          <Callout tone="down">
            <b className="mr-1.5 font-semibold">
              {total} {total === 1 ? "monitor" : "monitors"}
            </b>
            on this server will be deleted.
          </Callout>
          {list.length > 0 && (
            <RaisedCard className="max-h-[170px] overflow-auto">
              {list.map((monitor) => (
                <div
                  key={monitor.id}
                  className="grid grid-cols-[12px_1fr_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
                >
                  <Dot color="#a1a1aa" />
                  <b className="truncate font-medium">{monitor.name ?? monitor.id}</b>
                  <span className="text-faint">{monitorTypeLabel(monitor.type)}</span>
                </div>
              ))}
            </RaisedCard>
          )}
          <div role="radiogroup" aria-label="History" className="flex flex-col gap-1.5">
            <span className="text-[13px] font-medium">History</span>
            <Radio checked={keepHistory} onSelect={() => setKeepHistory(true)}>
              Keep reports for 90 days, then delete
            </Radio>
            <Radio checked={!keepHistory} onSelect={() => setKeepHistory(false)}>
              Delete everything now
            </Radio>
          </div>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Type the server name to confirm
            <Input
              className="font-normal"
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              placeholder={node.name}
            />
          </label>
          {error && <Callout tone="warn">{error}</Callout>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={typed !== node.name || decommission.isPending}
            >
              Remove server
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
