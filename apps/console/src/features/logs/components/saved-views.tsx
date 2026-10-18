import { useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import { useState } from "react";
import {
  Badge,
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
          <span key={view.id} className="group inline-flex items-center">
            <button
              type="button"
              aria-pressed={active}
              className="rounded-md outline-none focus-visible:ring-2 focus-visible:ring-focus"
              onClick={() => onApply(view.filters ?? {})}
            >
              <Badge
                tone={active ? "up" : "unknown"}
                shape={active ? "dot" : "none"}
                className={cn(
                  "h-6 px-2.5 text-[12.5px] normal-case transition-colors",
                  !active && "hover:bg-[var(--tone-stale-bg)]",
                )}
              >
                {view.name}
              </Badge>
            </button>
            <button
              type="button"
              aria-label={`Delete ${view.name}`}
              className="flex size-0 items-center justify-center overflow-hidden text-faint outline-none transition-[width,height] hover:text-ink focus-visible:size-5 group-hover:h-5 group-hover:w-5"
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
