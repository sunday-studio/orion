export const appStatusTone = (status?: string) =>
  status === "receiving" ? "up" : status === "quiet" ? "degraded" : "unknown";

export const appStatusLabel = (status?: string) =>
  status === "receiving" ? "Receiving" : status === "quiet" ? "Quiet" : "Never";

export const agoShort = (iso?: string) => {
  if (!iso) return "No logs yet";
  const seconds = Math.max(Math.round((Date.now() - new Date(iso).getTime()) / 1000), 0);
  if (seconds < 60) return `${seconds} s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.round(minutes / 60);
  return hours < 48 ? `${hours} h ago` : `${Math.round(hours / 24)} days ago`;
};

export const ratePerSecond = (rate?: number) =>
  rate === undefined || rate === null ? "-" : `${Math.round(rate)} / s`;

export const gigabytes = (bytes?: number) => {
  const gb = (bytes ?? 0) / 1_073_741_824;
  return gb >= 0.1 ? gb.toFixed(1) : ((bytes ?? 0) / 1_048_576).toFixed(0);
};

export const storageUnit = (bytes?: number) => ((bytes ?? 0) / 1_073_741_824 >= 0.1 ? "GB" : "MB");

export const compact = (n?: number) =>
  (n ?? 0) >= 1_000_000 ? `${((n ?? 0) / 1_000_000).toFixed(1)}M` : (n ?? 0).toLocaleString();
