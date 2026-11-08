import type { ApiUpdateSettings } from "../../orion-sdk/index.ts";

type Parsed = { nums: number[]; pre: string };

const parse = (version: string | undefined): Parsed | undefined => {
  const core = (version ?? "").trim().replace(/^v/, "").split("+")[0] ?? "";
  const [numbers, ...rest] = core.split("-");
  const parts = (numbers ?? "").split(".");
  if (!numbers || parts.length > 3) return undefined;
  const nums = parts.map((part) => (/^\d+$/.test(part) ? Number(part) : Number.NaN));
  if (nums.some(Number.isNaN)) return undefined;
  while (nums.length < 3) nums.push(0);
  return { nums, pre: rest.join("-") };
};

/** Orders two versions; undefined when either is not a version, such as a "dev" build. */
export const compareVersions = (a: string | undefined, b: string | undefined) => {
  const left = parse(a);
  const right = parse(b);
  if (!left || !right) return undefined;
  for (let i = 0; i < 3; i += 1) {
    const diff = (left.nums[i] ?? 0) - (right.nums[i] ?? 0);
    if (diff !== 0) return diff < 0 ? -1 : 1;
  }
  if (left.pre === right.pre) return 0;
  if (!left.pre) return 1;
  if (!right.pre) return -1;
  return left.pre < right.pre ? -1 : 1;
};

/** True when `latest` is a newer version than `current`. */
export const isBehind = (current: string | undefined, latest: string | undefined) =>
  compareVersions(current, latest) === -1;

/**
 * A Node that runs an older release than Core. A Node is only ever updated to the release Core runs,
 * so update Core first; that release is `settings.current_version`.
 */
export const nodeCanUpdate = (nodeVersion: string | undefined, settings?: ApiUpdateSettings) =>
  isBehind(nodeVersion, settings?.current_version);

/** The GitHub page of a release, for its notes. */
export const releaseNotesUrl = (version: string) =>
  `https://github.com/sunday-studio/orion/releases/tag/v${version.replace(/^v/, "")}`;

/** The update worth telling the person about: not skipped, not already installed. */
export const pendingUpdate = (settings?: ApiUpdateSettings) =>
  settings?.state === "available" || settings?.state === "security" ? settings : undefined;

export type UpdateTone = "up" | "info" | "down" | "degraded" | "unknown";

/** Badge text and tone for a state; the same words everywhere an update is shown. */
export const stateBadge = (settings?: ApiUpdateSettings): { tone: UpdateTone; text: string } => {
  const latest = settings?.latest_version;
  switch (settings?.state) {
    case "security":
      return { tone: "down", text: "Security update" };
    case "available":
      return { tone: "info", text: `${latest ?? "A new version"} available` };
    case "skipped":
      return { tone: "unknown", text: `Skipped ${latest ?? ""}`.trim() };
    case "error":
      return { tone: "degraded", text: "Could not check" };
    case "off":
      return { tone: "unknown", text: "Checks are off" };
    case "unchecked":
      return { tone: "unknown", text: "Not checked yet" };
    default:
      return { tone: "up", text: "Up to date" };
  }
};

/** The command that updates a Core run with Docker Compose to `version`. */
export const coreUpdateCommand = (version: string) =>
  `ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:${version} docker compose up -d`;
