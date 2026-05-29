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
import { PageHeader } from "@/components/page-header";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { DATE_TIME_FORMAT, formatDate } from "@/lib/date-utils";
import {
  type ServiceDataLifecycleSettingsPayload,
  getGetDataLifecycleSettingsQueryKey,
  useGetDataLifecycleSettings,
  useRunDataLifecycleArchive,
  useRunDataLifecycleRollup,
  useUpdateDataLifecycleSettings,
} from "@/orion-sdk";
import { useQueryClient } from "@tanstack/react-query";
import { Archive, BarChart3 } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";

type SettingsFormState = {
  rawReportHotDays: string;
  archiveRawReports: boolean;
  archiveDir: string;
  archiveSchedule: string;
  rollupsEnabled: boolean;
  rollupRetentionDays: string;
};

const defaultFormState: SettingsFormState = {
  rawReportHotDays: "",
  archiveRawReports: false,
  archiveDir: "",
  archiveSchedule: "daily",
  rollupsEnabled: false,
  rollupRetentionDays: "",
};

const asNumber = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
};

const formatCount = (value: number | null | undefined, label: string) =>
  `${value ?? 0} ${(value ?? 0) === 1 ? label : `${label}s`}`;

const formatArchiveCutoff = (hotDays: number | null | undefined) => {
  const days = hotDays ?? 90;
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - days);
  return `${formatDate(cutoff, DATE_TIME_FORMAT)} (${days} hot ${days === 1 ? "day" : "days"})`;
};

const maintenanceErrorMessage = (action: "archive" | "rollup", error: unknown) => {
  const status =
    typeof (error as { status?: unknown })?.status === "number"
      ? (error as { status: number }).status
      : undefined;
  if (status && status < 500) {
    return action === "archive"
      ? "Archive could not start. Check the saved archive settings and try again."
      : "Rollup could not start. Check the request and try again.";
  }
  return action === "archive"
    ? "Archive could not complete. Verify Core can write to the saved archive directory, then try again."
    : "Rollup could not complete. Try again after Core is healthy.";
};

const archiveHistoryErrorMessage = (error: string | null | undefined) =>
  error
    ? "Previous archive failed. Verify Core can write to the saved archive directory."
    : undefined;

const Field = ({
  label,
  children,
  description,
}: {
  label: string;
  children: ReactNode;
  description?: string;
}) => (
  <label className="block space-y-1">
    <span className="text-sm font-medium">{label}</span>
    {children}
    {description && <span className="block text-sm text-neutral-600">{description}</span>}
  </label>
);

export const SettingsPage = () => {
  const queryClient = useQueryClient();
  const refreshSettings = () => {
    void queryClient.invalidateQueries({ queryKey: getGetDataLifecycleSettingsQueryKey() });
  };
  const [formState, setFormState] = useState(defaultFormState);
  const [archiveConfirmOpen, setArchiveConfirmOpen] = useState(false);
  const [lastArchiveCompletedAt, setLastArchiveCompletedAt] = useState<string | null>(null);
  const [lastRollupCompletedAt, setLastRollupCompletedAt] = useState<string | null>(null);
  const settingsResponse = useGetDataLifecycleSettings();
  const updateSettings = useUpdateDataLifecycleSettings({
    mutation: {
      onSuccess: refreshSettings,
    },
  });
  const archiveRun = useRunDataLifecycleArchive({
    mutation: {
      onSuccess: () => {
        setLastArchiveCompletedAt(new Date().toISOString());
        refreshSettings();
      },
    },
  });
  const rollupRun = useRunDataLifecycleRollup({
    mutation: {
      onSuccess: () => {
        setLastRollupCompletedAt(new Date().toISOString());
        refreshSettings();
      },
    },
  });

  const settings = settingsResponse.data?.settings;

  useEffect(() => {
    if (!settings) return;
    setFormState({
      rawReportHotDays: String(settings.raw_report_hot_days ?? ""),
      archiveRawReports: Boolean(settings.archive_raw_reports),
      archiveDir: settings.archive_dir ?? "",
      archiveSchedule: settings.archive_schedule ?? "daily",
      rollupsEnabled: Boolean(settings.rollups_enabled),
      rollupRetentionDays: String(settings.rollup_retention_days ?? ""),
    });
  }, [settings]);

  const updateField = <TKey extends keyof SettingsFormState>(
    key: TKey,
    value: SettingsFormState[TKey],
  ) => {
    setFormState((current) => ({ ...current, [key]: value }));
  };

  const saveSettings = () => {
    const payload: ServiceDataLifecycleSettingsPayload = {
      archive_dir: formState.archiveDir.trim() || undefined,
      archive_raw_reports: formState.archiveRawReports,
      archive_schedule: formState.archiveSchedule,
      raw_report_hot_days:
        asNumber(formState.rawReportHotDays) ?? settings?.raw_report_hot_days ?? 90,
      rollup_retention_days: asNumber(formState.rollupRetentionDays),
      rollups_enabled: formState.rollupsEnabled,
    };
    updateSettings.mutate({ data: payload });
  };

  const runRollup = () => {
    if (rollupRun.isPending || archiveRun.isPending) return;
    rollupRun.mutate({ data: undefined });
  };

  const runArchive = () => {
    if (archiveRun.isPending || rollupRun.isPending) return;
    archiveRun.mutate(undefined, { onSuccess: () => setArchiveConfirmOpen(false) });
  };

  const lifecycleActionPending = rollupRun.isPending || archiveRun.isPending;
  const savedArchiveDir = settings?.archive_dir?.trim() || "No archive directory saved";
  const archiveCutoff = formatArchiveCutoff(settings?.raw_report_hot_days);
  const archiveResult = archiveRun.data?.result;
  const rollupResult = rollupRun.data?.result;
  const lastArchiveErrorMessage = archiveHistoryErrorMessage(settings?.last_archive_error);

  return (
    <div className="space-y-7">
      <PageHeader title="Settings" description="Data retention and manual maintenance." />

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Retention</h2>

        {settingsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading data lifecycle settings...</div>
        )}
        {settingsResponse.error && (
          <div className="text-sm">Unable to load data lifecycle settings.</div>
        )}

        <div className="grid gap-3 sm:grid-cols-3">
          <Field label="Raw report days" description="Recent report history kept in Core.">
            <Input
              type="number"
              min="1"
              value={formState.rawReportHotDays}
              onChange={(event) => updateField("rawReportHotDays", event.target.value)}
            />
          </Field>
          <Field label="Rollup days" description="Daily uptime history to retain.">
            <Input
              type="number"
              min="1"
              value={formState.rollupRetentionDays}
              onChange={(event) => updateField("rollupRetentionDays", event.target.value)}
            />
          </Field>
          <Field label="Archive directory" description="Local path for archived reports.">
            <Input
              value={formState.archiveDir}
              onChange={(event) => updateField("archiveDir", event.target.value)}
              placeholder="./data/archive"
            />
          </Field>
          <Field label="Archive schedule" description="Automatic archive cadence.">
            <Select
              value={formState.archiveSchedule}
              onValueChange={(value) => updateField("archiveSchedule", value)}
            >
              <SelectTrigger>
                <SelectValue placeholder="Archive schedule" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="daily">Daily</SelectItem>
                <SelectItem value="manual">Manual only</SelectItem>
              </SelectContent>
            </Select>
          </Field>
        </div>

        <div className="flex flex-wrap gap-4">
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={formState.archiveRawReports}
              onCheckedChange={(checked) => updateField("archiveRawReports", checked === true)}
            />
            Archive raw reports automatically
          </label>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={formState.rollupsEnabled}
              onCheckedChange={(checked) => updateField("rollupsEnabled", checked === true)}
            />
            Enable rollups
          </label>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={saveSettings} disabled={updateSettings.isPending || !settings}>
            {updateSettings.isPending ? "Saving..." : "Save settings"}
          </Button>
          {updateSettings.isError && <span className="text-sm">Unable to save settings.</span>}
          {updateSettings.isSuccess && (
            <span className="text-sm text-neutral-600">Settings saved.</span>
          )}
        </div>
      </section>

      <section className="space-y-3">
        <h2 className="text-sm font-medium">Manual Maintenance</h2>

        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={runRollup} disabled={lifecycleActionPending || !settings}>
            <BarChart3 />
            {rollupRun.isPending ? "Running..." : "Run rollup"}
          </Button>
          <Button
            variant="outline"
            onClick={() => setArchiveConfirmOpen(true)}
            disabled={lifecycleActionPending || !settings}
          >
            <Archive />
            {archiveRun.isPending ? "Running..." : "Run archive"}
          </Button>
        </div>
        <div className="grid gap-3 text-sm sm:grid-cols-2">
          <div className="border border-neutral-200 p-3">
            <div className="font-medium">Rollup result</div>
            {rollupRun.isPending && <div className="text-neutral-600">Running rollup...</div>}
            {rollupResult && (
              <div className="mt-2 space-y-1 text-neutral-600">
                <div>
                  {rollupResult.skipped_today
                    ? "Skipped because the current day is still in progress."
                    : `Processed ${formatCount(rollupResult.report_count, "report")} across ${formatCount(rollupResult.monitor_days, "monitor day")}.`}
                </div>
                <div>Rollup date: {rollupResult.date ?? "Not reported"}</div>
                <div>Completed: {formatDate(lastRollupCompletedAt, DATE_TIME_FORMAT)}</div>
              </div>
            )}
            {rollupRun.isError && (
              <div className="mt-2 text-neutral-900">
                {maintenanceErrorMessage("rollup", rollupRun.error)}
              </div>
            )}
            {!rollupRun.isPending && !rollupResult && !rollupRun.isError && (
              <div className="mt-2 text-neutral-600">No manual rollup has run in this session.</div>
            )}
          </div>
          <div className="border border-neutral-200 p-3">
            <div className="font-medium">Archive result</div>
            {archiveRun.isPending && <div className="text-neutral-600">Running archive...</div>}
            {archiveResult && (
              <div className="mt-2 space-y-1 text-neutral-600">
                {archiveResult.skipped_because_disabled && (
                  <div>Skipped because raw report archiving is disabled.</div>
                )}
                {archiveResult.skipped_because_no_reports && (
                  <div>Skipped because no reports were older than the cutoff.</div>
                )}
                {!archiveResult.skipped_because_disabled &&
                  !archiveResult.skipped_because_no_reports && (
                    <div>
                      Archived{" "}
                      {formatCount(
                        (archiveResult.agent_reports_archived ?? 0) +
                          (archiveResult.monitor_reports_archived ?? 0),
                        "report",
                      )}
                      .
                    </div>
                  )}
                <div>
                  Server reports: {archiveResult.agent_reports_archived ?? 0}; monitor reports:{" "}
                  {archiveResult.monitor_reports_archived ?? 0}
                </div>
                <div>Cutoff: {formatDate(archiveResult.cutoff, DATE_TIME_FORMAT)}</div>
                <div>Destination: {archiveResult.archive_path || savedArchiveDir}</div>
                <div>Completed: {formatDate(lastArchiveCompletedAt, DATE_TIME_FORMAT)}</div>
              </div>
            )}
            {archiveRun.isError && (
              <div className="mt-2 text-neutral-900">
                {maintenanceErrorMessage("archive", archiveRun.error)}
              </div>
            )}
            {!archiveRun.isPending && !archiveResult && !archiveRun.isError && (
              <div className="mt-2 text-neutral-600">
                No manual archive has run in this session.
              </div>
            )}
          </div>
        </div>
        {settings && (
          <div className="grid gap-3 text-sm sm:grid-cols-2">
            <div className="bg-neutral-100 p-3">
              <div className="text-neutral-600">Last rollup</div>
              <div className="font-medium">
                {formatDate(settings.last_rollup_run_at, DATE_TIME_FORMAT)}
              </div>
            </div>
            <div className="bg-neutral-100 p-3">
              <div className="text-neutral-600">Last archive</div>
              <div className="font-medium">
                {formatDate(settings.last_archive_run_at, DATE_TIME_FORMAT)}
              </div>
              {settings.last_archive_status && (
                <div className="text-neutral-600">{settings.last_archive_status}</div>
              )}
              {lastArchiveErrorMessage && <div>{lastArchiveErrorMessage}</div>}
            </div>
          </div>
        )}
      </section>

      <Dialog open={archiveConfirmOpen} onOpenChange={setArchiveConfirmOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Confirm manual archive</DialogTitle>
            <DialogDescription>
              This runs against saved lifecycle settings and can move raw reports out of the hot
              Core database.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-3 text-sm">
            <div className="border border-neutral-200 p-3">
              <div className="text-neutral-600">Cutoff</div>
              <div className="font-medium">{archiveCutoff}</div>
            </div>
            <div className="border border-neutral-200 p-3">
              <div className="text-neutral-600">Destination</div>
              <div className="break-all font-medium">{savedArchiveDir}</div>
            </div>
            <div className="border border-neutral-200 p-3">
              <div className="text-neutral-600">Hot database impact</div>
              <div className="font-medium">
                Matching raw Server and Monitor reports are copied to the archive database, then
                removed from the hot Core database.
              </div>
            </div>
            {settings?.archive_raw_reports === false && (
              <div className="border border-neutral-200 p-3 text-neutral-600">
                Raw report archiving is disabled, so Core will record this manual run as skipped.
              </div>
            )}
          </div>

          <DialogFooter showCloseButton>
            <Button onClick={runArchive} disabled={lifecycleActionPending || !settings}>
              <Archive />
              {archiveRun.isPending ? "Archiving..." : "Confirm archive"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};
