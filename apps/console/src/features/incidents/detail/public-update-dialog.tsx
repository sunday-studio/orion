import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Select,
  Textarea,
} from "@/design-system";
import {
  ApiIncidentPublicUpdateRequestStatus,
  getGetIncidentPublicTargetsQueryKey,
  getGetIncidentQueryKey,
  useGetIncidentPublicTargets,
  usePostIncidentPublicUpdate,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";

const statuses = [
  { value: ApiIncidentPublicUpdateRequestStatus.investigating, label: "Investigating" },
  { value: ApiIncidentPublicUpdateRequestStatus.identified, label: "Identified" },
  { value: ApiIncidentPublicUpdateRequestStatus.monitoring, label: "Monitoring" },
  { value: ApiIncidentPublicUpdateRequestStatus.resolved, label: "Resolved" },
] as const;

const labelClass = "flex flex-col gap-1.5 text-[13px] font-medium";

/** Posts a public status page update for an incident (canvas incident-public-update). */
export const PublicUpdateDialog = ({
  open,
  incidentId,
  incidentLabel,
  onOpenChange,
}: {
  open: boolean;
  incidentId: string;
  incidentLabel: string;
  onOpenChange: (open: boolean) => void;
}) => {
  const queryClient = useQueryClient();
  const targets = useGetIncidentPublicTargets(incidentId, { query: { enabled: open } });
  const list = useMemo(() => targets.data?.targets ?? [], [targets.data]);
  const [pageId, setPageId] = useState("");
  const [status, setStatus] = useState<string>("investigating");
  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [notify, setNotify] = useState(true);
  const [error, setError] = useState(false);
  const post = usePostIncidentPublicUpdate();

  useEffect(() => {
    if (open && !pageId && list[0]?.status_page_id) setPageId(list[0].status_page_id);
  }, [open, pageId, list]);

  const target = list.find((item) => item.status_page_id === pageId);
  const needsTitle = Boolean(target?.title_required);
  const subscribers = target?.subscriber_count ?? 0;
  const canPost =
    Boolean(pageId) &&
    message.trim() !== "" &&
    (!needsTitle || title.trim() !== "") &&
    !post.isPending;

  const submit = () => {
    setError(false);
    post.mutate(
      {
        id: incidentId,
        data: {
          status_page_id: pageId,
          status: status as (typeof statuses)[number]["value"],
          message: message.trim(),
          notify_subscribers: notify && subscribers > 0,
          title: needsTitle ? title.trim() : undefined,
        },
      },
      {
        onSuccess: () => {
          void queryClient.invalidateQueries({ queryKey: getGetIncidentQueryKey(incidentId) });
          void queryClient.invalidateQueries({
            queryKey: getGetIncidentPublicTargetsQueryKey(incidentId),
          });
          setMessage("");
          setTitle("");
          onOpenChange(false);
        },
        onError: () => setError(true),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[520px] gap-3.5 rounded-2xl p-5">
        <form
          className="flex flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (canPost) submit();
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Post public update</DialogTitle>
          </DialogHeader>
          {targets.isSuccess && list.length === 0 ? (
            <Callout tone="warn">
              No status page is available yet. Create one under Status first.
            </Callout>
          ) : (
            <div className={labelClass}>
              Status page
              <Select
                aria-label="Status page"
                className="w-full font-normal"
                options={list.map((item) => ({
                  value: item.status_page_id ?? "",
                  label: item.title ?? item.slug ?? "Status page",
                }))}
                value={pageId}
                onValueChange={setPageId}
              />
            </div>
          )}
          {needsTitle && (
            <>
              <Callout tone="warn">
                No public incident exists for {incidentLabel} yet. Give it a public title; posting
                creates it on this page.
              </Callout>
              <label className={labelClass}>
                Public title
                <Input
                  className="font-normal"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder="Elevated API errors"
                />
              </label>
            </>
          )}
          <div className={labelClass}>
            Public status
            <div
              role="radiogroup"
              aria-label="Public status"
              className="flex w-fit gap-0.5 rounded-lg bg-tray p-[3px]"
            >
              {statuses.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={status === option.value}
                  onClick={() => setStatus(option.value)}
                  className={cn(
                    "rounded-md px-3 py-1 text-[13px] font-normal outline-none focus-visible:ring-2 focus-visible:ring-focus",
                    status === option.value
                      ? "bg-chip font-medium shadow-[0_1px_2px_rgba(0,0,0,0.08)]"
                      : "text-mid",
                  )}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>
          <label className={labelClass}>
            Message
            <Textarea
              className="font-normal"
              value={message}
              onChange={(event) => setMessage(event.target.value)}
              placeholder="What do customers need to know?"
            />
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox
              checked={notify && subscribers > 0}
              disabled={subscribers === 0}
              onCheckedChange={(next) => setNotify(Boolean(next))}
            />
            {subscribers > 0
              ? `Notify ${subscribers} subscribers by email`
              : "No subscribers to notify"}
          </label>
          {error && <Callout tone="warn">Unable to post the update. Try again.</Callout>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!canPost}>
              Post update
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
