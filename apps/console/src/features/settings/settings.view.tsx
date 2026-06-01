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
import { cn } from "@/lib/utils";
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
import { ArchiveIcon, History, RotateCwIcon, Save } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

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

type SettingsFieldKey = keyof SettingsFormState | "archiveRollupCompatibility";

type SettingsFormErrors = Partial<Record<SettingsFieldKey, string>>;

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

const isPositiveInteger = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return false;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed >= 1;
};

const optionalPositiveInteger = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return true;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed >= 1;
};

const validateSettingsForm = (formState: SettingsFormState): SettingsFormErrors => {
  const errors: SettingsFormErrors = {};
  if (!isPositiveInteger(formState.rawReportHotDays)) {
    errors.rawReportHotDays = "Enter at least 1 day.";
  }
  if (!optionalPositiveInteger(formState.rollupRetentionDays)) {
    errors.rollupRetentionDays = "Enter at least 1 day, or leave it blank.";
  }
  if (formState.archiveRawReports && formState.archiveDir.trim() === "") {
    errors.archiveDir = "Archive directory is required when raw report archiving is enabled.";
  }
  if (formState.archiveRawReports && !formState.rollupsEnabled) {
    errors.archiveRollupCompatibility = "Enable rollups before archiving raw reports.";
  }
  return errors;
};

const safeInlineMessage = (value: unknown, fallback = "Unable to complete action.") => {
  if (typeof value !== "string") return fallback;
  const compact = value.replace(/\s+/g, " ").trim();
  if (compact === "") return fallback;
  return compact.length > 180 ? `${compact.slice(0, 177)}...` : compact;
};

const archiveCount = (result: {
  agent_reports_archived?: number;
  monitor_reports_archived?: number;
}) => (result.agent_reports_archived ?? 0) + (result.monitor_reports_archived ?? 0);

const Field = ({
  label,
  children,
  description,
  error,
}: {
  label: string;
  children: ReactNode;
  description?: string;
  error?: string;
}) => (
  <label className="block space-y-1">
    <span className="text-sm font-medium">{label}</span>
    {children}
    {description && <span className="block text-sm text-neutral-600">{description}</span>}
    {error && <span className="block text-sm text-red-700">{error}</span>}
  </label>
);

const Section = ({
  title,
  description,
  children,
}: {
  title: string;
  description?: string;
  children: ReactNode;
}) => (
  <section className="space-y-3 border-t border-neutral-200 pt-5">
    <div className="space-y-1">
      <h2 className="text-sm font-medium">{title}</h2>
      {description && <p className="text-sm text-neutral-600">{description}</p>}
    </div>
    {children}
  </section>
);

const ActivityItem = ({
  label,
  value,
  detail,
  tone = "neutral",
}: {
  label: string;
  value: string;
  detail?: ReactNode;
  tone?: "neutral" | "success" | "error" | "pending";
}) => (
  <div
    className={cn(
      "space-y-1 border-l-2 bg-neutral-50 px-3 py-2 text-sm",
      tone === "success" && "border-emerald-500",
      tone === "error" && "border-red-500",
      tone === "pending" && "border-amber-500",
      tone === "neutral" && "border-neutral-300",
    )}
  >
    <div className="text-neutral-600">{label}</div>
    <div className="font-medium">{value}</div>
    {detail && <div className="text-neutral-600">{detail}</div>}
  </div>
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
  const [touchedFields, setTouchedFields] = useState<Partial<Record<SettingsFieldKey, boolean>>>(
    {},
  );
  const [showAllErrors, setShowAllErrors] = useState(false);

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
    setTouchedFields({});
    setShowAllErrors(false);
  }, [settings]);

  const updateField = <TKey extends keyof SettingsFormState>(
    key: TKey,
    value: SettingsFormState[TKey],
  ) => {
    setFormState((current) => ({ ...current, [key]: value }));
    setTouchedFields((current) => ({ ...current, [key]: true }));
  };

  const formErrors = useMemo(() => validateSettingsForm(formState), [formState]);
  const hasFormErrors = Object.keys(formErrors).length > 0;
  const fieldError = (key: SettingsFieldKey) =>
    showAllErrors || touchedFields[key] ? formErrors[key] : undefined;
  const archiveCompatibilityError =
    showAllErrors || touchedFields.archiveRawReports || touchedFields.rollupsEnabled
      ? formErrors.archiveRollupCompatibility
      : undefined;

  const saveSettings = () => {
    if (hasFormErrors) {
      setShowAllErrors(true);
      return;
    }
    const payload: ServiceDataLifecycleSettingsPayload = {
      archive_dir: formState.archiveDir.trim() || undefined,
      archive_raw_reports: formState.archiveRawReports,
      archive_schedule: formState.archiveSchedule,
      raw_report_hot_days: asNumber(formState.rawReportHotDays) ?? 90,
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
  const latestManualActivity = (() => {
    if (rollupRun.isPending) {
      return {
        value: "Rollup running",
        detail: "Core is computing daily uptime rollups.",
        tone: "pending" as const,
      };
    }
    if (archiveRun.isPending) {
      return {
        value: "Archive running",
        detail: "Core is moving eligible raw reports into archive storage.",
        tone: "pending" as const,
      };
    }
    if (rollupRun.data?.result) {
      const result = rollupRun.data.result;
      return {
        value: "Rollup completed",
        detail: `${result.report_count ?? 0} reports across ${result.monitor_days ?? 0} monitor days.`,
        tone: "success" as const,
      };
    }
    if (archiveRun.data?.result) {
      const result = archiveRun.data.result;
      return {
        value: "Archive completed",
        detail: `${archiveCount(result)} reports archived${
          result.archive_path ? ` to ${result.archive_path}` : ""
        }.`,
        tone: "success" as const,
      };
    }
    if (rollupRun.isError || archiveRun.isError) {
      return {
        value: "Manual action failed",
        detail: "Review Core logs if the action keeps failing.",
        tone: "error" as const,
      };
    }
    return {
      value: "No manual action this session",
      detail: "Run rollup or archive to see the latest action result here.",
      tone: "neutral" as const,
    };
  })();

  return (
    <div className="space-y-7">
      <PageHeader title="Settings" description="Data retention and manual maintenance." />

      {settingsResponse.isLoading && (
        <div className="text-sm text-neutral-600">Loading data lifecycle settings...</div>
      )}
      {settingsResponse.error && (
        <div className="text-sm text-red-700">Unable to load data lifecycle settings.</div>
      )}

      <Section
        title="Retention Policy"
        description="Control how much recent and rolled-up history Core keeps online."
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="Raw report days"
            description="Recent report history kept in Core."
            error={fieldError("rawReportHotDays")}
          >
            <Input
              aria-invalid={Boolean(fieldError("rawReportHotDays"))}
              className={fieldError("rawReportHotDays") ? "border-red-500" : undefined}
              type="number"
              min="1"
              step="1"
              value={formState.rawReportHotDays}
              onChange={(event) => updateField("rawReportHotDays", event.target.value)}
            />
          </Field>
          <Field
            label="Rollup days"
            description="Daily uptime history to retain after raw reports age out."
            error={fieldError("rollupRetentionDays")}
          >
            <Input
              aria-invalid={Boolean(fieldError("rollupRetentionDays"))}
              className={fieldError("rollupRetentionDays") ? "border-red-500" : undefined}
              type="number"
              min="1"
              step="1"
              value={formState.rollupRetentionDays}
              onChange={(event) => updateField("rollupRetentionDays", event.target.value)}
            />
          </Field>
        </div>
      </Section>

      <Section
        title="Archive Storage"
        description="Choose where eligible raw reports are archived and when Core runs that job."
      >
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="Archive directory"
            description="Local path for archived reports."
            error={fieldError("archiveDir")}
          >
            <Input
              aria-invalid={Boolean(fieldError("archiveDir"))}
              className={fieldError("archiveDir") ? "border-red-500" : undefined}
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
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={formState.archiveRawReports}
            onCheckedChange={(checked) => updateField("archiveRawReports", checked === true)}
          />
          Archive raw reports automatically
        </label>
      </Section>

      <Section
        title="Rollups"
        description="Keep compact uptime history available after raw reports are no longer hot."
      >
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={formState.rollupsEnabled}
              onCheckedChange={(checked) => updateField("rollupsEnabled", checked === true)}
            />
            Enable rollups
          </label>
          {archiveCompatibilityError && (
            <div className="text-sm text-red-700">{archiveCompatibilityError}</div>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={saveSettings} disabled={updateSettings.isPending || !settings}>
            <Save />
            {updateSettings.isPending ? "Saving..." : "Save settings"}
          </Button>
          {showAllErrors && hasFormErrors && (
            <span className="text-sm text-red-700">
              Fix the highlighted settings before saving.
            </span>
          )}
          {updateSettings.isError && <span className="text-sm">Unable to save settings.</span>}
          {updateSettings.isSuccess && (
            <span className="text-sm text-neutral-600">Settings saved.</span>
          )}
        </div>
      </Section>

      <Section
        title="Manual Maintenance"
        description="Run lifecycle jobs immediately without changing the saved schedule."
      >
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
      </Section>

      <Section title="Recent Activity">
        {settings ? (
          <div className="grid gap-3 sm:grid-cols-3">
            <ActivityItem
              label="Last rollup"
              value={formatDate(
                settings.last_rollup_run_at,
                DATE_TIME_FORMAT,
                "No rollup recorded",
              )}
              detail="Latest persisted rollup timestamp."
            />
            <ActivityItem
              label="Last archive"
              value={formatDate(
                settings.last_archive_run_at,
                DATE_TIME_FORMAT,
                "No archive recorded",
              )}
              detail={
                settings.last_archive_error
                  ? safeInlineMessage(settings.last_archive_error)
                  : (settings.last_archive_status ?? "Latest persisted archive status.")
              }
              tone={settings.last_archive_error ? "error" : "neutral"}
            />
            <ActivityItem
              label="This session"
              value={latestManualActivity.value}
              detail={latestManualActivity.detail}
              tone={latestManualActivity.tone}
            />
          </div>
        ) : (
          <div className="flex items-center gap-2 text-sm text-neutral-600">
            <History className="size-4" />
            Activity loads with lifecycle settings.
          </div>
        )}
      </Section>
    </div>
  );
};
