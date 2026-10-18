import { getApiBase } from "@/api/client";

/** Absolute URL of a status page: a custom domain as given, or the /status path on Core. */
export const publicPageHref = (publicUrl: string | undefined, coreUrl?: string) => {
  const value = publicUrl ?? "";
  if (/^https?:\/\//.test(value)) return value.replace(/\/$/, "");
  const core = (coreUrl || getApiBase() || window.location.origin).replace(/\/$/, "");
  if (value.startsWith("/")) return `${core}${value}`;
  return value ? `https://${value}` : core;
};

/** The public pages Core serves for one status page. */
export const publicPageLinks = (href: string) => [
  { label: "View page", href },
  { label: "History", href: `${href}/history` },
  { label: "Subscribe", href: `${href}/subscribe` },
  { label: "RSS feed", href: `${href}/feed.rss` },
];
