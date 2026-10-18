import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
} from "@/design-system";
import { getGetAlertChannelsQueryKey, useCreateAlertChannel } from "@/orion-sdk";
import { alertEventOptions, defaultAlertEvents, getMutationErrorMessage } from "./alert-constants";

type EmailChannelDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Called with the new channel id so the caller can open the recipients drawer. */
  onCreated: (channelId: string | undefined) => void;
};

export const EmailChannelDialog = ({ open, onOpenChange, onCreated }: EmailChannelDialogProps) => {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [events, setEvents] = useState<string[]>([...defaultAlertEvents]);
  const createChannel = useCreateAlertChannel({
    mutation: {
      onSuccess: (response) => {
        void queryClient.invalidateQueries({ queryKey: getGetAlertChannelsQueryKey() });
        setName("");
        setEvents([...defaultAlertEvents]);
        onOpenChange(false);
        onCreated(response.channel?.id);
      },
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <form
          className="space-y-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!name.trim() || events.length === 0) return;
            createChannel.mutate({
              data: { type: "email", name: name.trim(), enabled: true, subscribed_events: events },
            });
          }}
        >
          <DialogHeader>
            <DialogTitle>New email channel</DialogTitle>
            <DialogDescription>
              Send incident and recovery notifications by email. You add recipients next; each one
              confirms by email before receiving alerts.
            </DialogDescription>
          </DialogHeader>
          <label className="block space-y-1.5 text-[13px]">
            <span>Name</span>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="on-call-email"
              required
            />
          </label>
          <div className="space-y-2 text-[13px]">
            <div className="font-medium">Events</div>
            {alertEventOptions.map((option) => (
              <label key={option.value} className="flex items-center gap-2">
                <Checkbox
                  checked={events.includes(option.value)}
                  onCheckedChange={(checked) =>
                    setEvents(
                      checked === true
                        ? [...events, option.value]
                        : events.filter((value) => value !== option.value),
                    )
                  }
                />
                {option.label}
              </label>
            ))}
          </div>
          {createChannel.isError && (
            <p role="alert" className="text-xs text-destructive">
              {getMutationErrorMessage(createChannel.error, "Unable to create the email channel.")}
            </p>
          )}
          <DialogFooter>
            <Button
              type="submit"
              disabled={createChannel.isPending || !name.trim() || events.length === 0}
            >
              Create channel
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
};
