import { Button, RaisedCard, Select, Switch, useToast } from "@/design-system";
import { formatDate } from "@/utils/date";
import { SettingRow } from "../settings/components-ui";
import { type ApiUpdateSettings, useChangeUpdateSettings, useCheckForUpdates } from "./use-updates";

const CHANNELS = [
  { value: "stable", label: "Stable" },
  { value: "prerelease", label: "Stable and pre-release" },
];

/** Whether Core looks for releases, on which channel, and a way to look now. */
export const UpdatesSettings = ({ updates }: { updates: ApiUpdateSettings }) => {
  const toast = useToast();
  const change = useChangeUpdateSettings();
  const check = useCheckForUpdates();
  const fail = (failure: unknown) =>
    toast.add({ title: (failure as Error).message || "Unable to save" });
  return (
    <RaisedCard>
      <SettingRow
        label="Check for updates"
        detail="Core asks GitHub for the latest release once a day"
      >
        <Switch
          aria-label="Check for updates"
          checked={updates.auto_check ?? true}
          disabled={change.isPending}
          onCheckedChange={(next) =>
            change.mutate({ data: { auto_check: next } }, { onError: fail })
          }
        />
      </SettingRow>
      <SettingRow
        label="Channel"
        detail={updates.channel === "prerelease" ? "Pre-releases included" : "Stable releases only"}
      >
        <Select
          aria-label="Channel"
          options={CHANNELS}
          value={updates.channel ?? "stable"}
          disabled={change.isPending}
          onValueChange={(next) =>
            change.mutate(
              { data: { channel: next } },
              { onSuccess: () => check.mutate(), onError: fail },
            )
          }
        />
      </SettingRow>
      <SettingRow
        label="Last checked"
        detail={
          updates.state === "error"
            ? (updates.error ?? "Could not check")
            : updates.checked_at
              ? formatDate(updates.checked_at, "MMM d, HH:mm")
              : "Never"
        }
      >
        <Button
          size="sm"
          variant="secondary"
          disabled={check.isPending}
          onClick={() => check.mutate()}
        >
          {check.isPending ? "Checking..." : "Check now"}
        </Button>
      </SettingRow>
    </RaisedCard>
  );
};
