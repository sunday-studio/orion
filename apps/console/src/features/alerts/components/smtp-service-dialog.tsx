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

type SMTPServiceDialogProps = {
  enabled: boolean;
  fromEmail: string;
  host: string;
  isEditing: boolean;
  isOpen: boolean;
  isPending: boolean;
  mutationError: string;
  name: string;
  password: string;
  port: string;
  username: string;
  onClose: () => void;
  onEnabledChange: (enabled: boolean) => void;
  onFromEmailChange: (fromEmail: string) => void;
  onHostChange: (host: string) => void;
  onNameChange: (name: string) => void;
  onOpenChange: (open: boolean) => void;
  onPasswordChange: (password: string) => void;
  onPortChange: (port: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onUsernameChange: (username: string) => void;
};

export const SMTPServiceDialog = ({
  enabled,
  fromEmail,
  host,
  isEditing,
  isOpen,
  isPending,
  mutationError,
  name,
  password,
  port,
  username,
  onClose,
  onEnabledChange,
  onFromEmailChange,
  onHostChange,
  onNameChange,
  onOpenChange,
  onPasswordChange,
  onPortChange,
  onSubmit,
  onUsernameChange,
}: SMTPServiceDialogProps) => (
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
    <DialogContent className="sm:max-w-lg">
      <form className="space-y-5" onSubmit={onSubmit}>
        <DialogHeader>
          <DialogTitle>{isEditing ? "Edit SMTP service" : "New SMTP service"}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? "Update reusable SMTP connection settings. Leave password blank to keep the stored secret."
              : "Add reusable SMTP settings for email alert destinations."}
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 sm:grid-cols-2">
          <label className="block space-y-1.5 text-sm sm:col-span-2">
            <span>Name</span>
            <Input
              value={name}
              onChange={(event) => onNameChange(event.target.value)}
              placeholder="primary-smtp"
              required
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>Host</span>
            <Input
              value={host}
              onChange={(event) => onHostChange(event.target.value)}
              placeholder="smtp.example.com"
              required
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>Port</span>
            <Input
              value={port}
              onChange={(event) => onPortChange(event.target.value)}
              inputMode="numeric"
              min={1}
              max={65535}
              placeholder="587"
              required
              type="number"
            />
          </label>
          <label className="block space-y-1.5 text-sm sm:col-span-2">
            <span>From email</span>
            <Input
              value={fromEmail}
              onChange={(event) => onFromEmailChange(event.target.value)}
              placeholder="alerts@example.com"
              required
              type="email"
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>Username</span>
            <Input
              value={username}
              onChange={(event) => onUsernameChange(event.target.value)}
              autoComplete="username"
              placeholder="smtp-user"
            />
          </label>
          <label className="block space-y-1.5 text-sm">
            <span>Password</span>
            <Input
              value={password}
              onChange={(event) => onPasswordChange(event.target.value)}
              autoComplete="new-password"
              placeholder={isEditing ? "Keep existing" : "SMTP password"}
              type="password"
            />
          </label>
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <Checkbox
              checked={enabled}
              onCheckedChange={(value) => onEnabledChange(value === true)}
            />
            <span>Enabled</span>
          </label>
          {mutationError && (
            <div className="text-sm text-red-700 sm:col-span-2">{mutationError}</div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={
              isPending ||
              !name.trim() ||
              !host.trim() ||
              !fromEmail.trim() ||
              !port.trim() ||
              Number(port) < 1 ||
              Number(port) > 65535
            }
          >
            {isPending
              ? isEditing
                ? "Saving..."
                : "Creating..."
              : isEditing
                ? "Save SMTP service"
                : "Create SMTP service"}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
);
