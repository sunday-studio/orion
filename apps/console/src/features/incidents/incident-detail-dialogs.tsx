import { SeverityBadge, StatusBadge, toSeverity, toStatus } from "@/components/status-badges";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import type { ApiMonitorReportResponse, ApiStatusPageIncidentResponse, ApiStatusPageResponse } from "@/orion-sdk";
import { CheckIcon, CircleCheckIcon, MegaphoneIcon, RotateCcwIcon, ShieldCheckIcon } from "lucide-react";
import { type FormEvent, useEffect, useState } from "react";
import { DetailGroup, DetailItem, coverageUntilInputValue, coverageUntilPayload, lifecycleActionLabels, reportTimestamp, type LifecycleAction } from "./incident-detail-utils";

export type CoverIncidentDialogProps = {
  open: boolean;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: { covered_until?: string; note?: string }) => void;
};

export const CoverIncidentDialog = ({
  open,
  pending,
  onOpenChange,
  onSubmit,
}: CoverIncidentDialogProps) => {
  const [coveredUntil, setCoveredUntil] = useState(coverageUntilInputValue);
  const [note, setNote] = useState("");

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit({
      covered_until: coverageUntilPayload(coveredUntil),
      note: note.trim() || undefined,
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>Cover incident</DialogTitle>
          </DialogHeader>
          <label className="block space-y-1">
            <span className="text-sm font-medium">covered until</span>
            <Input
              type="datetime-local"
              value={coveredUntil}
              onChange={(event) => setCoveredUntil(event.target.value)}
            />
          </label>
          <label className="block space-y-1">
            <span className="text-sm font-medium">note</span>
            <Textarea value={note} onChange={(event) => setNote(event.target.value)} />
          </label>
          <DialogFooter showCloseButton>
            <Button type="submit" disabled={pending}>
              <ShieldCheckIcon />
              Cover
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};

export type ActionNoteDialogProps = {
  action: LifecycleAction | null;
  pending: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (payload: { note?: string }) => void;
};

export const ActionNoteDialog = ({ action, pending, onOpenChange, onSubmit }: ActionNoteDialogProps) => {
  const [note, setNote] = useState("");
  const label = action ? lifecycleActionLabels[action] : "";
  const icon =
    action === "acknowledge" ? (
      <CheckIcon />
    ) : action === "resolve" ? (
      <CircleCheckIcon />
    ) : action === "reopen" ? (
      <RotateCcwIcon />
    ) : null;

  useEffect(() => {
    if (action === null) setNote("");
  }, [action]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const formData = new FormData(event.currentTarget);
    const submittedNote = String(formData.get("note") ?? "").trim();
    onSubmit({ note: submittedNote || undefined });
  };

  return (
    <Dialog open={action !== null} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={handleSubmit} className="space-y-4">
          <DialogHeader>
            <DialogTitle>{label} incident</DialogTitle>
          </DialogHeader>
          <label className="block space-y-1">
            <span className="text-sm font-medium">note</span>
            <Textarea name="note" value={note} onChange={(event) => setNote(event.target.value)} />
          </label>
          <DialogFooter showCloseButton>
            <Button type="submit" disabled={pending}>
              {icon}
              {label}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};

export type PublicIncidentDraftDialogProps = {
  open: boolean;
  pages: ApiStatusPageResponse[];
  selectedStatusPageID: string;
  draft?: {
    incident_title?: string;
    impact_summary?: string;
    severity?: string;
    status_page_title?: string;
    suggested_component_ids?: string[];
    suggested_components?: Array<{ component_id?: string; component_name?: string; matches?: Array<{ resource_id?: string; resource_type?: string }> }>;
  };
  createdIncident?: ApiStatusPageIncidentResponse;
  loadingPages: boolean;
  loadingDraft: boolean;
  pending: boolean;
  hasError: boolean;
  onOpenChange: (open: boolean) => void;
  onStatusPageChange: (statusPageID: string) => void;
  onCreate: () => void;
};

export const PublicIncidentDraftDialog = ({
  open,
  pages,
  selectedStatusPageID,
  draft,
  createdIncident,
  loadingPages,
  loadingDraft,
  pending,
  hasError,
  onOpenChange,
  onStatusPageChange,
  onCreate,
}: PublicIncidentDraftDialogProps) => {
  const suggestions = draft?.suggestions ?? [];
  const selectedPage = pages.find((page) => page.id === selectedStatusPageID);
  const canCreate = Boolean(selectedStatusPageID && draft && !createdIncident && !pending);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Create public draft</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <label className="block space-y-1">
            <span className="text-sm font-medium">status page</span>
            <Select value={selectedStatusPageID} onValueChange={onStatusPageChange}>
              <SelectTrigger className="w-full">
                <span className="truncate">
                  {selectedPage?.title ??
                    selectedPage?.slug ??
                    (loadingPages ? "Loading..." : "Select status page")}
                </span>
              </SelectTrigger>
              <SelectContent align="start" position="popper">
                {pages.map((page) => (
                  <SelectItem key={page.id ?? page.slug} value={page.id ?? ""}>
                    {page.title ?? page.slug ?? "Untitled status page"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>

          {pages.length === 0 && !loadingPages && (
            <div className="text-sm text-neutral-600">No status pages available.</div>
          )}

          {selectedPage && (
            <div className="grid gap-3 md:grid-cols-[0.9fr_1.1fr]">
              <div className="space-y-2 bg-neutral-50 px-3 py-3">
                <h3 className="text-sm font-medium">Suggested components</h3>
                {loadingDraft && <div className="text-sm text-neutral-600">Loading draft...</div>}
                {!loadingDraft && suggestions.length === 0 && (
                  <div className="text-sm text-neutral-600">No mapped public components.</div>
                )}
                <div className="space-y-2">
                  {suggestions.map((suggestion) => (
                    <div
                      key={suggestion.component_id}
                      className="border border-neutral-200 bg-white px-3 py-2 text-sm"
                    >
                      <div className="font-medium">{suggestion.component_name}</div>
                      <div className="text-xs text-neutral-500">
                        {(suggestion.matches ?? []).map((match) => match.match_reason).join(", ")}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="space-y-3 bg-neutral-50 px-3 py-3">
                <h3 className="text-sm font-medium">Draft copy</h3>
                {draft ? (
                  <div className="space-y-3 text-sm">
                    <DetailItem label="title" value={draft.title ?? "Untitled incident"} />
                    <DetailItem label="impact" value={draft.impact_summary ?? "—"} />
                    <DetailItem
                      label="initial update"
                      value={draft.initial_update_message ?? "—"}
                    />
                    <div className="flex flex-wrap gap-2">
                      <StatusBadge value={toStatus(draft.public_status)} />
                      <SeverityBadge value={toSeverity(draft.severity)} />
                    </div>
                  </div>
                ) : (
                  <div className="text-sm text-neutral-600">No draft generated.</div>
                )}
              </div>
            </div>
          )}

          {createdIncident && (
            <div className="text-sm text-emerald-700">
              Draft created: {createdIncident.title ?? "Untitled incident"}
            </div>
          )}
          {hasError && <div className="text-sm text-rose-700">Unable to create public draft.</div>}
        </div>
        <DialogFooter showCloseButton>
          <Button type="button" disabled={!canCreate} onClick={onCreate}>
            <MegaphoneIcon />
            {pending ? "Creating..." : "Create draft"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export const EvidenceReportGroup = ({
  title,
  report,
  reason,
  onInspect,
}: {
  title: string;
  report?: ApiMonitorReportResponse;
  reason: string;
  onInspect: () => void;
}) => (
  <DetailGroup title={title}>
    <DetailItem
      label="result"
      value={
        <span className="inline-flex items-center gap-2">
          <StatusBadge value={toStatus(report?.health)} />
          <span>{formatDate(reportTimestamp(report), DATE_TIME_FORMAT)}</span>
        </span>
      }
    />
    <DetailItem label="reason" value={reason} />
    <Button type="button" variant="outline" size="sm" disabled={!report} onClick={onInspect}>
      Inspect report
    </Button>
  </DetailGroup>
);
