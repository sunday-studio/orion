import type { GetIncidentsParams } from "@/orion-sdk";

/** Shared by the header chip and the Overview so both read one cached request. */
export const ACTIVE_INCIDENT_PARAMS: GetIncidentsParams = {
  status: "open,acknowledged,covered",
  limit: 20,
};
