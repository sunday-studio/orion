import { useEffect, useState } from "react";
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
} from "@/design-system";
import type { ServerRow } from "@/features/agents/server-format";
import { useGetAgentMonitors } from "@/orion-sdk";
import { cn } from "@/utils/cn";
import { SERVER_REMOVE_API_AVAILABLE } from "../server-api-flags";
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
  agent,
  onOpenChange,
}: {
  open: boolean;
  agent: ServerRow;
  onOpenChange: (open: boolean) => void;
}) => {
  const monitors = useGetAgentMonitors(
    agent.id ?? "",
    { limit: 100 },
    { query: { enabled: open } },
  );
  const list = monitors.data?.monitors ?? [];
  const total = monitors.data?.count ?? list.length;
  const [keepHistory, setKeepHistory] = useState(true);
  const [typed, setTyped] = useState("");
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    if (!open) return;
    setKeepHistory(true);
    setTyped("");
    setUnavailable(false);
  }, [open]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5">
        <form
          className="flex min-w-0 flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!SERVER_REMOVE_API_AVAILABLE) setUnavailable(true);
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Remove {agent.name}</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] text-mid">The agent is revoked and can no longer report.</p>
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
              placeholder={agent.name}
            />
          </label>
          {unavailable && (
            <Callout tone="warn">
              Removing a server needs a Core update that is not available yet.
            </Callout>
          )}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="destructive" disabled={typed !== agent.name}>
              Remove server
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
