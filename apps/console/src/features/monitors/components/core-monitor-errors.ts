const blockedTargetMarkers = [
  "blocked for core monitors",
  "blocked core monitor address",
  "targets localhost",
  "loopback address",
  "private network address",
  "metadata address",
  "link-local address",
  "multicast address",
  "unspecified address",
];

const blockedTargetMessage =
  "Core blocked that monitor target. Use a public hostname, or enable ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS only for a trusted internal deployment. Localhost, private network, link-local, multicast, and cloud metadata targets are blocked by default.";

const rawMutationErrorMessage = (error: unknown, fallback: string) => {
  if (!error) return "";
  if (error instanceof Error) return error.message;
  if (typeof error === "object" && "message" in error) {
    return String((error as { message?: unknown }).message ?? fallback);
  }
  return fallback;
};

export const coreMonitorMutationErrorMessage = (error: unknown, fallback: string) => {
  const message = rawMutationErrorMessage(error, fallback);
  const normalized = message.toLowerCase();
  if (blockedTargetMarkers.some((marker) => normalized.includes(marker))) {
    return blockedTargetMessage;
  }
  return message;
};
