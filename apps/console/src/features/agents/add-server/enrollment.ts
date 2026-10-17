import { useCallback, useMemo, useState } from "react";
import { getApiBase } from "@/api/client";

/**
 * Enrollment token model for the Add server flow.
 *
 * Core does not expose enrollment-token routes yet (asked of orion:backend). Until it does, the
 * token below is a placeholder that makes the stub visible instead of inventing a working secret:
 * the command tells the operator where the real enrollment secret goes.
 */
export const ENROLLMENT_API_AVAILABLE = false;

export const ENROLLMENT_TTL_MS = 15 * 60 * 1000;
export const NOT_CONNECTED_AFTER_MS = 5 * 60 * 1000;

export type EnrollmentToken = {
  token: string;
  expiresAt: number;
  /** True while Core cannot mint real tokens and the value is only a placeholder. */
  placeholder: boolean;
};

const mintToken = (): EnrollmentToken => ({
  token: ENROLLMENT_API_AVAILABLE ? "" : "<enrollment-token>",
  expiresAt: Date.now() + ENROLLMENT_TTL_MS,
  placeholder: !ENROLLMENT_API_AVAILABLE,
});

export const coreOrigin = () => getApiBase() || window.location.origin;

export const coreHost = () => {
  try {
    return new URL(coreOrigin()).host;
  } catch {
    return window.location.host;
  }
};

export type ServerSystem = "linux" | "macos" | "docker" | "windows";

export const systemTiles = [
  { value: "linux", title: "Linux", caption: "systemd" },
  { value: "macos", title: "macOS", caption: "launchd" },
  { value: "docker", title: "Docker", caption: "compose" },
  { value: "windows", title: "Windows", caption: "service" },
] as const;

export const installCommand = (system: ServerSystem, token: string, origin = coreOrigin()) => {
  switch (system) {
    case "macos":
      return `curl -fsSL ${origin}/install.sh | sh -s -- --token ${token}`;
    case "docker":
      return `docker run -d --name orion-agent --restart unless-stopped -e ORION_CORE_URL=${origin} -e ORION_ENROLLMENT_TOKEN=${token} ghcr.io/sunday-studio/orion-agent`;
    case "windows":
      return `iwr ${origin}/install.ps1 -UseBasicParsing | iex; orion-agent install --token ${token}`;
    default:
      return `curl -fsSL ${origin}/install.sh | sudo sh -s -- --token ${token}`;
  }
};

/** A token that can be replaced ("Start over") and expires after 15 minutes. */
export const useEnrollmentToken = () => {
  const [enrollment, setEnrollment] = useState<EnrollmentToken>(mintToken);
  const renew = useCallback(() => setEnrollment(mintToken()), []);
  return useMemo(() => ({ enrollment, renew }), [enrollment, renew]);
};
