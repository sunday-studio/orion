import { Link } from "react-router-dom";
import type { ServiceServiceDetail } from "@/orion-sdk";
import { serviceHref } from "../services.domain";

const names = (items: readonly { name?: string }[] | undefined) =>
  (items ?? [])
    .map((item) => item.name)
    .filter(Boolean)
    .join(", ") || "None";

/** What the incident's service is made of, so the people on call see the whole picture. */
export const incidentServiceFacts = (service: ServiceServiceDetail) => {
  const monitors = service.monitors ?? [];
  const down = monitors.filter((monitor) => monitor.health === "down").length;
  return [
    [
      "Service",
      <Link key="s" to={serviceHref(service.id ?? "")} className="truncate underline">
        {service.name}
      </Link>,
    ],
    [
      "Apps",
      <span key="a" className="truncate">
        {names(service.apps)}
      </span>,
    ],
    [
      "Resources",
      <span key="r" className="truncate">
        {names(service.resources)}
      </span>,
    ],
    [
      "Monitors",
      <span key="m">{down > 0 ? `${monitors.length}, ${down} down` : monitors.length}</span>,
    ],
  ] as const;
};
