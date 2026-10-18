import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  FieldPanel,
  Skeleton,
  Switch,
  useToast,
} from "@/design-system";
import {
  type ApiAlertChannelResponse,
  type ApiAlertRuleResponse,
  type ApiRuleReplayResponse,
  getGetAlertRulesQueryKey,
  useCreateAlertRule,
  useDeleteAlertRule,
  useDisableAlertRule,
  useEnableAlertRule,
  useGetAlertRules,
  useReplayAlertRule,
  useUpdateAlertRule,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import {
  type RouteFormState,
  defaultRouteForm,
  formToRequest,
  routeToForm,
} from "../alerts.domain";
import { RuleDialog } from "../rules/rule-dialog";

const Cell = ({ label, children }: { label: string; children: React.ReactNode }) => (
  <div className="min-w-0 border-l border-line-soft px-3.5 py-3 first:border-l-0">
    <span className="block font-mono text-[10.5px] font-medium uppercase tracking-[0.1em] text-faint">
      {label}
    </span>
    <div className="mt-1 text-[13px]">{children}</div>
  </div>
);

export const RulesTab = ({
  channels,
  editor,
  onEditorChange,
}: {
  channels: ApiAlertChannelResponse[];
  /** The new/edit dialog is opened from the page header's New rule button. */
  editor: { open: boolean; rule?: ApiAlertRuleResponse };
  onEditorChange: (next: { open: boolean; rule?: ApiAlertRuleResponse }) => void;
}) => {
  const queryClient = useQueryClient();
  const toast = useToast();
  const query = useGetAlertRules();
  const rules = query.data?.rules ?? [];
  const [selectedId, setSelectedId] = useState<string>();
  const selected = rules.find((rule) => rule.id === selectedId) ?? rules[0];
  const [form, setForm] = useState<RouteFormState>(() => defaultRouteForm(channels));
  const [error, setError] = useState<string>();
  const [replay, setReplay] = useState<ApiRuleReplayResponse>();
  const [deleting, setDeleting] = useState(false);

  const refresh = () =>
    void queryClient.invalidateQueries({ queryKey: getGetAlertRulesQueryKey() });
  const fail = (title: string) => () => toast.add({ title });
  const create = useCreateAlertRule();
  const update = useUpdateAlertRule();
  const enable = useEnableAlertRule({
    mutation: { onSuccess: refresh, onError: fail("Unable to enable the rule") },
  });
  const disable = useDisableAlertRule({
    mutation: { onSuccess: refresh, onError: fail("Unable to disable the rule") },
  });
  const remove = useDeleteAlertRule({
    mutation: {
      onSuccess: () => {
        refresh();
        toast.add({ title: "Rule deleted" });
      },
      onError: fail("Unable to delete the rule"),
    },
  });
  const run = useReplayAlertRule({
    mutation: { onSuccess: setReplay, onError: fail("Unable to replay the rule") },
  });

  // Seed the form whenever the dialog opens.
  const [seeded, setSeeded] = useState(false);
  if (editor.open && !seeded) {
    setForm(editor.rule ? routeToForm(editor.rule, channels) : defaultRouteForm(channels));
    setError(undefined);
    setSeeded(true);
  }
  if (!editor.open && seeded) setSeeded(false);

  const save = async () => {
    setError(undefined);
    try {
      const data = formToRequest(form);
      if (editor.rule?.id) await update.mutateAsync({ id: editor.rule.id, data });
      else await create.mutateAsync({ data });
      refresh();
      toast.add({ title: editor.rule ? "Rule saved" : "Rule created" });
      onEditorChange({ open: false });
    } catch (cause) {
      setError((cause as Error).message || "Unable to save the rule.");
    }
  };

  if (query.isPending) return <Skeleton className="mt-3 h-40 w-full rounded-xl" />;
  if (query.isError) {
    return (
      <EmptyState
        tone="error"
        title="Unable to load rules"
        action={
          <Button size="sm" variant="secondary" onClick={() => void query.refetch()}>
            Retry
          </Button>
        }
      />
    );
  }
  if (rules.length === 0) {
    return (
      <div className="mt-3 flex min-h-[207px] flex-col items-center justify-center gap-1 rounded-xl border border-hairline bg-card px-4 text-center shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        <b className="text-[15px] font-medium">No rules yet</b>
        <p className="text-[13px] text-mid">
          With no rules and no matching route, every event goes to every enabled channel.
        </p>
        <Button size="sm" className="mt-3" onClick={() => onEditorChange({ open: true })}>
          New rule
        </Button>
        <RuleDialog
          open={editor.open}
          editing={false}
          form={form}
          channels={channels}
          error={error}
          pending={create.isPending}
          onChange={setForm}
          onOpenChange={(open) => onEditorChange({ open })}
          onSubmit={() => void save()}
        />
      </div>
    );
  }

  const fired = selected?.last_24h_fired ?? 0;
  const how = selected?.how_it_runs;
  return (
    <div className="mt-3 flex flex-col gap-3">
      <div className="overflow-hidden rounded-xl border border-hairline bg-card shadow-[0_1px_2px_rgba(0,0,0,0.05)]">
        {rules.map((rule) => (
          <div
            key={rule.id}
            role="button"
            tabIndex={0}
            onClick={() => setSelectedId(rule.id)}
            onKeyDown={(event) => event.key === "Enter" && setSelectedId(rule.id)}
            className="grid cursor-pointer grid-cols-[auto_minmax(0,0.7fr)_minmax(0,1.5fr)_auto] items-center gap-4 border-t border-line-soft px-3.5 py-2.5 text-[13px] transition-colors first:border-t-0 hover:bg-sub"
          >
            <span onClick={(event) => event.stopPropagation()}>
              <Switch
                aria-label={`${rule.name} enabled`}
                checked={rule.enabled ?? true}
                onCheckedChange={(next) => (next ? enable : disable).mutate({ id: rule.id ?? "" })}
              />
            </span>
            <b className="truncate font-medium">{rule.name}</b>
            <span className="truncate text-faint">{rule.summary}</span>
            <Button
              size="sm"
              variant="secondary"
              onClick={(event) => {
                event.stopPropagation();
                onEditorChange({ open: true, rule });
              }}
            >
              Edit
            </Button>
          </div>
        ))}
      </div>
      {selected && (
        <FieldPanel
          title={`${selected.name}, how it runs`}
          aside={`Last 24h: ${fired === 0 ? "did not fire" : fired === 1 ? "fired once" : `fired ${fired} times`}`}
        >
          <div className="grid grid-cols-4 overflow-hidden rounded-[10px] border border-hairline bg-card">
            <Cell label="When">{how?.when ?? "-"}</Cell>
            <Cell label="If">{how?.if ?? "-"}</Cell>
            <Cell label="Limit">{how?.limit ?? "-"}</Cell>
            <Cell label="Notify">
              {how?.quiet ? "Nobody (suppressed)" : (how?.notify ?? []).join(", ") || "-"}
            </Cell>
          </div>
          <div className="mt-3 flex items-center gap-3 text-[13px] text-quiet">
            <Button
              size="sm"
              variant="secondary"
              disabled={run.isPending}
              onClick={() => run.mutate({ id: selected.id ?? "", params: { hours: 24 } })}
            >
              {run.isPending ? "Replaying..." : "Dry run"}
            </Button>
            <span className="flex-1">
              Replays the last 24 hours and shows what would be sent. Nothing is delivered.
            </span>
            <Button size="sm" variant="ghost" onClick={() => setDeleting(true)}>
              Delete rule
            </Button>
          </div>
        </FieldPanel>
      )}
      <RuleDialog
        open={editor.open}
        editing={Boolean(editor.rule)}
        form={form}
        channels={channels}
        error={error}
        pending={create.isPending || update.isPending}
        onChange={setForm}
        onOpenChange={(open) => onEditorChange({ ...editor, open })}
        onSubmit={() => void save()}
      />
      <Dialog open={Boolean(replay)} onOpenChange={(open) => !open && setReplay(undefined)}>
        <DialogContent className="max-w-[520px] gap-3 rounded-2xl p-5">
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">Dry run</DialogTitle>
          </DialogHeader>
          <p className="text-[13px] text-quiet">
            {replay?.incidents_checked ?? 0} incidents in the last {replay?.window_hours ?? 24}{" "}
            hours checked; {replay?.would_send?.length ?? 0} would send.
          </p>
          <div className="max-h-[280px] overflow-auto rounded-[10px] border border-hairline">
            {(replay?.would_send ?? []).map((item, index) => (
              <div
                key={`${item.incident_id}-${index}`}
                className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 border-t border-line-soft px-3 py-2 text-[13px] first:border-t-0"
              >
                <span className="font-mono text-xs text-faint">{formatDate(item.at, "HH:mm")}</span>
                <span className="truncate">
                  <b className="font-medium">{item.title}</b>{" "}
                  <span className="text-faint">{item.incident_display_id}</span>
                </span>
                <span className="text-faint">{(item.channels ?? []).join(", ")}</span>
              </div>
            ))}
            {(replay?.would_send ?? []).length === 0 && (
              <p className="px-3 py-4 text-[13px] text-quiet">
                This rule would not have sent anything.
              </p>
            )}
          </div>
          {replay?.note && <p className="text-xs text-quiet">{replay.note}</p>}
          <DialogFooter>
            <Button onClick={() => setReplay(undefined)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${selected?.name}?`}
        description="Matching alerts stop using this rule."
        confirmLabel="Delete rule"
        onConfirm={() => selected?.id && remove.mutate({ id: selected.id })}
      />
    </div>
  );
};
