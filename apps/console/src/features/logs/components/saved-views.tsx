import { useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  useToast,
} from "@/design-system";
import {
  type ApiLogSavedView,
  getListLogViewsQueryKey,
  useCreateLogView,
  useDeleteLogView,
  useListLogViews,
} from "@/orion-sdk";
import { cn } from "@/utils/cn";

const same = (a: Record<string, string>, b: Record<string, string>) => {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  return [...keys].every((key) => (a[key] ?? "") === (b[key] ?? ""));
};

/** Saved views as a row of chips under the tabs: click to apply, x to delete. */
export const SavedViewsBar = ({
  current,
  onApply,
}: {
  current: Record<string, string>;
  onApply: (filters: Record<string, string>) => void;
}) => {
  const queryClient = useQueryClient();
  const views = useListLogViews();
  const remove = useDeleteLogView({
    mutation: {
      onSuccess: () => void queryClient.invalidateQueries({ queryKey: getListLogViewsQueryKey() }),
    },
  });
  const list: ApiLogSavedView[] = views.data?.views ?? [];
  if (list.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="text-xs text-quiet">Views</span>
      {list.map((view) => {
        const active = same(view.filters ?? {}, current);
        return (
          <span
            key={view.id}
            className={cn(
              "group inline-flex h-7 items-center overflow-hidden rounded-full border text-[12.5px] transition-colors",
              active
                ? "border-brand bg-brand/10 text-ink"
                : "border-input-border bg-card text-mid hover:bg-sub",
            )}
          >
            <button
              type="button"
              aria-pressed={active}
              className="h-full pl-3 pr-2.5 outline-none focus-visible:ring-2 focus-visible:ring-focus"
              onClick={() => onApply(view.filters ?? {})}
            >
              {view.name}
            </button>
            <button
              type="button"
              aria-label={`Delete ${view.name}`}
              className="flex h-full w-0 items-center justify-center overflow-hidden text-faint outline-none transition-[width] hover:text-ink focus-visible:w-6 group-hover:w-6"
              onClick={() => remove.mutate({ id: view.id ?? "" })}
            >
              <X className="size-3" />
            </button>
          </span>
        );
      })}
    </div>
  );
};

/** "Save view" button with the naming dialog. */
export const SaveViewButton = ({ current }: { current: Record<string, string> }) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const create = useCreateLogView({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getListLogViewsQueryKey() });
        setOpen(false);
        setName("");
        toast.add({ title: "View saved" });
      },
      onError: () => toast.add({ title: "Unable to save the view" }),
    },
  });
  return (
    <>
      <Button size="sm" variant="secondary" onClick={() => setOpen(true)}>
        Save view
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-[380px] rounded-2xl p-5">
          <DialogHeader>
            <DialogTitle>Save view</DialogTitle>
          </DialogHeader>
          <Input
            aria-label="View name"
            placeholder="Errors, last hour"
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
          <DialogFooter>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button
              disabled={!name.trim() || create.isPending}
              onClick={() => create.mutate({ data: { name: name.trim(), filters: current } })}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};
