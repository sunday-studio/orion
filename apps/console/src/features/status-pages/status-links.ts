import { getApiBase } from "@/api/client";

/**
 * Absolute URL of a status page. A custom domain is used only once Core verified it; until
 * then the page is reachable at its /status/<slug> path on Core.
 */
export const publicPageHref = ({
  slug,
  customDomain,
  domainStatus,
  coreUrl,
}: {
  slug?: string;
  customDomain?: string;
  domainStatus?: string;
  coreUrl?: string;
}) => {
  if (customDomain && domainStatus === "verified") {
    return /^https?:\/\//.test(customDomain)
      ? customDomain.replace(/\/$/, "")
      : `https://${customDomain}`;
  }
  const core = (coreUrl || getApiBase() || window.location.origin).replace(/\/$/, "");
  return `${core}/status/${slug ?? ""}`;
};

/** The public pages Core serves for one status page. */
export const publicPageLinks = (href: string) => [
  { label: "View page", href },
  { label: "History", href: `${href}/history` },
  { label: "Subscribe", href: `${href}/subscribe` },
  { label: "RSS feed", href: `${href}/feed.rss` },
];
