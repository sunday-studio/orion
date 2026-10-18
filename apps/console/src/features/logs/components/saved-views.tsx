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
  Popover,
  PopoverContent,
  PopoverTrigger,
  SelectChevron,
  selectTriggerClass,
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

/** Saved views dropdown in the toolbar: click a view to apply it, x to delete it. */
export const SavedViewsMenu = ({
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
  const activeView = list.find((view) => same(view.filters ?? {}, current));
  return (
    <Popover>
      <PopoverTrigger className={cn(selectTriggerClass, "min-w-[130px]")}>
        <span className="min-w-0 flex-1 truncate text-left">
          {activeView?.name ?? (views.isPending ? "Views" : `Views (${list.length})`)}
        </span>
        <SelectChevron />
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[240px] p-1.5">
        <p className="px-2.5 py-1.5 text-xs text-quiet">Saved views</p>
        {views.isError && (
          <p className="px-2.5 pb-2 text-[13px] text-quiet">Unable to load saved views.</p>
        )}
        {!views.isError && !views.isPending && list.length === 0 && (
          <p className="px-2.5 pb-2 text-[13px] text-quiet">
            No saved views yet. Set filters, then use Save view.
          </p>
        )}
        <div className="grid gap-1">
          {list.map((view) => {
            const active = view.id === activeView?.id;
            return (
              <Badge
                key={view.id}
                tone="down"
                shape="square"
                className={cn(
                  "h-7 w-full justify-between gap-1 pl-2.5 pr-1.5 text-[12.5px] normal-case",
                  active && "ring-1 ring-current",
                )}
              >
                <button
                  type="button"
                  aria-pressed={active}
                  className="min-w-0 flex-1 truncate rounded text-left outline-none focus-visible:ring-2 focus-visible:ring-focus"
                  onClick={() => onApply(view.filters ?? {})}
                >
                  {view.name}
                </button>
                <button
                  type="button"
                  aria-label={`Delete ${view.name}`}
                  className="flex size-4 shrink-0 items-center justify-center rounded opacity-60 outline-none transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:ring-focus"
                  onClick={() => remove.mutate({ id: view.id ?? "" })}
                >
                  <X className="size-3" />
                </button>
              </Badge>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
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
      onError: (failure) =>
        toast.add({ title: (failure as Error).message || "Unable to save the view" }),
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
