import { X } from "lucide-react";
import { type KeyboardEvent, useEffect, useState } from "react";
import {
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Textarea,
} from "@/design-system";
import type { ServerRow } from "@/features/agents/server-format";
import { SERVER_EDIT_API_AVAILABLE } from "../server-api-flags";

type Props = {
  open: boolean;
  agent: ServerRow & { labels?: string[] | null; notes?: string | null };
  onOpenChange: (open: boolean) => void;
  onRemove: () => void;
};

const LABEL = /^[A-Za-z0-9_.-]+=[^\s=]+$/;

export const ServerEditDialog = ({ open, agent, onOpenChange, onRemove }: Props) => {
  const [name, setName] = useState("");
  const [labels, setLabels] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [notes, setNotes] = useState("");
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    if (!open) return;
    setName(agent.name ?? "");
    setLabels(agent.labels ?? []);
    setNotes(agent.notes ?? "");
    setDraft("");
    setUnavailable(false);
    // Seed once per open: a background refetch must not overwrite what is being typed.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const addLabel = () => {
    const value = draft.trim();
    if (!LABEL.test(value) || labels.includes(value)) return;
    setLabels([...labels, value]);
    setDraft("");
  };
  const onKey = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      addLabel();
    } else if (event.key === "Backspace" && draft === "" && labels.length > 0) {
      setLabels(labels.slice(0, -1));
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[480px] gap-3.5 rounded-2xl p-5">
        <form
          className="flex min-w-0 flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!SERVER_EDIT_API_AVAILABLE) setUnavailable(true);
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Edit {agent.name}</DialogTitle>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Name
            <Input
              className="font-normal"
              value={name}
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <div className="flex flex-col gap-1.5 text-[13px] font-medium">
            <label htmlFor="server-labels">Labels</label>
            <div className="flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border border-input-border bg-card px-2 py-1">
              {labels.map((label) => (
                <span
                  key={label}
                  className="inline-flex items-center gap-1.5 rounded-md bg-tray px-2 py-0.5 text-xs font-medium"
                >
                  {label}
                  <button
                    type="button"
                    aria-label={`Remove ${label}`}
                    className="text-faint hover:text-ink"
                    onClick={() => setLabels(labels.filter((item) => item !== label))}
                  >
                    <X className="size-3" />
                  </button>
                </span>
              ))}
              <input
                id="server-labels"
                className="min-w-[90px] flex-1 bg-transparent text-[13px] font-normal outline-none placeholder:text-faint"
                placeholder="key=value"
                value={draft}
                onChange={(event) => setDraft(event.target.value)}
                onKeyDown={onKey}
                onBlur={addLabel}
              />
            </div>
            <span className="text-xs font-normal text-quiet">
              Alert routes can match on labels.
            </span>
          </div>
          <label className="flex flex-col gap-1.5 text-[13px] font-medium">
            Notes
            <Textarea
              className="font-normal"
              placeholder="Where it lives, who owns it"
              value={notes}
              onChange={(event) => setNotes(event.target.value)}
            />
          </label>
          {unavailable && (
            <Callout tone="warn">
              Editing a server needs a Core update that is not available yet.
            </Callout>
          )}
          <DialogFooter className="sm:justify-between">
            <Button type="button" variant="destructive" onClick={onRemove}>
              Remove server
            </Button>
            <div className="flex gap-2">
              <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!name.trim()}>
                Save
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
