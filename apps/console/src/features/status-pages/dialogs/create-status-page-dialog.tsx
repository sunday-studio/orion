import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  AnimatedHeight,
  Button,
  Callout,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Segmented,
} from "@/design-system";
import { checkStatusPageSlug, getListStatusPagesQueryKey, useCreateStatusPage } from "@/orion-sdk";
import { slugify } from "../status-format";

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";

export const CreateStatusPageDialog = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const create = useCreateStatusPage();
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [touched, setTouched] = useState(false);
  const [startFrom, setStartFrom] = useState("blank");
  const [slugState, setSlugState] = useState<{ available: boolean; reason?: string } | null>(null);
  const [error, setError] = useState<string>();

  useEffect(() => {
    if (!open) return;
    setName("");
    setSlug("");
    setTouched(false);
    setStartFrom("blank");
    setSlugState(null);
    setError(undefined);
  }, [open]);

  useEffect(() => {
    if (!slug) {
      setSlugState(null);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      checkStatusPageSlug({ slug })
        .then((result) => {
          if (!cancelled)
            setSlugState({ available: Boolean(result.available), reason: result.reason });
        })
        .catch(() => !cancelled && setSlugState(null));
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [slug]);

  const valid = name.trim() !== "" && slug !== "" && slugState?.available !== false;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5">
        <form
          className="-mx-1.5 flex min-w-0 flex-col gap-3.5 px-1.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!valid) return;
            setError(undefined);
            create.mutate(
              { data: { title: name.trim(), slug, start_from: startFrom } },
              {
                onSuccess: (result) => {
                  void queryClient.invalidateQueries({ queryKey: getListStatusPagesQueryKey() });
                  onOpenChange(false);
                  const id = (result as { page?: { id?: string } }).page?.id;
                  if (id) navigate(`/status-pages/${id}`);
                },
                onError: (failure) =>
                  setError((failure as Error).message || "Unable to create the page."),
              },
            );
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">New status page</DialogTitle>
          </DialogHeader>
          <AnimatedHeight>
            <label className={label}>
              Name
              <Input
                className="font-normal"
                value={name}
                placeholder="Example Status"
                onChange={(event) => {
                  setName(event.target.value);
                  if (!touched) setSlug(slugify(event.target.value));
                }}
              />
            </label>
            <label className={label}>
              Address
              <Input
                className="font-normal"
                value={slug}
                placeholder="example"
                onChange={(event) => {
                  setTouched(true);
                  setSlug(slugify(event.target.value));
                }}
              />
              <span className="text-[13px] font-normal text-quiet">
                {slugState?.available === false
                  ? (slugState.reason ?? "That address is taken.")
                  : slug
                    ? `Public at /status/${slug}`
                    : "Lowercase letters, numbers and dashes."}
              </span>
            </label>
            <div className={label}>
              Start from
              <Segmented
                aria-label="Start from"
                surface="card"
                options={[
                  { value: "blank", label: "Blank" },
                  { value: "monitors", label: "Suggest from my monitors" },
                ]}
                value={startFrom}
                onValueChange={setStartFrom}
              />
            </div>
            <p className="text-[13px] text-quiet">
              Pages start as drafts. Nothing is public until you publish.
            </p>
            {error && <Callout tone="warn">{error}</Callout>}
          </AnimatedHeight>
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!valid || create.isPending}>
              Create page
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
