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
import type { ApiAlertSMTPServiceResponse } from "@/orion-sdk";
import type { FormEvent } from "react";
import { alertEventOptions } from "./alert-constants";

type EmailDestinationDialogProps = {
  emailTo: string;
  enabled: boolean;
  events: string[];
  isEditing: boolean;
  isOpen: boolean;
  isPending: boolean;
  mutationError: string;
  name: string;
  services: ApiAlertSMTPServiceResponse[];
  smtpServiceId: string;
  onClose: () => void;
  onEmailToChange: (emailTo: string) => void;
  onEnabledChange: (enabled: boolean) => void;
  onEventToggle: (event: string, enabled: boolean) => void;
  onNameChange: (name: string) => void;
  onOpenChange: (open: boolean) => void;
  onSMTPServiceChange: (serviceId: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export const EmailDestinationDialog = ({
  emailTo,
  enabled,
  events,
  isEditing,
  isOpen,
  isPending,
  mutationError,
  name,
  services,
  smtpServiceId,
  onClose,
  onEmailToChange,
  onEnabledChange,
  onEventToggle,
  onNameChange,
  onOpenChange,
  onSMTPServiceChange,
  onSubmit,
}: EmailDestinationDialogProps) => (
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
          <DialogTitle>
            {isEditing ? "Edit email destination" : "New email destination"}
          </DialogTitle>
          <DialogDescription>
            {isEditing
              ? "Update the recipient, SMTP service, enabled state, and event subscriptions."
              : "Add a reusable email recipient for alert delivery."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3">
          <label className="block space-y-1.5 text-sm">
            <span>Name</span>
            <Input
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="ops-email"
              required
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>Email to</span>
            <Input
              value={emailTo}
              onChange={(event) => onEmailToChange(event.target.value)}
              placeholder="ops@example.com"
              required
              type="email"
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>SMTP service</span>
            <Select value={smtpServiceId} onValueChange={onSMTPServiceChange}>
              <SelectTrigger>
                <span data-slot="select-value">
                  {services.find((service) => service.id === smtpServiceId)?.name ??
                    "Select SMTP service"}
                </span>
              </SelectTrigger>
              <SelectContent>
                {services
                  .filter((service): service is ApiAlertSMTPServiceResponse & { id: string } =>
                    Boolean(service.id),
                  )
                  .map((service) => (
                    <SelectItem key={service.id} value={service.id}>
                      {service.name ?? service.host ?? "SMTP service"}
                    </SelectItem>
                  ))}
              </SelectContent>
            </Select>
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
          {services.length === 0 && (
            <div className="text-sm text-neutral-600">
              Create an SMTP service before adding email destinations.
            </div>
          )}
          {mutationError && <div className="text-sm text-red-700">{mutationError}</div>}
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={
              isPending ||
              services.length === 0 ||
              !name.trim() ||
              !emailTo.trim() ||
              !smtpServiceId ||
              events.length === 0
            }
          >
            {isPending
              ? isEditing
                ? "Saving..."
                : "Creating..."
              : isEditing
                ? "Save email destination"
                : "Create email destination"}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
);
