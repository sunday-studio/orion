import { getApiBase } from "@/api/client";
import { toZoned } from "@/utils/date";

export const NOT_CONNECTED_AFTER_MS = 5 * 60 * 1000;

export type ServerPlatform = "linux" | "macos" | "docker";

/** Core only mints enrollments for these platforms (Windows is out of scope, D-014). */
export const platformTiles = [
  { value: "linux", title: "Linux", caption: "systemd" },
  { value: "macos", title: "macOS", caption: "launchd" },
  { value: "docker", title: "Docker", caption: "compose" },
] as const;

export const coreOrigin = () => getApiBase() || window.location.origin;

export const coreHost = (coreUrl?: string) => {
  try {
    return new URL(coreUrl || coreOrigin()).host;
  } catch {
    return window.location.host;
  }
};

const pad = (value: number) => String(value).padStart(2, "0");

export const clockWithSeconds = (iso?: string) => {
  if (!iso) return "-";
  const date = toZoned(new Date(iso));
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
};
