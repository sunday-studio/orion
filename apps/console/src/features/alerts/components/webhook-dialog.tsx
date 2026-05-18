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
import type { FormEvent } from "react";
import { alertEventOptions } from "./alert-constants";

type WebhookDialogProps = {
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
  onUrlChange: (url: string) => void;
  onOpenChange: (open: boolean) => void;
  url: string;
};

export const WebhookDialog = ({
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
  onUrlChange,
  onOpenChange,
  url,
}: WebhookDialogProps) => (
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
          <DialogTitle>{isEditing ? "Edit webhook" : "New webhook"}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? "Update the webhook name, enabled state, or replace the stored URL."
              : "Add a webhook channel for incident and recovery notifications."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="block space-y-1.5 text-sm">
            <span className="font-medium">Name</span>
            <Input
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="ops-webhook"
              required
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span className="font-medium">Webhook URL</span>
            <Input
              value={url}
              onChange={(event) => onUrlChange(event.target.value)}
              placeholder={isEditing ? "Leave blank to keep the current URL" : "https://example.com/webhook"}
              required={!isEditing}
              type="url"
            />
          </label>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={enabled} onCheckedChange={(value) => onEnabledChange(value === true)} />
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
            disabled={isPending || !name.trim() || (!isEditing && !url.trim()) || events.length === 0}
          >
            {isPending
              ? isEditing
                ? "Saving..."
                : "Creating..."
              : isEditing
                ? "Save webhook"
                : "Create webhook"}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
);
