import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import { Archive, Database, History, Play, RotateCw, Save } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useState } from "react";

type SettingsFormState = {
  rawReportHotDays: string;
  archiveRawReports: boolean;
  archiveDir: string;
  archiveSchedule: string;
  rollupsEnabled: boolean;
  rollupRetentionDays: string;
};

type ValidationErrors = Partial<Record<keyof SettingsFormState | "archiveRollup", string>>;

const defaultFormState: SettingsFormState = {
  rawReportHotDays: "",
  archiveRawReports: false,
  archiveDir: "",
  archiveSchedule: "daily",
  rollupsEnabled: false,
  rollupRetentionDays: "",
};

const toPositiveInteger = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number(trimmed);
  return Number.isInteger(parsed) && parsed >= 1 ? parsed : undefined;
};

const validateSettings = (formState: SettingsFormState): ValidationErrors => {
  const errors: ValidationErrors = {};
  const rawReportDays = formState.rawReportHotDays.trim();
  const rollupRetentionDays = formState.rollupRetentionDays.trim();

  if (rawReportDays === "") {
    errors.rawReportHotDays = "Raw report days is required.";
  } else if (!toPositiveInteger(rawReportDays)) {
    errors.rawReportHotDays = "Use a whole number of 1 or more.";
  }

  if (rollupRetentionDays !== "" && !toPositiveInteger(rollupRetentionDays)) {
    errors.rollupRetentionDays = "Use a whole number of 1 or more, or leave blank.";
  }

  if (formState.archiveRawReports && formState.archiveDir.trim() === "") {
    errors.archiveDir = "Archive path is required when archiving is enabled.";
  }

  if (formState.archiveRawReports && !formState.rollupsEnabled) {
    errors.archiveRollup = "Archiving raw reports requires rollups to stay enabled.";
  }

  return errors;
};

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

const SettingsSection = ({
  title,
  icon,
  children,
}: {
  title: string;
  icon: ReactNode;
  children: ReactNode;
}) => (
  <section className="space-y-3 border-t border-neutral-200 pt-4">
    <h2 className="flex items-center gap-2 text-sm font-medium">
      <span className="text-neutral-500">{icon}</span>
      {title}
    </h2>
    {children}
  </section>
);

const ActivityItem = ({
  label,
  value,
  meta,
  tone = "neutral",
}: {
  label: string;
  value: string;
  meta?: ReactNode;
  tone?: "neutral" | "success" | "warning" | "error";
}) => {
  const toneClass = {
    error: "border-red-200 bg-red-50 text-red-900",
    neutral: "border-neutral-200 bg-neutral-50 text-neutral-900",
    success: "border-emerald-200 bg-emerald-50 text-emerald-900",
    warning: "border-amber-200 bg-amber-50 text-amber-900",
  }[tone];

  return (
    <div className={`space-y-1 border p-3 ${toneClass}`}>
      <div className="text-xs font-medium uppercase tracking-wide text-neutral-500">{label}</div>
      <div className="text-sm font-medium">{value}</div>
      {meta && <div className="text-sm text-neutral-700">{meta}</div>}
    </div>
  );
};

const archiveReportCount = (result?: {
  agent_reports_archived?: number;
  monitor_reports_archived?: number;
}) => (result?.agent_reports_archived ?? 0) + (result?.monitor_reports_archived ?? 0);

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
  const validationErrors = useMemo(() => validateSettings(formState), [formState]);
  const hasValidationErrors = Object.keys(validationErrors).length > 0;

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
    if (hasValidationErrors) return;
    const payload: ServiceDataLifecycleSettingsPayload = {
      archive_dir: formState.archiveDir.trim() || undefined,
      archive_raw_reports: formState.archiveRawReports,
      archive_schedule: formState.archiveSchedule,
      raw_report_hot_days: toPositiveInteger(formState.rawReportHotDays) ?? 90,
      rollup_retention_days: toPositiveInteger(formState.rollupRetentionDays),
      rollups_enabled: formState.rollupsEnabled,
    };
    updateSettings.mutate({ data: payload });
  };

  const runRollup = () => {
    rollupRun.mutate({ data: undefined });
  };

  const runArchive = () => {
    archiveRun.mutate(undefined);
  };

  const rollupResult = rollupRun.data?.result;
  const archiveResult = archiveRun.data?.result;
  const archivedReports = archiveReportCount(archiveResult);
  const lastArchiveStatus = archiveRun.isPending
    ? "running"
    : archiveRun.isError
      ? "failed"
      : archiveRun.isSuccess
        ? archiveResult?.skipped_because_disabled || archiveResult?.skipped_because_no_reports
          ? "skipped"
          : "completed"
        : settings?.last_archive_status || "not run";
  const archiveTone =
    lastArchiveStatus === "failed"
      ? "error"
      : lastArchiveStatus === "skipped"
        ? "warning"
        : lastArchiveStatus === "completed" || lastArchiveStatus === "success"
          ? "success"
          : "neutral";
  const rollupTone = rollupRun.isError
    ? "error"
    : rollupRun.isSuccess
      ? rollupResult?.skipped_today
        ? "warning"
        : "success"
      : "neutral";

  return (
    <div className="space-y-7">
      <PageHeader title="Settings" description="Data retention and manual maintenance." />

      {settingsResponse.isLoading && (
        <div className="text-sm text-neutral-600">Loading data lifecycle settings...</div>
      )}
      {settingsResponse.error && (
        <div className="text-sm">Unable to load data lifecycle settings.</div>
      )}

      <SettingsSection title="Retention Policy" icon={<Database className="size-4" />}>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="Raw report days"
            description="Recent report history kept in Core."
            error={validationErrors.rawReportHotDays}
          >
            <Input
              aria-invalid={Boolean(validationErrors.rawReportHotDays)}
              type="number"
              min="1"
              step="1"
              value={formState.rawReportHotDays}
              onChange={(event) => updateField("rawReportHotDays", event.target.value)}
            />
          </Field>
          <Field
            label="Rollup retention days"
            description="Daily uptime history to retain."
            error={validationErrors.rollupRetentionDays}
          >
            <Input
              aria-invalid={Boolean(validationErrors.rollupRetentionDays)}
              type="number"
              min="1"
              step="1"
              value={formState.rollupRetentionDays}
              onChange={(event) => updateField("rollupRetentionDays", event.target.value)}
              placeholder="Unlimited"
            />
          </Field>
        </div>
      </SettingsSection>

      <SettingsSection title="Archive Storage" icon={<Archive className="size-4" />}>
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_180px]">
          <Field
            label="Archive directory"
            description="Local path for archived reports."
            error={validationErrors.archiveDir}
          >
            <Input
              aria-invalid={Boolean(validationErrors.archiveDir)}
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
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={formState.archiveRawReports}
            onCheckedChange={(checked) => updateField("archiveRawReports", checked === true)}
          />
          Archive raw reports
        </label>
      </SettingsSection>

      <SettingsSection title="Rollups" icon={<RotateCw className="size-4" />}>
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={formState.rollupsEnabled}
              onCheckedChange={(checked) => updateField("rollupsEnabled", checked === true)}
            />
            Enable uptime rollups
          </label>
          {validationErrors.archiveRollup && (
            <div className="text-sm text-red-700">{validationErrors.archiveRollup}</div>
          )}
        </div>
      </SettingsSection>

      <div className="flex flex-wrap items-center gap-3 border-t border-neutral-200 pt-4">
        <Button
          onClick={saveSettings}
          disabled={updateSettings.isPending || !settings || hasValidationErrors}
        >
          <Save />
          {updateSettings.isPending ? "Saving..." : "Save settings"}
        </Button>
        {updateSettings.isError && <span className="text-sm">Unable to save settings.</span>}
        {updateSettings.isSuccess && (
          <span className="text-sm text-neutral-600">Settings saved.</span>
        )}
      </div>

      <SettingsSection title="Manual Maintenance" icon={<Play className="size-4" />}>
        <div className="flex flex-wrap items-center gap-3">
          <Button onClick={runRollup} disabled={rollupRun.isPending}>
            <RotateCw />
            {rollupRun.isPending ? "Running..." : "Run rollup"}
          </Button>
          <Button variant="outline" onClick={runArchive} disabled={archiveRun.isPending}>
            <Archive />
            {archiveRun.isPending ? "Running..." : "Run archive"}
          </Button>
          {rollupResult && (
            <span className="text-sm text-neutral-600">
              Rolled up {rollupResult.report_count ?? 0} reports.
            </span>
          )}
          {archiveResult && (
            <span className="text-sm text-neutral-600">Archived {archivedReports} reports.</span>
          )}
          {(rollupRun.isError || archiveRun.isError) && (
            <span className="text-sm">Unable to run maintenance.</span>
          )}
        </div>
      </SettingsSection>

      <SettingsSection title="Recent Activity" icon={<History className="size-4" />}>
        <div className="grid gap-3 text-sm lg:grid-cols-2">
          <ActivityItem
            label="Last rollup"
            value={formatDate(settings?.last_rollup_run_at, DATE_TIME_FORMAT)}
            tone={rollupTone}
            meta={
              rollupRun.isPending
                ? "Running now"
                : rollupRun.isError
                  ? "Last manual rollup failed."
                  : rollupResult
                    ? `${rollupResult.report_count ?? 0} reports, ${
                        rollupResult.monitor_days ?? 0
                      } monitor-days.`
                    : "No recent result count available."
            }
          />
          <ActivityItem
            label="Last archive"
            value={formatDate(settings?.last_archive_run_at, DATE_TIME_FORMAT)}
            tone={archiveTone}
            meta={
              <div className="space-y-1">
                <div>Status: {lastArchiveStatus}</div>
                {archiveResult && <div>{archivedReports} reports archived.</div>}
                {archiveResult?.archive_path && <div>Path: {archiveResult.archive_path}</div>}
                {settings?.last_archive_error && (
                  <div className="text-red-800">Error: {settings.last_archive_error}</div>
                )}
              </div>
            }
          />
        </div>
      </SettingsSection>
    </div>
  );
};
