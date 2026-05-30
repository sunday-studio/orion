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
import { ArchiveIcon, RotateCwIcon } from "lucide-react";
import { type ReactNode, useEffect, useRef, useState } from "react";

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

const getArchiveCutoff = (hotDays?: number) => {
  if (!hotDays) return null;
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - hotDays);
  return cutoff.toISOString();
};

const getErrorMessage = (error: unknown) => {
  if (error instanceof Error && error.message) return error.message;
  return "The maintenance action could not be completed.";
};

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
  const settingsResponse = useGetDataLifecycleSettings();
  const updateSettings = useUpdateDataLifecycleSettings({
    mutation: {
      onSuccess: refreshSettings,
    },
  });
  const archiveRun = useRunDataLifecycleArchive({ mutation: { onSuccess: refreshSettings } });
  const rollupRun = useRunDataLifecycleRollup({ mutation: { onSuccess: refreshSettings } });

  const settings = settingsResponse.data?.settings;
  const [formState, setFormState] = useState(defaultFormState);
  const archiveScheduleLabel = formState.archiveSchedule === "manual" ? "Manual only" : "Daily";
  const [archiveDialogOpen, setArchiveDialogOpen] = useState(false);
  const maintenanceActionInFlight = useRef(false);

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

  const maintenancePending = rollupRun.isPending || archiveRun.isPending;

  const runRollup = () => {
    if (maintenanceActionInFlight.current) return;
    maintenanceActionInFlight.current = true;
    rollupRun.mutate(
      { data: undefined },
      {
        onSettled: () => {
          maintenanceActionInFlight.current = false;
        },
      },
    );
  };

  const runArchive = () => {
    if (maintenanceActionInFlight.current) return;
    maintenanceActionInFlight.current = true;
    archiveRun.mutate(undefined, {
      onSettled: () => {
        maintenanceActionInFlight.current = false;
        setArchiveDialogOpen(false);
      },
    });
  };

  const archiveCutoff = getArchiveCutoff(settings?.raw_report_hot_days);
  const archiveResult = archiveRun.data?.result;
  const archiveTotal =
    (archiveResult?.agent_reports_archived ?? 0) + (archiveResult?.monitor_reports_archived ?? 0);
  const rollupResult = rollupRun.data?.result;

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
              <SelectTrigger aria-label="Archive schedule">
                <SelectValue placeholder="Archive schedule">{archiveScheduleLabel}</SelectValue>
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
          <Button onClick={runRollup} disabled={maintenancePending}>
            <RotateCwIcon />
            {rollupRun.isPending ? "Running rollup..." : "Run rollup"}
          </Button>
          <Button
            variant="outline"
            onClick={() => setArchiveDialogOpen(true)}
            disabled={maintenancePending || !settings}
          >
            <ArchiveIcon />
            Run archive
          </Button>
        </div>

        <Dialog open={archiveDialogOpen} onOpenChange={setArchiveDialogOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Archive raw reports</DialogTitle>
              <DialogDescription>
                Confirm before old raw reports move out of the hot Core database.
              </DialogDescription>
            </DialogHeader>
            <dl className="grid gap-3 text-sm">
              <div className="bg-neutral-100 p-3">
                <dt className="text-neutral-600">Reports older than</dt>
                <dd className="font-medium">
                  {formatDate(archiveCutoff, DATE_TIME_FORMAT)} ({settings?.raw_report_hot_days}{" "}
                  days)
                </dd>
              </div>
              <div className="bg-neutral-100 p-3">
                <dt className="text-neutral-600">Archive destination</dt>
                <dd className="break-all font-medium">{settings?.archive_dir}</dd>
              </div>
              {!settings?.archive_raw_reports && (
                <div className="border border-neutral-300 p-3 text-neutral-700">
                  Raw report archiving is disabled, so this run will record a skipped archive.
                </div>
              )}
            </dl>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setArchiveDialogOpen(false)}
                disabled={archiveRun.isPending}
              >
                Cancel
              </Button>
              <Button onClick={runArchive} disabled={maintenancePending || !settings}>
                <ArchiveIcon />
                {archiveRun.isPending ? "Running archive..." : "Run archive"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <div className="grid gap-3 text-sm lg:grid-cols-2">
          {rollupResult && (
            <div className="border border-neutral-200 p-3">
              <div className="font-medium">
                Rolled up {rollupResult.report_count ?? 0} reports for {rollupResult.date}.
              </div>
              <div className="mt-1 text-neutral-600">
                {rollupResult.monitor_days ?? 0} monitor days updated.
              </div>
              {rollupResult.report_count === 0 && (
                <div className="mt-1 text-neutral-600">
                  No monitor reports matched this rollup day.
                </div>
              )}
              {rollupResult.skipped_today && (
                <div className="mt-1 text-neutral-600">
                  Skipped because rollups only run after a day is complete.
                </div>
              )}
            </div>
          )}
          {archiveResult && (
            <div className="border border-neutral-200 p-3">
              <div className="font-medium">Archived {archiveTotal} reports.</div>
              <div className="mt-1 text-neutral-600">
                {archiveResult.agent_reports_archived ?? 0} server reports and{" "}
                {archiveResult.monitor_reports_archived ?? 0} monitor reports moved.
              </div>
              {archiveResult.cutoff && (
                <div className="mt-1 text-neutral-600">
                  Cutoff {formatDate(archiveResult.cutoff, DATE_TIME_FORMAT)}.
                </div>
              )}
              {archiveResult.archive_path && (
                <div className="mt-1 break-all text-neutral-600">
                  Destination {archiveResult.archive_path}.
                </div>
              )}
              {archiveResult.skipped_because_disabled && (
                <div className="mt-1 text-neutral-600">
                  Skipped because raw report archiving is disabled.
                </div>
              )}
              {archiveResult.skipped_because_no_reports && (
                <div className="mt-1 text-neutral-600">No raw reports matched the cutoff.</div>
              )}
            </div>
          )}
          {rollupRun.isError && (
            <div className="border border-neutral-300 p-3">
              <div className="font-medium">Rollup failed</div>
              <div className="mt-1 text-neutral-600">{getErrorMessage(rollupRun.error)}</div>
            </div>
          )}
          {archiveRun.isError && (
            <div className="border border-neutral-300 p-3">
              <div className="font-medium">Archive failed</div>
              <div className="mt-1 text-neutral-600">{getErrorMessage(archiveRun.error)}</div>
            </div>
          )}
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
              {settings.last_archive_error && <div>{settings.last_archive_error}</div>}
            </div>
          </div>
        )}
      </section>
    </div>
  );
};
