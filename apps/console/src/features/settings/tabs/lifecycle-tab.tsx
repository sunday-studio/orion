import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
  Button,
  Callout,
  ConfirmDialog,
  FieldPanel,
  Input,
  Select,
  Skeleton,
  useToast,
} from "@/design-system";
import {
  type ServiceDataLifecycleSettingsPayload,
  getGetDataLifecycleSettingsQueryKey,
  useGetDataLifecycleSettings,
  useRunDataLifecycleArchive,
  useRunDataLifecycleRollup,
  useUpdateDataLifecycleSettings,
} from "@/orion-sdk";
import { formatDate } from "@/utils/date";
import { Labeled } from "../components-ui";
import { archiveCount, asNumber } from "../settings.domain";

const SCHEDULES = [
  { value: "daily", label: "Daily at 03:00" },
  { value: "manual", label: "Manual only" },
];

export const LifecycleTab = () => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const query = useGetDataLifecycleSettings();
  const settings = query.data?.settings;
  const [raw, setRaw] = useState("");
  const [rollup, setRollup] = useState("");
  const [dir, setDir] = useState("");
  const [schedule, setSchedule] = useState("daily");
  const [confirming, setConfirming] = useState(false);
  const [ran, setRan] = useState<string>();

  useEffect(() => {
    if (!settings) return;
    setRaw(String(settings.raw_report_hot_days ?? ""));
    setRollup(String(settings.rollup_retention_days ?? ""));
    setDir(settings.archive_dir ?? "");
    setSchedule(settings.archive_schedule === "manual" ? "manual" : "daily");
  }, [settings]);

  const refresh = () =>
    void queryClient.invalidateQueries({ queryKey: getGetDataLifecycleSettingsQueryKey() });
  const fail = (failure: unknown) => toast.add({ title: (failure as Error).message || "Failed" });
  const save = useUpdateDataLifecycleSettings({
    mutation: {
      onSuccess: () => {
        refresh();
        toast.add({ title: "Settings saved" });
      },
      onError: fail,
    },
  });
  const rollupRun = useRunDataLifecycleRollup({
    mutation: {
      onSuccess: (result) => {
        refresh();
        setRan(`Rolled up ${result.result?.report_count ?? 0} reports.`);
      },
      onError: fail,
    },
  });
  const archiveRun = useRunDataLifecycleArchive({
    mutation: {
      onSuccess: (result) => {
        refresh();
        setConfirming(false);
        setRan(`Archived ${archiveCount(result.result ?? {})} reports.`);
      },
      onError: fail,
    },
  });

  const rawDays = asNumber(raw);
  const errors = {
    raw: rawDays === undefined || rawDays < 1 ? "Enter at least 1 day." : undefined,
    rollup:
      rollup.trim() !== "" && (asNumber(rollup) ?? 0) < 1 ? "Enter at least 1 day." : undefined,
  };
  const submit = () => {
    if (errors.raw || errors.rollup) return;
    const payload: ServiceDataLifecycleSettingsPayload = {
      raw_report_hot_days: rawDays,
      rollup_retention_days: asNumber(rollup),
      rollups_enabled: rollup.trim() !== "",
      archive_dir: dir.trim() || undefined,
      archive_raw_reports: schedule !== "manual",
      archive_schedule: schedule,
    };
    save.mutate({ data: payload });
  };

  const dirty =
    raw !== String(settings?.raw_report_hot_days ?? "") ||
    rollup !== String(settings?.rollup_retention_days ?? "") ||
    dir !== (settings?.archive_dir ?? "") ||
    schedule !== (settings?.archive_schedule === "manual" ? "manual" : "daily");

  if (query.isPending) return <Skeleton className="h-64 w-full rounded-xl" />;
  if (query.isError || !settings)
    return <Callout tone="down">Unable to load data lifecycle settings.</Callout>;

  return (
    <div className="flex flex-col gap-3.5">
      <FieldPanel title="Retention" aside="How long Core keeps history online">
        <div className="grid grid-cols-2 gap-4">
          <Labeled label="Raw report days" error={errors.raw}>
            <Input inputMode="numeric" value={raw} onChange={(e) => setRaw(e.target.value)} />
          </Labeled>
          <Labeled label="Rollup days" error={errors.rollup}>
            <Input inputMode="numeric" value={rollup} onChange={(e) => setRollup(e.target.value)} />
          </Labeled>
        </div>
      </FieldPanel>
      <FieldPanel title="Archive" aside="Where eligible raw reports go">
        <div className="grid grid-cols-2 gap-4">
          <Labeled label="Archive directory">
            <Input value={dir} onChange={(e) => setDir(e.target.value)} />
          </Labeled>
          <Labeled label="Schedule">
            <Select
              aria-label="Schedule"
              className="w-full"
              options={SCHEDULES}
              value={schedule}
              onValueChange={setSchedule}
            />
          </Labeled>
        </div>
      </FieldPanel>
      <FieldPanel title="Run now" aside="Without changing the schedule">
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="secondary"
            disabled={rollupRun.isPending}
            onClick={() => rollupRun.mutate({})}
          >
            Run rollup
          </Button>
          <Button
            size="sm"
            variant="secondary"
            disabled={archiveRun.isPending}
            onClick={() => setConfirming(true)}
          >
            Run archive
          </Button>
        </div>
        <p className="mt-2.5 text-[13px] text-quiet">
          Last rollup {formatDate(settings.last_rollup_run_at, "MMM d, HH:mm", "never")}. Last
          archive {formatDate(settings.last_archive_run_at, "MMM d, HH:mm", "never")}
          {settings.last_archive_error ? ` (${settings.last_archive_error})` : ""}.
          {ran ? ` ${ran}` : ""}
        </p>
      </FieldPanel>
      <div className="flex justify-end">
        <Button
          disabled={save.isPending || !dirty || Boolean(errors.raw || errors.rollup)}
          onClick={submit}
        >
          Save settings
        </Button>
      </div>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Run archive now"
        description={`Raw reports older than ${settings.raw_report_hot_days} days move to ${settings.archive_dir || "the archive directory"}.`}
        confirmLabel="Run archive"
        tone="default"
        onConfirm={() => archiveRun.mutate()}
      />
    </div>
  );
};
