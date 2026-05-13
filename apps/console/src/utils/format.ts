export function formatLastSeen(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  const s = Math.floor((Date.now() - d.getTime()) / 1000);
  if (s < 60) return "now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 604800) return `${Math.floor(s / 86400)}d ago`;
  if (s < 2592000) return `${Math.floor(s / 604800)}w ago`;
  return d.toLocaleDateString();
}

export function formatUptime(seconds?: number | null): string {
  if (seconds == null || seconds < 0) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const parts: string[] = [];
  if (d > 0) parts.push(`${d}d`);
  if (h > 0) parts.push(`${h}h`);
  if (m > 0 || parts.length === 0) parts.push(`${m}m`);
  return parts.join(" ");
}

export function descriptionFromMeta(meta?: string | null): string {
  if (!meta) return "—";
  try {
    const o = JSON.parse(meta) as unknown;
    if (
      o &&
      typeof o === "object" &&
      "description" in o &&
      typeof (o as { description?: string }).description === "string"
    ) {
      return (o as { description: string }).description;
    }
  } catch {
    // ignore
  }
  return meta.slice(0, 40) + (meta.length > 40 ? "…" : "");
}
