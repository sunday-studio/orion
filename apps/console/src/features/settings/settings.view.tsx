import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import {
  getGetDataLifecycleSettingsQueryKey,
  type ServiceDataLifecycleSettingsPayload,
  useGetDataLifecycleSettings,
  useRunDataLifecycleArchive,
  useRunDataLifecycleRollup,
  useUpdateDataLifecycleSettings,
} from "@/orion-sdk";
import { DATE_TIME_FORMAT, ISO_DATE_FORMAT, formatDate } from "@/lib/date-utils";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type ReactNode } from "react";

type SettingsFormState = {
  rawReportHotDays: string;
  archiveRawReports: boolean;
  archiveDir: string;
  rollupsEnabled: boolean;
  rollupRetentionDays: string;
  archiveSchedule: string;
};

const defaultFormState: SettingsFormState = {
  rawReportHotDays: "",
  archiveRawReports: false,
  archiveDir: "",
  rollupsEnabled: false,
  rollupRetentionDays: "",
  archiveSchedule: "",
};

const asNumber = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
};

const DetailItem = ({ label, value }: { label: string; value: string | number }) => (
  <div>
    <div className="text-sm text-neutral-600">{label}</div>
    <div className="text-sm font-medium">{value}</div>
  </div>
);

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
  const settingsResponse = useGetDataLifecycleSettings();
  const updateSettings = useUpdateDataLifecycleSettings({
    mutation: {
      onSuccess: () => {
        void queryClient.invalidateQueries({ queryKey: getGetDataLifecycleSettingsQueryKey() });
      },
    },
  });
  const archiveRun = useRunDataLifecycleArchive();
  const rollupRun = useRunDataLifecycleRollup();

  const settings = settingsResponse.data?.settings;
  const [formState, setFormState] = useState(defaultFormState);
  const [rollupDate, setRollupDate] = useState("");

  useEffect(() => {
    if (!settings) return;
    setFormState({
      rawReportHotDays: String(settings.raw_report_hot_days ?? ""),
      archiveRawReports: Boolean(settings.archive_raw_reports),
      archiveDir: settings.archive_dir ?? "",
      rollupsEnabled: Boolean(settings.rollups_enabled),
      rollupRetentionDays: String(settings.rollup_retention_days ?? ""),
      archiveSchedule: settings.archive_schedule ?? "",
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
      archive_schedule: formState.archiveSchedule.trim() || undefined,
      raw_report_hot_days: asNumber(formState.rawReportHotDays),
      rollup_retention_days: asNumber(formState.rollupRetentionDays),
      rollups_enabled: formState.rollupsEnabled,
    };
    updateSettings.mutate({ data: payload });
  };

  const runRollup = () => {
    rollupRun.mutate({
      data: rollupDate.trim() === "" ? undefined : { date: rollupDate.trim() },
    });
  };

  const runArchive = () => {
    archiveRun.mutate(undefined);
  };

  return (
    <div className="space-y-7">
      <div>
        <h1 className="text-base font-medium">Settings</h1>
        <p className="text-sm text-neutral-600">Control how Core keeps and archives report data.</p>
      </div>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Data Retention</h2>
          <p className="text-sm text-neutral-600">
            Keep recent reports fast in Core, then move older report history into local archives and
            daily rollups.
          </p>
        </div>

        {settingsResponse.isLoading && (
          <div className="text-sm text-neutral-600">Loading data lifecycle settings...</div>
        )}
        {settingsResponse.error && (
          <div className="text-sm">Unable to load data lifecycle settings.</div>
        )}

        <div className="grid gap-3 sm:grid-cols-2">
          <Field
            label="Keep raw reports in Core"
            description="Older raw reports can be archived so the Core database stays quick."
          >
            <Input
              type="number"
              min="1"
              value={formState.rawReportHotDays}
              onChange={(event) => updateField("rawReportHotDays", event.target.value)}
            />
          </Field>
          <Field
            label="Keep daily rollups"
            description="Rollups preserve long-term uptime history without keeping every raw check."
          >
            <Input
              type="number"
              min="1"
              value={formState.rollupRetentionDays}
              onChange={(event) => updateField("rollupRetentionDays", event.target.value)}
            />
          </Field>
          <Field
            label="Archive directory"
            description="Local path where archived reports are stored."
          >
            <Input
              value={formState.archiveDir}
              onChange={(event) => updateField("archiveDir", event.target.value)}
              placeholder="./data/archive"
            />
          </Field>
          <Field label="Archive schedule" description="How often Core should archive old reports.">
            <Input
              value={formState.archiveSchedule}
              onChange={(event) => updateField("archiveSchedule", event.target.value)}
              placeholder="daily"
            />
          </Field>
        </div>

        <div className="flex flex-wrap gap-4">
          <label className="flex items-center gap-2 text-sm">
            <Checkbox
              checked={formState.archiveRawReports}
              onCheckedChange={(checked) => updateField("archiveRawReports", checked === true)}
            />
            Archive raw reports
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
          <Button onClick={saveSettings} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? "Saving..." : "Save settings"}
          </Button>
          {updateSettings.isError && <span className="text-sm">Unable to save settings.</span>}
          {updateSettings.isSuccess && (
            <span className="text-sm text-neutral-600">Settings saved.</span>
          )}
        </div>

        {(settings?.last_archive_run_at ||
          settings?.last_rollup_run_at ||
          settings?.last_archive_status ||
          settings?.last_archive_error) && (
          <div className="space-y-2">
            <h3 className="text-sm font-medium">Last Run</h3>
            <div className="grid gap-3 sm:grid-cols-3">
              <DetailItem
                label="archive"
                value={formatDate(settings?.last_archive_run_at, DATE_TIME_FORMAT)}
              />
              <DetailItem
                label="rollup"
                value={formatDate(settings?.last_rollup_run_at, DATE_TIME_FORMAT)}
              />
              <DetailItem label="archive status" value={settings?.last_archive_status ?? "—"} />
            </div>
            {settings?.last_archive_error && (
              <div className="text-sm text-red-700">{settings.last_archive_error}</div>
            )}
          </div>
        )}
      </section>

      <section className="space-y-3">
        <div>
          <h2 className="text-sm font-medium">Run Now</h2>
          <p className="text-sm text-neutral-600">Run archive or rollup maintenance immediately.</p>
        </div>

        <div className="flex flex-wrap items-end gap-3">
          <Field label="Rollup date" description="Leave empty to roll up the default eligible day.">
            <Input
              type="date"
              value={rollupDate}
              onChange={(event) => setRollupDate(event.target.value)}
              max={formatDate(new Date(), ISO_DATE_FORMAT, "")}
              className="w-48"
            />
          </Field>
          <Button onClick={runRollup} disabled={rollupRun.isPending}>
            {rollupRun.isPending ? "Running..." : "Run rollup"}
          </Button>
          <Button variant="outline" onClick={runArchive} disabled={archiveRun.isPending}>
            {archiveRun.isPending ? "Running..." : "Run archive"}
          </Button>
        </div>

        {rollupRun.data?.result && (
          <div className="grid gap-3 sm:grid-cols-3">
            <DetailItem label="rollup date" value={rollupRun.data.result.date ?? "—"} />
            <DetailItem label="monitor days" value={rollupRun.data.result.monitor_days ?? 0} />
            <DetailItem label="reports read" value={rollupRun.data.result.report_count ?? 0} />
          </div>
        )}
        {archiveRun.data?.result && (
          <div className="grid gap-3 sm:grid-cols-3">
            <DetailItem
              label="agent reports"
              value={archiveRun.data.result.agent_reports_archived ?? 0}
            />
            <DetailItem
              label="monitor reports"
              value={archiveRun.data.result.monitor_reports_archived ?? 0}
            />
            <DetailItem label="archive path" value={archiveRun.data.result.archive_path ?? "—"} />
          </div>
        )}
        {(rollupRun.isError || archiveRun.isError) && (
          <div className="text-sm">Unable to run one of the data actions.</div>
        )}
      </section>
    </div>
  );
};
