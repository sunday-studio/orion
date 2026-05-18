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
    rollupRun.mutate({ data: undefined });
  };

  const runArchive = () => {
    archiveRun.mutate(undefined);
  };

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
          <Button onClick={runRollup} disabled={rollupRun.isPending}>
            {rollupRun.isPending ? "Running..." : "Run rollup"}
          </Button>
          <Button variant="outline" onClick={runArchive} disabled={archiveRun.isPending}>
            {archiveRun.isPending ? "Running..." : "Run archive"}
          </Button>
          {rollupRun.data?.result && (
            <span className="text-sm text-neutral-600">
              Rolled up {rollupRun.data.result.report_count ?? 0} reports.
            </span>
          )}
          {archiveRun.data?.result && (
            <span className="text-sm text-neutral-600">
              Archived{" "}
              {(archiveRun.data.result.agent_reports_archived ?? 0) +
                (archiveRun.data.result.monitor_reports_archived ?? 0)}{" "}
              reports.
            </span>
          )}
          {(rollupRun.isError || archiveRun.isError) && (
            <span className="text-sm">Unable to run maintenance.</span>
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
