import { useEffect, useState } from "react";
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
import type {
  ApiAlertChannelResponse,
  ApiLabelRouteRequest,
  ApiLabelRouteResponse,
} from "@/orion-sdk";

type Row = { key: string; op: "=" | "!="; value: string };

const label = "flex min-w-0 flex-col gap-1.5 text-[13px] font-medium";

export const RouteDialog = ({
  open,
  route,
  channels,
  pending,
  error,
  onOpenChange,
  onSave,
  onDelete,
}: {
  open: boolean;
  route?: ApiLabelRouteResponse;
  channels: ApiAlertChannelResponse[];
  pending: boolean;
  error?: string;
  onOpenChange: (open: boolean) => void;
  onSave: (data: ApiLabelRouteRequest) => void;
  onDelete?: () => void;
}) => {
  const [name, setName] = useState("");
  const [rows, setRows] = useState<Row[]>([]);
  const [channelIds, setChannelIds] = useState<string[]>([]);
  const [cont, setCont] = useState(false);
  const [enabled, setEnabled] = useState(true);

  useEffect(() => {
    if (!open) return;
    setName(route?.name ?? "");
    setRows(
      (route?.matchers ?? []).map((m) => ({
        key: m.key ?? "",
        op: m.op === "!=" ? "!=" : "=",
        value: m.value ?? "",
      })),
    );
    setChannelIds(route?.channel_ids ?? []);
    setCont(route?.continue_matching ?? false);
    setEnabled(route?.enabled ?? true);
  }, [open, route]);

  const valid = name.trim() !== "" && channelIds.length > 0;
  const patchRow = (index: number, patch: Partial<Row>) =>
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-[520px] gap-3.5 overflow-y-auto rounded-2xl p-5">
        <form
          className="-mx-1.5 flex min-w-0 flex-col gap-3.5 px-1.5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!valid) return;
            onSave({
              name: name.trim(),
              enabled,
              continue_matching: cont,
              channel_ids: channelIds,
              matchers: rows
                .filter((row) => row.key.trim())
                .map((row) => ({ key: row.key.trim(), op: row.op, value: row.value.trim() })),
            });
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-[15px] font-medium">
              {route ? "Edit route" : "New route"}
            </DialogTitle>
          </DialogHeader>
          <label className={label}>
            Name
            <Input
              className="font-normal"
              value={name}
              placeholder="Production criticals"
              onChange={(event) => setName(event.target.value)}
            />
          </label>
          <div className={label}>
            When labels match
            {rows.map((row, index) => (
              <div key={index} className="grid grid-cols-[1fr_72px_1fr_auto] items-center gap-2">
                <Input
                  aria-label="Label key"
                  className="font-normal"
                  value={row.key}
                  placeholder="env"
                  onChange={(event) => patchRow(index, { key: event.target.value })}
                />
                <Select
                  aria-label="Operator"
                  className="min-w-0 font-normal"
                  options={[
                    { value: "=", label: "is" },
                    { value: "!=", label: "is not" },
                  ]}
                  value={row.op}
                  onValueChange={(next) => patchRow(index, { op: next === "!=" ? "!=" : "=" })}
                />
                <Input
                  aria-label="Label value"
                  className="font-normal"
                  value={row.value}
                  placeholder="prod"
                  onChange={(event) => patchRow(index, { value: event.target.value })}
                />
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  onClick={() => setRows((current) => current.filter((_, i) => i !== index))}
                >
                  Remove
                </Button>
              </div>
            ))}
            <button
              type="button"
              className="w-fit text-[13px] font-normal text-mid hover:text-ink"
              onClick={() => setRows((current) => [...current, { key: "", op: "=", value: "" }])}
            >
              + Add matcher
            </button>
            <span className="text-[13px] font-normal text-quiet">
              No matchers means the route matches everything. Labels include severity.
            </span>
          </div>
          <div className={label}>
            Send to
            {channels.length === 0 ? (
              <Callout tone="warn">Add a channel first.</Callout>
            ) : (
              <div className="flex flex-col gap-1.5 font-normal">
                {channels.map((channel) => (
                  <label key={channel.id} className="flex items-center gap-2">
                    <Checkbox
                      checked={channelIds.includes(channel.id ?? "")}
                      onCheckedChange={(checked) =>
                        setChannelIds((current) =>
                          checked
                            ? [...new Set([...current, channel.id ?? ""])]
                            : current.filter((id) => id !== channel.id),
                        )
                      }
                    />
                    {channel.name}
                  </label>
                ))}
              </div>
            )}
          </div>
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox checked={cont} onCheckedChange={(next) => setCont(Boolean(next))} />
            Continue to the next route after this one matches
          </label>
          <label className="flex items-center gap-2 text-[13px]">
            <Checkbox checked={enabled} onCheckedChange={(next) => setEnabled(Boolean(next))} />
            Enabled
          </label>
          {error && <Callout tone="warn">{error}</Callout>}
          <DialogFooter className="items-center sm:justify-between">
            {onDelete ? (
              <Button type="button" variant="destructive" onClick={onDelete}>
                Delete route
              </Button>
            ) : (
              <span />
            )}
            <div className="flex gap-2">
              <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={!valid || pending}>
                Save route
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
