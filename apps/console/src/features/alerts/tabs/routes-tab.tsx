import { useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronUp } from "lucide-react";
import { useState } from "react";
import {
  Button,
  Callout,
  ConfirmDialog,
  EmptyState,
  FieldPanel,
  GridRow,
  GridTable,
  Input,
  Skeleton,
  StatusBadge,
  useToast,
} from "@/design-system";
import {
  type ApiAlertChannelResponse,
  type ApiLabelRouteDryRunResponse,
  type ApiLabelRouteRequest,
  type ApiLabelRouteResponse,
  getListLabelRoutesQueryKey,
  useCreateLabelRoute,
  useDeleteLabelRoute,
  useDryRunLabelRoutes,
  useListLabelRoutes,
  useReorderLabelRoutes,
  useUpdateLabelRoute,
} from "@/orion-sdk";
import { RouteDialog } from "../routes/route-dialog";

const COLUMNS = "28px minmax(0,1.4fr) minmax(0,1fr) 92px 84px";

export const RoutesTab = ({
  channels,
  editor,
  onEditorChange,
}: {
  channels: ApiAlertChannelResponse[];
  editor: { open: boolean; route?: ApiLabelRouteResponse };
  onEditorChange: (next: { open: boolean; route?: ApiLabelRouteResponse }) => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const query = useListLabelRoutes();
  const routes = query.data?.routes ?? [];
  const refresh = () =>
    void queryClient.invalidateQueries({ queryKey: getListLabelRoutesQueryKey() });
  const [error, setError] = useState<string>();
  const [deleting, setDeleting] = useState(false);
  const [labels, setLabels] = useState("env=prod, severity=critical");
  const [result, setResult] = useState<ApiLabelRouteDryRunResponse>();
  const create = useCreateLabelRoute();
  const update = useUpdateLabelRoute();
  const remove = useDeleteLabelRoute({
    mutation: {
      onSuccess: () => {
        refresh();
        onEditorChange({ open: false });
        toast.add({ title: "Route deleted" });
      },
      onError: () => toast.add({ title: "Unable to delete the route" }),
    },
  });
  const reorder = useReorderLabelRoutes({
    mutation: {
      onSuccess: refresh,
      onError: () => toast.add({ title: "Unable to reorder routes" }),
    },
  });
  const dryRun = useDryRunLabelRoutes({
    mutation: {
      onSuccess: setResult,
      onError: () => toast.add({ title: "Unable to check the route" }),
    },
  });

  const move = (index: number, by: number) => {
    const ids = routes.map((route) => route.id ?? "");
    const target = index + by;
    if (target < 0 || target >= ids.length) return;
    [ids[index], ids[target]] = [ids[target] ?? "", ids[index] ?? ""];
    reorder.mutate({ data: { ids } });
  };
  const save = async (data: ApiLabelRouteRequest) => {
    setError(undefined);
    try {
      if (editor.route?.id) await update.mutateAsync({ id: editor.route.id, data });
      else await create.mutateAsync({ data });
      refresh();
      onEditorChange({ open: false });
      toast.add({ title: editor.route ? "Route saved" : "Route added" });
    } catch (cause) {
      setError((cause as Error).message || "Unable to save the route.");
    }
  };

  if (query.isPending) return <Skeleton className="mt-3 h-40 w-full rounded-xl" />;
  if (query.isError) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load routes"
        action={
          <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  }

  return (
    <div className="mt-3 flex flex-col gap-3">
      <p className="text-center text-[13px] text-mid">
        Routes pick the channel by label. The first match wins unless Continue is on.
      </p>
      {routes.length === 0 ? (
        <EmptyState
          title="No routes yet"
          description="Without routes, events that no rule handles go to every enabled channel."
          action={
            <Button size="sm" onClick={() => onEditorChange({ open: true })}>
              New route
            </Button>
          }
        />
      ) : (
        <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
          <GridTable>
            <GridRow columns={COLUMNS} header>
              <span>#</span>
              <span>When labels match</span>
              <span>Send to</span>
              <span>Then</span>
              <span />
            </GridRow>
            {routes.map((route, index) => (
              <GridRow key={route.id} flipId={route.id} columns={COLUMNS}>
                <span className="text-faint">{route.number ?? index + 1}</span>
                <span className="text-faint">{route.when || "any"}</span>
                <b className="font-medium">{(route.channel_names ?? []).join(", ")}</b>
                <span>
                  <StatusBadge
                    status={route.then === "Continue" ? "up" : "unknown"}
                    shape={route.then === "Continue" ? undefined : "hollow"}
                  >
                    {route.then ?? "Stop"}
                  </StatusBadge>
                </span>
                <span className="flex items-center justify-end gap-1 !overflow-visible">
                  <button
                    type="button"
                    aria-label="Move up"
                    disabled={index === 0 || reorder.isPending}
                    className="rounded p-0.5 text-faint hover:text-ink disabled:opacity-30"
                    onClick={() => move(index, -1)}
                  >
                    <ChevronUp className="size-3.5" />
                  </button>
                  <button
                    type="button"
                    aria-label="Move down"
                    disabled={index === routes.length - 1 || reorder.isPending}
                    className="rounded p-0.5 text-faint hover:text-ink disabled:opacity-30"
                    onClick={() => move(index, 1)}
                  >
                    <ChevronDown className="size-3.5" />
                  </button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => onEditorChange({ open: true, route })}
                  >
                    Edit
                  </Button>
                </span>
              </GridRow>
            ))}
          </GridTable>
        </div>
      )}
      <FieldPanel title="Dry run" aside="Test a label set">
        <div className="flex items-center gap-2">
          <Input
            aria-label="Labels"
            value={labels}
            className="font-normal"
            onChange={(event) => setLabels(event.target.value)}
          />
          <Button
            size="sm"
            disabled={dryRun.isPending || !labels.trim()}
            onClick={() =>
              dryRun.mutate({
                data: {
                  labels: labels
                    .split(/[,\n]/)
                    .map((item) => item.trim())
                    .filter(Boolean),
                },
              })
            }
          >
            Check route
          </Button>
        </div>
        {result && (
          <Callout tone={(result.channels ?? []).length > 0 ? "up" : "warn"} className="mt-3">
            <b className="mr-1.5 font-semibold">
              {(result.channels ?? []).length > 0 ? "Match" : "No match"}
            </b>
            {result.summary}
          </Callout>
        )}
      </FieldPanel>
      <RouteDialog
        open={editor.open}
        route={editor.route}
        channels={channels}
        pending={create.isPending || update.isPending}
        error={error}
        onOpenChange={(open) => onEditorChange({ ...editor, open })}
        onSave={(data) => void save(data)}
        onDelete={editor.route ? () => setDeleting(true) : undefined}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${editor.route?.name ?? "this route"}?`}
        description="Events stop matching it. The remaining routes keep their order."
        confirmLabel="Delete route"
        onConfirm={() => editor.route?.id && remove.mutate({ id: editor.route.id })}
      />
    </div>
  );
};
