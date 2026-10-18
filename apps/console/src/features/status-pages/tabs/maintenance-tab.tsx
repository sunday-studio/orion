import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  Callout,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  FieldPanel,
  Input,
  RaisedCard,
  Skeleton,
  StatusBadge,
  Textarea,
  useToast,
} from "@/design-system";
import {
  type ApiStatusPageComponentResponse,
  type ApiStatusPageMaintenanceResponse,
  getListStatusPageMaintenanceQueryKey,
  getPreviewStatusPageQueryKey,
  useCreateStatusPageMaintenance,
  useDeleteStatusPageMaintenance,
  useListStatusPageMaintenance,
  useUpdateStatusPageMaintenance,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";

const toLocal = (iso?: string) => {
  if (!iso) return "";
  const date = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

const WindowDialog = ({
  pageId,
  components,
  window: existing,
  onClose,
}: {
  pageId: string;
  components: ApiStatusPageComponentResponse[];
  window?: ApiStatusPageMaintenanceResponse;
  onClose: () => void;
}) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [title, setTitle] = useState(existing?.title ?? "");
  const [description, setDescription] = useState(existing?.description ?? "");
  const [startsAt, setStartsAt] = useState(toLocal(existing?.starts_at));
  const [endsAt, setEndsAt] = useState(toLocal(existing?.ends_at));
  const [ids, setIds] = useState<string[]>(existing?.component_ids ?? []);
  const [hold, setHold] = useState(existing?.hold_alerts ?? false);
  const [notify, setNotify] = useState(existing?.notify_subscribers ?? false);
  const [error, setError] = useState<string>();
  const options = {
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({
          queryKey: getListStatusPageMaintenanceQueryKey(pageId),
        });
        void queryClient.invalidateQueries({ queryKey: getPreviewStatusPageQueryKey(pageId) });
        onClose();
      },
      onError: (failure: unknown) =>
        setError((failure as Error).message || "Unable to save the window."),
    },
  };
  const create = useCreateStatusPageMaintenance(options);
  const update = useUpdateStatusPageMaintenance(options);
  const remove = useDeleteStatusPageMaintenance({
    mutation: { ...options.mutation, onError: () => toast.add({ title: "Unable to delete" }) },
  });
  const pending = create.isPending || update.isPending;
  const save = () => {
    setError(undefined);
    const data = {
      title: title.trim(),
      description,
      starts_at: new Date(startsAt).toISOString(),
      ends_at: new Date(endsAt).toISOString(),
      component_ids: ids,
      hold_alerts: hold,
      notify_subscribers: notify,
    };
    if (existing?.id) update.mutate({ id: pageId, windowId: existing.id, data });
    else create.mutate({ id: pageId, data });
  };
  const toggle = (id: string) =>
    setIds((current) =>
      current.includes(id) ? current.filter((x) => x !== id) : [...current, id],
    );
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-[500px] gap-3.5 rounded-2xl p-5">
        <DialogHeader>
          <DialogTitle>{existing ? "Edit maintenance" : "Schedule maintenance"}</DialogTitle>
        </DialogHeader>
        <label className="grid gap-1.5 text-[12.5px] font-medium text-mid">
          Title
          <Input value={title} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <label className="grid gap-1.5 text-[12.5px] font-medium text-mid">
          Description
          <Textarea
            rows={2}
            value={description}
            onChange={(event) => setDescription(event.target.value)}
          />
        </label>
        <div className="grid grid-cols-2 gap-3">
          <label className="grid gap-1.5 text-[12.5px] font-medium text-mid">
            Starts
            <Input
              type="datetime-local"
              value={startsAt}
              onChange={(event) => setStartsAt(event.target.value)}
            />
          </label>
          <label className="grid gap-1.5 text-[12.5px] font-medium text-mid">
            Ends
            <Input
              type="datetime-local"
              value={endsAt}
              onChange={(event) => setEndsAt(event.target.value)}
            />
          </label>
        </div>
        <div className="grid gap-1.5">
          <span className="text-[12.5px] font-medium text-mid">Affected components</span>
          {components.map((component) => (
            <label key={component.id} className="flex items-center gap-2 text-[13px]">
              <Checkbox
                checked={ids.includes(component.id ?? "")}
                onCheckedChange={() => toggle(component.id ?? "")}
              />
              {component.public_name}
            </label>
          ))}
        </div>
        <label className="flex items-center gap-2 text-[13px]">
          <Checkbox checked={hold} onCheckedChange={(next) => setHold(Boolean(next))} />
          Hold alerts and incidents for these components
        </label>
        <label className="flex items-center gap-2 text-[13px]">
          <Checkbox checked={notify} onCheckedChange={(next) => setNotify(Boolean(next))} />
          Email subscribers before it starts
        </label>
        {error && <Callout tone="warn">{error}</Callout>}
        <DialogFooter>
          {existing?.id && (
            <Button
              variant="destructive"
              onClick={() => remove.mutate({ id: pageId, windowId: existing.id ?? "" })}
            >
              Delete
            </Button>
          )}
          <Button
            disabled={pending || !title.trim() || !startsAt || !endsAt || ids.length === 0}
            onClick={save}
          >
            {existing ? "Save" : "Schedule"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

const statusTone = (status?: string) =>
  status === "in_progress" ? "maintenance" : status === "completed" ? "up" : "unknown";

export const MaintenanceTab = ({
  pageId,
  components,
}: {
  pageId: string;
  components: ApiStatusPageComponentResponse[];
}) => {
  const list = useListStatusPageMaintenance(pageId);
  const [editing, setEditing] = useState<ApiStatusPageMaintenanceResponse | "new">();
  if (list.isPending) return <Skeleton className="h-40 w-full rounded-xl" />;
  if (list.isError) return <EmptyState tone="error" title="Unable to load maintenance" />;
  const windows = list.data?.maintenance ?? [];
  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => setEditing("new")}>
          Schedule maintenance
        </Button>
      </div>
      {windows.length === 0 ? (
        <EmptyState
          title="No maintenance scheduled"
          description="Planned work shows on the page ahead of time."
        />
      ) : (
        <FieldPanel title="Maintenance windows" aside={`${windows.length}`}>
          <RaisedCard>
            {windows.map((item) => (
              <div
                key={item.id}
                className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3.5 py-2.5 text-[13px] first:border-t-0"
              >
                <span className="min-w-0">
                  <b className="block truncate font-medium">{item.title}</b>
                  <small className="text-xs text-faint">
                    {formatDate(item.starts_at, "MMM d, HH:mm")} to{" "}
                    {formatDate(item.ends_at, "MMM d, HH:mm")}
                    {item.component_names?.length ? ` · ${item.component_names.join(", ")}` : ""}
                  </small>
                </span>
                <StatusBadge status={statusTone(item.status)}>
                  {(item.status ?? "scheduled").replace("_", " ")}
                </StatusBadge>
                <Button size="sm" variant="secondary" onClick={() => setEditing(item)}>
                  Edit
                </Button>
              </div>
            ))}
          </RaisedCard>
        </FieldPanel>
      )}
      {editing && (
        <WindowDialog
          key={editing === "new" ? "new" : editing.id}
          pageId={pageId}
          components={components}
          window={editing === "new" ? undefined : editing}
          onClose={() => setEditing(undefined)}
        />
      )}
    </div>
  );
};
