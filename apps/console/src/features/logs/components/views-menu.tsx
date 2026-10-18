import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
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
  useToast,
} from "@/design-system";
import {
  getListLogViewsQueryKey,
  useCreateLogView,
  useDeleteLogView,
  useListLogViews,
} from "@/orion-sdk";

/** Saved views: apply one, delete one, or save the current filters as a new one. */
export const ViewsMenu = ({
  current,
  onApply,
}: {
  current: Record<string, string>;
  onApply: (filters: Record<string, string>) => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const views = useListLogViews();
  const [naming, setNaming] = useState(false);
  const [name, setName] = useState("");
  const refresh = () => void queryClient.invalidateQueries({ queryKey: getListLogViewsQueryKey() });
  const create = useCreateLogView({
    mutation: {
      onSuccess: () => {
        refresh();
        setNaming(false);
        setName("");
        toast.add({ title: "View saved" });
      },
      onError: () => toast.add({ title: "Unable to save the view" }),
    },
  });
  const remove = useDeleteLogView({ mutation: { onSuccess: refresh } });
  return (
    <>
      <Popover>
        <PopoverTrigger
          render={
            <Button size="sm" variant="secondary">
              Save view
            </Button>
          }
        />
        <PopoverContent align="end" className="w-[230px] p-1.5">
          <p className="px-2.5 py-1.5 text-xs text-quiet">Saved views</p>
          {(views.data?.views ?? []).map((view) => (
            <div key={view.id} className="group flex items-center rounded-md hover:bg-sub">
              <button
                type="button"
                className="min-w-0 flex-1 truncate px-2.5 py-2 text-left text-[13px]"
                onClick={() => onApply(view.filters ?? {})}
              >
                {view.name}
              </button>
              <button
                type="button"
                aria-label={`Delete ${view.name}`}
                className="px-2 text-xs text-faint opacity-0 hover:text-ink group-hover:opacity-100"
                onClick={() => remove.mutate({ id: view.id ?? "" })}
              >
                Delete
              </button>
            </div>
          ))}
          <div className="mt-1 border-t border-line-soft pt-1">
            <button
              type="button"
              className="block w-full rounded-md px-2.5 py-2 text-left text-[13px] hover:bg-sub"
              onClick={() => setNaming(true)}
            >
              Save current view...
            </button>
          </div>
        </PopoverContent>
      </Popover>
      <Dialog open={naming} onOpenChange={setNaming}>
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
            <Button variant="secondary" onClick={() => setNaming(false)}>
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
