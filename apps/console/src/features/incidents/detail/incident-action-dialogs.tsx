import { type FormEvent, useEffect, useState } from "react";
import {
  Button,
  Combobox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Select,
  Textarea,
} from "@/design-system";
import { useGetIncidents } from "@/orion-sdk";
import type { LifecycleAction } from "../components/incident-detail-utils";
import { incidentLabel } from "../incident-format";

const copy: Record<LifecycleAction, { title: string; body: string; placeholder: string }> = {
  acknowledge: {
    title: "Acknowledge incident",
    body: "Tell the team you are on it. Alerts continue until it resolves.",
    placeholder: "Checking the load balancer.",
  },
  resolve: {
    title: "Resolve incident",
    body: "Closes the timeline. If checks fail again, Orion opens a new incident.",
    placeholder: "What fixed it?",
  },
  reopen: {
    title: "Reopen incident",
    body: "Restores the incident and resumes alerts.",
    placeholder: "Why is it back?",
  },
};

const dialogClass = "max-w-[340px] gap-3 rounded-2xl p-5";
const labelClass = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";

export const ActionNoteDialog = ({
  action,
  pending,
  onOpenChange,
  onSubmit,
}: {
  action: LifecycleAction | null;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: { note?: string }) => void;
}) => {
  const [note, setNote] = useState("");
  useEffect(() => {
    if (action === null) setNote("");
  }, [action]);
  const text = action ? copy[action] : copy.acknowledge;
  const submit = (event: FormEvent) => {
    event.preventDefault();
    onSubmit({ note: note.trim() || undefined });
  };
  return (
    <Dialog open={action !== null} onOpenChange={onOpenChange}>
      <DialogContent className={dialogClass}>
        <form onSubmit={submit} className="flex min-w-0 flex-col gap-3">
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">{text.title}</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] leading-[1.45] text-mid">{text.body}</p>
          <label className={labelClass}>
            Note, optional
            <Textarea
              className="font-normal"
              placeholder={text.placeholder}
              value={note}
              onChange={(event) => setNote(event.target.value)}
            />
          </label>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {action ? action.charAt(0).toUpperCase() + action.slice(1) : ""}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};

const untilOptions = [
  { value: "1", label: "In 1 hour" },
  { value: "4", label: "In 4 hours" },
  { value: "24", label: "In 24 hours" },
];

export const CoverIncidentDialog = ({
  open,
  incidentId,
  pending,
  onOpenChange,
  onSubmit,
}: {
  open: boolean;
  incidentId: string;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: { covered_until: string; covered_by?: string; note?: string }) => void;
}) => {
  const [hours, setHours] = useState("4");
  const [coveredBy, setCoveredBy] = useState("");
  const [note, setNote] = useState("");
  const active = useGetIncidents(
    { status: "open,acknowledged,covered", limit: 50 },
    { query: { enabled: open } },
  );
  const others = (active.data?.incidents ?? []).filter((incident) => incident.id !== incidentId);
  const options = others.map((incident) => ({
    value: incident.id ?? "",
    label: `${incidentLabel(incident)} ${incident.title ?? ""}`.trim(),
  }));
  const submit = (event: FormEvent) => {
    event.preventDefault();
    onSubmit({
      covered_until: new Date(Date.now() + Number(hours) * 3_600_000).toISOString(),
      covered_by: coveredBy || undefined,
      note: note.trim() || undefined,
    });
  };
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={dialogClass}>
        <form onSubmit={submit} className="flex min-w-0 flex-col gap-3">
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Cover incident</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] leading-[1.45] text-mid">
            Pause repeat alerts while you work on it. It stays open.
          </p>
          <div className={labelClass}>
            Cover until
            <Select
              aria-label="Cover until"
              className="w-full font-normal"
              options={untilOptions}
              value={hours}
              onValueChange={setHours}
            />
          </div>
          <div className={labelClass}>
            Covered by, optional
            <Combobox
              aria-label="Covered by"
              placeholder="Search incidents"
              options={options}
              value={coveredBy || null}
              onValueChange={(next) => setCoveredBy(next ?? "")}
            />
          </div>
          <label className={labelClass}>
            Note, optional
            <Textarea
              className="font-normal"
              placeholder="Why is it covered?"
              value={note}
              onChange={(event) => setNote(event.target.value)}
            />
          </label>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              Cover
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
