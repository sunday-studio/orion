// Applies a list of link requests one at a time and reports what failed, so one bad link
// never hides the rest. Pure: the caller passes in the request function.
import type {
  ServiceServiceInput,
  ServiceServiceLinkRef,
  ServiceServiceLinkResult,
} from "../../orion-sdk/index.ts";

export type LinkOutcome = {
  linked: ServiceServiceLinkResult[];
  failed: { ref: ServiceServiceLinkRef; message: string }[];
};

export const runLinks = async (
  refs: readonly ServiceServiceLinkRef[],
  link: (ref: ServiceServiceLinkRef) => Promise<{ link?: ServiceServiceLinkResult }>,
): Promise<LinkOutcome> => {
  const outcome: LinkOutcome = { linked: [], failed: [] };
  for (const ref of refs) {
    try {
      const response = await link(ref);
      if (response.link) outcome.linked.push(response.link);
    } catch (error) {
      outcome.failed.push({
        ref,
        message: error instanceof Error && error.message ? error.message : "Unable to link.",
      });
    }
  }
  return outcome;
};

export type AcceptPlan = {
  create?: ServiceServiceInput;
  serviceId?: string;
  refs: ServiceServiceLinkRef[];
};

export type AcceptResult = {
  serviceId?: string;
  name?: string;
  outcome: LinkOutcome;
  /** Set when the service could not be created, so nothing was linked for this plan. */
  error?: string;
};

/** Creates the service when the plan needs one, then links its members. */
export const acceptPlan = async (
  plan: AcceptPlan,
  api: {
    create: (input: ServiceServiceInput) => Promise<{ service?: { id?: string; name?: string } }>;
    link: (
      serviceId: string,
      ref: ServiceServiceLinkRef,
    ) => Promise<{ link?: ServiceServiceLinkResult }>;
  },
): Promise<AcceptResult> => {
  let serviceId = plan.serviceId;
  let name: string | undefined;
  if (!serviceId && plan.create) {
    try {
      const created = await api.create(plan.create);
      serviceId = created.service?.id;
      name = created.service?.name;
    } catch (error) {
      return {
        name: plan.create.name,
        outcome: { linked: [], failed: [] },
        error:
          error instanceof Error && error.message ? error.message : "Unable to create service.",
      };
    }
  }
  if (!serviceId) {
    return { outcome: { linked: [], failed: [] }, error: "No service to link to." };
  }
  const id = serviceId;
  return { serviceId: id, name, outcome: await runLinks(plan.refs, (ref) => api.link(id, ref)) };
};
