export type SettingsFormState = {
  rawReportHotDays: string;
  archiveRawReports: boolean;
  archiveDir: string;
  archiveSchedule: string;
  rollupsEnabled: boolean;
  rollupRetentionDays: string;
};

export const defaultFormState: SettingsFormState = {
  rawReportHotDays: "",
  archiveRawReports: false,
  archiveDir: "",
  archiveSchedule: "daily",
  rollupsEnabled: false,
  rollupRetentionDays: "",
};

export type SettingsFieldKey = keyof SettingsFormState | "archiveRollupCompatibility";

export type SettingsFormErrors = Partial<Record<SettingsFieldKey, string>>;

export const asNumber = (value: string) => {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
};

export const getArchiveCutoff = (hotDays?: number) => {
  if (!hotDays) return null;
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - hotDays);
  return cutoff.toISOString();
};

export const getErrorMessage = (error: unknown) => {
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

export const validateSettingsForm = (formState: SettingsFormState): SettingsFormErrors => {
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

export const safeInlineMessage = (value: unknown, fallback = "Unable to complete action.") => {
  if (typeof value !== "string") return fallback;
  const compact = value.replace(/\s+/g, " ").trim();
  if (compact === "") return fallback;
  return compact.length > 180 ? `${compact.slice(0, 177)}...` : compact;
};

export const archiveCount = (result: {
  agent_reports_archived?: number;
  monitor_reports_archived?: number;
}) => (result.agent_reports_archived ?? 0) + (result.monitor_reports_archived ?? 0);
