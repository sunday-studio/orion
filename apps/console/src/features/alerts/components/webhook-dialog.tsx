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
import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import type { FormEvent } from "react";
import {
  alertChannelTypeLabel,
  alertChannelTypeOptions,
  alertEventOptions,
} from "./alert-constants";

type WebhookDialogProps = {
  channelType: string;
  enabled: boolean;
  events: string[];
  isEditing: boolean;
  isOpen: boolean;
  isPending: boolean;
  mutationError: string;
  name: string;
  onClose: () => void;
  onEnabledChange: (enabled: boolean) => void;
  onEventToggle: (event: string, enabled: boolean) => void;
  onNameChange: (name: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onTypeChange: (type: string) => void;
  onUrlChange: (url: string) => void;
  onOpenChange: (open: boolean) => void;
  url: string;
};

export const WebhookDialog = ({
  channelType,
  enabled,
  events,
  isEditing,
  isOpen,
  isPending,
  mutationError,
  name,
  onClose,
  onEnabledChange,
  onEventToggle,
  onNameChange,
  onSubmit,
  onTypeChange,
  onUrlChange,
  onOpenChange,
  url,
}: WebhookDialogProps) => {
  const typeLabel = alertChannelTypeLabel(channelType);
  const selectedType =
    alertChannelTypeOptions.find((option) => option.value === channelType) ??
    alertChannelTypeOptions[0];

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (open) {
          onOpenChange(true);
          return;
        }
        onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        <form className="space-y-5" onSubmit={onSubmit}>
          <DialogHeader>
            <DialogTitle>{isEditing ? `Edit ${typeLabel}` : "New alert destination"}</DialogTitle>
            <DialogDescription>
              {isEditing
                ? "Update the destination name, URL, enabled state, and event subscriptions."
                : "Add a webhook-backed destination for incident and recovery notifications."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3">
            <label className="block space-y-1.5 text-sm">
              <span>Type</span>
              <Select value={channelType} onValueChange={onTypeChange}>
                <SelectTrigger className="w-full">{typeLabel}</SelectTrigger>
                <SelectContent>
                  {alertChannelTypeOptions.map((option) => (
                    <SelectItem key={option.value} value={option.value}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <label className="block space-y-1.5 text-sm">
              <span>Name</span>
              <Input
                value={name}
                onChange={(event) => onNameChange(event.target.value)}
                placeholder={
                  channelType === "slack"
                    ? "ops-slack"
                    : channelType === "discord"
                      ? "ops-discord"
                      : "ops-webhook"
                }
                required
              />
            </label>
            <label className="block space-y-1.5 text-sm">
              <span>Webhook URL</span>
              <Input
                value={url}
                onChange={(event) => onUrlChange(event.target.value)}
                placeholder={selectedType.urlPlaceholder}
                required
                type="url"
              />
            </label>
            <label className="flex items-center gap-2 text-sm">
              <Checkbox
                checked={enabled}
                onCheckedChange={(value) => onEnabledChange(value === true)}
              />
              <span>Enabled</span>
            </label>
            <div className="space-y-2 text-sm">
              <div className="font-medium">Events</div>
              <div className="space-y-2">
                {alertEventOptions.map((event) => (
                  <label key={event.value} className="flex items-center gap-2">
                    <Checkbox
                      checked={events.includes(event.value)}
                      onCheckedChange={(value) => onEventToggle(event.value, value === true)}
                    />
                    <span>{event.label}</span>
                  </label>
                ))}
              </div>
              {events.length === 0 && (
                <div className="text-xs text-red-700">Select at least one event.</div>
              )}
            </div>
            {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
          </div>

          <DialogFooter>
            <Button variant="ghost" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={isPending || !name.trim() || !url.trim() || events.length === 0}
            >
              {isPending
                ? isEditing
                  ? "Saving..."
                  : "Creating..."
                : isEditing
                  ? "Save destination"
                  : "Create destination"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
