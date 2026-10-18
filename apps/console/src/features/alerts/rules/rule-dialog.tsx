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
  Input,
  Select,
} from "@/design-system";
import type { ApiAlertChannelResponse } from "@/orion-sdk";
import { alertEventOptions } from "../components/alert-constants";
import {
  type RouteFormState,
  groupingOptions,
  severityOptions,
  toggleValue,
} from "../alerts.domain";

const COOLDOWNS = [
  { value: "0", label: "Use the global default" },
  { value: "600", label: "Once per 10 minutes" },
  { value: "1800", label: "Once per 30 minutes" },
  { value: "3600", label: "Once per hour" },
  { value: "21600", label: "Once per 6 hours" },
  { value: "86400", label: "Once per day" },
];

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";

type Props = {
  open: boolean;
  editing: boolean;
  form: RouteFormState;
  channels: ApiAlertChannelResponse[];
  error?: string;
  pending: boolean;
  onChange: (form: RouteFormState) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
};

export const RuleDialog = ({
  open,
  editing,
  form,
  channels,
  error,
  pending,
  onChange,
  onOpenChange,
  onSubmit,
}: Props) => {
  const [advanced, setAdvanced] = useState(false);
  const valid =
    form.name.trim() !== "" &&
    form.eventTypes.length > 0 &&
    (form.suppress || form.channelIds.length > 0);
  const set = (patch: Partial<RouteFormState>) => onChange({ ...form, ...patch });
  const cooldownOptions = COOLDOWNS.some((o) => o.value === form.cooldownSeconds)
    ? COOLDOWNS
    : [
        ...COOLDOWNS,
        { value: form.cooldownSeconds, label: `Once per ${form.cooldownSeconds} seconds` },
      ];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-[520px] gap-3.5 overflow-y-auto rounded-2xl p-5">
        <form
          className="flex min-w-0 flex-col gap-3.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (valid) onSubmit();
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">
              {editing ? "Edit rule" : "New rule"}
            </DialogTitle>
          </DialogHeader>
          <label className={label}>
            Name
            <Input
              className="font-normal"
              value={form.name}
              placeholder="Critical down"
              onChange={(event) => set({ name: event.target.value })}
            />
          </label>
          <div className={label}>
            When
            <div className="flex flex-wrap gap-x-4 gap-y-1.5 font-normal">
              {alertEventOptions.map((option) => (
                <label key={option.value} className="flex items-center gap-2">
                  <Checkbox
                    checked={form.eventTypes.includes(option.value)}
                    onCheckedChange={(checked) =>
                      set({
                        eventTypes: toggleValue(form.eventTypes, option.value, Boolean(checked)),
                      })
                    }
                  />
                  {option.label}
                </label>
              ))}
            </div>
          </div>
          <div className={label}>
            If severity is
            <div className="flex flex-wrap gap-x-4 gap-y-1.5 font-normal">
              {severityOptions.map((option) => (
                <label key={option.value} className="flex items-center gap-2">
                  <Checkbox
                    checked={form.severities.includes(option.value)}
                    onCheckedChange={(checked) =>
                      set({
                        severities: toggleValue(form.severities, option.value, Boolean(checked)),
                      })
                    }
                  />
                  {option.label}
                </label>
              ))}
            </div>
            <span className="text-[13px] font-normal text-quiet">
              Leave empty to match every severity.
            </span>
          </div>
          <div className={label}>
            Limit
            <Select
              aria-label="Limit"
              className="w-full font-normal"
              options={cooldownOptions}
              value={form.cooldownSeconds}
              onValueChange={(next) => set({ cooldownSeconds: next })}
            />
          </div>
          <div className={label}>
            Notify
            <label className="flex items-center gap-2 font-normal">
              <Checkbox
                checked={form.suppress}
                onCheckedChange={(checked) => set({ suppress: Boolean(checked), channelIds: [] })}
              />
              Suppress matching alerts (notify nobody)
            </label>
            {!form.suppress &&
              (channels.length === 0 ? (
                <Callout tone="warn">Add a channel first.</Callout>
              ) : (
                <div className="flex flex-col gap-1.5 font-normal">
                  {channels.map((channel) => (
                    <label key={channel.id} className="flex items-center gap-2">
                      <Checkbox
                        checked={form.channelIds.includes(channel.id ?? "")}
                        onCheckedChange={(checked) =>
                          set({
                            channelIds: toggleValue(
                              form.channelIds,
                              channel.id ?? "",
                              Boolean(checked),
                            ),
                          })
                        }
                      />
                      {channel.name}
                    </label>
                  ))}
                </div>
              ))}
          </div>
          <button
            type="button"
            className="w-fit text-[13px] text-mid hover:text-ink"
            onClick={() => setAdvanced((current) => !current)}
          >
            {advanced ? "Hide advanced" : "Advanced filters and grouping"}
          </button>
          {advanced && (
            <div className="flex flex-col gap-3">
              <label className={label}>
                Priority
                <Input
                  type="number"
                  min={0}
                  className="font-normal"
                  value={form.priority}
                  onChange={(event) => set({ priority: event.target.value })}
                />
              </label>
              <label className={label}>
                Monitor types
                <Input
                  className="font-normal"
                  value={form.monitorTypes}
                  placeholder="http, tcp"
                  onChange={(event) => set({ monitorTypes: event.target.value })}
                />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <div className={label}>
                  Grouping
                  <Select
                    aria-label="Grouping"
                    className="w-full font-normal"
                    options={groupingOptions}
                    value={form.groupingPolicy}
                    onValueChange={(next) => set({ groupingPolicy: next })}
                  />
                </div>
                <label className={label}>
                  Delay seconds
                  <Input
                    type="number"
                    min={1}
                    className="font-normal"
                    value={form.groupingDelaySeconds}
                    onChange={(event) => set({ groupingDelaySeconds: event.target.value })}
                  />
                </label>
              </div>
            </div>
          )}
          {error && <Callout tone="warn">{error}</Callout>}
          <DialogFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!valid || pending}>
              {editing ? "Save rule" : "Create rule"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
