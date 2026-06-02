import { EmptyState } from "@/components/shared/empty-state";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import type {
  ApiAlertChannelResponse,
  ApiAlertRouteDryRunResponse,
  ApiAlertRouteResponse,
} from "@/orion-sdk";
import { Bell, Clock3, FlaskConical, GitBranch, Webhook } from "lucide-react";
import type { FormEvent } from "react";
import { alertEventOptions, getMutationErrorMessage } from "./alert-constants";
import { Connector, FlowNode } from "./alert-flow-node";
import {
  type DryRunFormState,
  groupingOptions,
  type RouteFormState,
  severityOptions,
  toggleValue,
} from "../alerts.domain";

type AlertRuleEditorDialogProps = {
  canSubmit: boolean;
  closeEditor: () => void;
  dryRun: {
    error: unknown;
    isPending: boolean;
  };
  dryRunForm: DryRunFormState;
  dryRunResult: ApiAlertRouteDryRunResponse | null;
  editingRoute: ApiAlertRouteResponse | null;
  editorOpen: boolean;
  form: RouteFormState;
  handleDryRun: () => void;
  handleSubmit: (event: FormEvent<HTMLFormElement>) => void;
  isSaving: boolean;
  setDryRunForm: (form: DryRunFormState) => void;
  setEditorOpen: (open: boolean) => void;
  setForm: (form: RouteFormState) => void;
  webhookChannels: ApiAlertChannelResponse[];
};

export const AlertRuleEditorDialog = ({
  canSubmit,
  closeEditor,
  dryRun,
  dryRunForm,
  dryRunResult,
  editingRoute,
  editorOpen,
  form,
  handleDryRun,
  handleSubmit,
  isSaving,
  setDryRunForm,
  setEditorOpen,
  setForm,
  webhookChannels,
}: AlertRuleEditorDialogProps) => (
  <Dialog open={editorOpen} onOpenChange={(open) => (!open ? closeEditor() : setEditorOpen(true))}>
    <DialogContent className="max-h-[calc(100vh-3rem)] overflow-y-auto sm:max-w-5xl">
      <form className="space-y-5" onSubmit={handleSubmit}>
        <DialogHeader>
          <DialogTitle>{editingRoute ? "Edit alert rule" : "New alert rule"}</DialogTitle>
          <DialogDescription>
            Connect trigger conditions to suppression, grouping, and webhook delivery.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4 md:grid-cols-[1fr_1.5rem_1fr_1.5rem_1fr]">
          <FlowNode icon={GitBranch} title="Trigger">
            <div className="space-y-3">
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Name</span>
                <Input
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Priority</span>
                <Input
                  min={0}
                  type="number"
                  value={form.priority}
                  onChange={(event) => setForm({ ...form, priority: event.target.value })}
                />
              </label>
              <label className="flex items-center gap-2">
                <Checkbox
                  checked={form.enabled}
                  onCheckedChange={(checked) => setForm({ ...form, enabled: Boolean(checked) })}
                />
                <span>Enabled</span>
              </label>
            </div>
          </FlowNode>
          <Connector />
          <FlowNode icon={Bell} title="Match">
            <div className="space-y-4">
              <div className="space-y-2">
                <div className="text-xs font-medium text-neutral-700">Events</div>
                {alertEventOptions.map((option) => (
                  <label key={option.value} className="flex items-center gap-2">
                    <Checkbox
                      checked={form.eventTypes.includes(option.value)}
                      onCheckedChange={(checked) =>
                        setForm({
                          ...form,
                          eventTypes: toggleValue(form.eventTypes, option.value, Boolean(checked)),
                        })
                      }
                    />
                    <span>{option.label}</span>
                  </label>
                ))}
              </div>
              <div className="space-y-2">
                <div className="text-xs font-medium text-neutral-700">Severity</div>
                <div className="grid grid-cols-2 gap-2">
                  {severityOptions.map((option) => (
                    <label key={option.value} className="flex items-center gap-2">
                      <Checkbox
                        checked={form.severities.includes(option.value)}
                        onCheckedChange={(checked) =>
                          setForm({
                            ...form,
                            severities: toggleValue(
                              form.severities,
                              option.value,
                              Boolean(checked),
                            ),
                          })
                        }
                      />
                      <span>{option.label}</span>
                    </label>
                  ))}
                </div>
              </div>
            </div>
          </FlowNode>
          <Connector />
          <FlowNode icon={Webhook} tone={form.suppress ? "warning" : "success"} title="Action">
            <div className="space-y-3">
              <label className="flex items-center gap-2">
                <Checkbox
                  checked={form.suppress}
                  onCheckedChange={(checked) =>
                    setForm({ ...form, suppress: Boolean(checked), channelIds: [] })
                  }
                />
                <span>Suppress matching alerts</span>
              </label>
              {!form.suppress && (
                <div className="space-y-2">
                  <div className="text-xs font-medium text-neutral-700">Webhook destinations</div>
                  {webhookChannels.length === 0 ? (
                    <div className="text-sm text-red-700">Create a webhook destination first.</div>
                  ) : (
                    webhookChannels.map((channel) => (
                      <label key={channel.id} className="flex items-center gap-2">
                        <Checkbox
                          checked={form.channelIds.includes(channel.id ?? "")}
                          onCheckedChange={(checked) =>
                            setForm({
                              ...form,
                              channelIds: toggleValue(
                                form.channelIds,
                                channel.id ?? "",
                                Boolean(checked),
                              ),
                            })
                          }
                        />
                        <span>{channel.name ?? channel.id}</span>
                      </label>
                    ))
                  )}
                </div>
              )}
            </div>
          </FlowNode>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <FlowNode icon={Clock3} title="Grouping">
            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Policy</span>
                <Select
                  value={form.groupingPolicy}
                  onValueChange={(value) => setForm({ ...form, groupingPolicy: value })}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {groupingOptions.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Delay seconds</span>
                <Input
                  min={1}
                  type="number"
                  value={form.groupingDelaySeconds}
                  onChange={(event) =>
                    setForm({ ...form, groupingDelaySeconds: event.target.value })
                  }
                />
              </label>
            </div>
          </FlowNode>
          <FlowNode icon={GitBranch} title="Filters">
            <div className="grid gap-3 sm:grid-cols-3">
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Agent IDs</span>
                <Textarea
                  className="min-h-20 text-sm"
                  value={form.agentIds}
                  onChange={(event) => setForm({ ...form, agentIds: event.target.value })}
                />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Monitor IDs</span>
                <Textarea
                  className="min-h-20 text-sm"
                  value={form.monitorIds}
                  onChange={(event) => setForm({ ...form, monitorIds: event.target.value })}
                />
              </label>
              <label className="block space-y-1">
                <span className="text-xs font-medium text-neutral-700">Monitor types</span>
                <Textarea
                  className="min-h-20 text-sm"
                  value={form.monitorTypes}
                  onChange={(event) => setForm({ ...form, monitorTypes: event.target.value })}
                />
              </label>
            </div>
          </FlowNode>
        </div>

        <div className="border border-neutral-300 bg-neutral-50 p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm font-medium">
              <FlaskConical className="size-4" />
              Dry Run
            </div>
            <Button size="sm" variant="outline" disabled={dryRun.isPending} onClick={handleDryRun}>
              {dryRun.isPending ? "Running..." : "Run"}
            </Button>
          </div>
          <div className="grid gap-3 md:grid-cols-3">
            <Select
              value={dryRunForm.eventType}
              onValueChange={(value) => setDryRunForm({ ...dryRunForm, eventType: value })}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {alertEventOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              value={dryRunForm.severity}
              onValueChange={(value) => setDryRunForm({ ...dryRunForm, severity: value })}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {severityOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              placeholder="incident id"
              value={dryRunForm.incidentId}
              onChange={(event) => setDryRunForm({ ...dryRunForm, incidentId: event.target.value })}
            />
            <Input
              placeholder="agent id"
              value={dryRunForm.agentId}
              onChange={(event) => setDryRunForm({ ...dryRunForm, agentId: event.target.value })}
            />
            <Input
              placeholder="monitor id"
              value={dryRunForm.monitorId}
              onChange={(event) => setDryRunForm({ ...dryRunForm, monitorId: event.target.value })}
            />
            <Input
              placeholder="monitor type"
              value={dryRunForm.monitorType}
              onChange={(event) =>
                setDryRunForm({ ...dryRunForm, monitorType: event.target.value })
              }
            />
          </div>
          {Boolean(dryRun.error) && (
            <EmptyState
              className="mt-3 min-h-24"
              title="Unable to dry-run alert routes"
              description={getMutationErrorMessage(dryRun.error, "Dry run failed.")}
              tone="error"
            />
          )}
          {dryRunResult && (
            <div className="mt-4 grid gap-3 md:grid-cols-2">
              <div className="border border-neutral-300 bg-white p-3 text-sm">
                <div className="font-medium">Route evaluations</div>
                {dryRunResult.route_evaluations?.length ? (
                  <ul className="mt-2 space-y-2">
                    {dryRunResult.route_evaluations.map((evaluation, index) => (
                      <li key={`${evaluation.route?.id ?? "route"}-${index}`}>
                        <span className="font-medium">{evaluation.route?.name ?? "Unnamed"}</span>
                        {evaluation.matched ? " matched" : " did not match"}
                        {evaluation.reasons?.length ? `: ${evaluation.reasons.join(", ")}` : ""}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <EmptyState title="No route evaluations returned." />
                )}
              </div>
              <div className="border border-neutral-300 bg-white p-3 text-sm">
                <div className="font-medium">Destination decisions</div>
                {dryRunResult.destination_decisions?.length ? (
                  <ul className="mt-2 space-y-2">
                    {dryRunResult.destination_decisions.map((decision, index) => (
                      <li key={`${decision.channel_id ?? "decision"}-${index}`}>
                        <span className="font-medium">
                          {decision.channel_name ?? decision.channel_id ?? "Destination"}
                        </span>
                        {` ${decision.status ?? "unknown"}`}
                        {decision.reason ? `: ${decision.reason}` : ""}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <div className="mt-2 text-neutral-600">
                    {dryRunResult.suppressed
                      ? (dryRunResult.suppression_reason ?? "Suppressed")
                      : "No destinations selected."}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={closeEditor}>
            Cancel
          </Button>
          <Button type="submit" disabled={!canSubmit}>
            {isSaving ? "Saving..." : editingRoute ? "Save rule" : "Create rule"}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
);
