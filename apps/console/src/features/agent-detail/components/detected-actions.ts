import { useQueryClient } from "@tanstack/react-query";
import {
  getGetAgentDiscoveryQueryKey,
  getListAgentDiscoveryRequestsQueryKey,
  useCancelAgentDiscoveryRequest,
  useCreateAgentDiscoveryRequest,
} from "@/orion-sdk";
import { useToast } from "@/design-system";

/** Queues an action for the agent (rescan, dismiss, ...) and refreshes the request list. */
export const useDiscoveryRequest = (agentId: string) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const refresh = () => {
    void queryClient.invalidateQueries({
      queryKey: getListAgentDiscoveryRequestsQueryKey(agentId),
    });
    void queryClient.invalidateQueries({ queryKey: getGetAgentDiscoveryQueryKey(agentId) });
  };
  const create = useCreateAgentDiscoveryRequest({
    mutation: {
      onSuccess: refresh,
      onError: (failure) =>
        toast.add({ title: (failure as Error).message || "Unable to queue the request" }),
    },
  });
  const cancel = useCancelAgentDiscoveryRequest({ mutation: { onSuccess: refresh } });
  return {
    pending: create.isPending,
    send: (type: string, payload?: Record<string, unknown>) =>
      create.mutate({
        agentId,
        data: { type: type as "rescan", payload },
      }),
    cancel: (requestId: string) => cancel.mutate({ id: agentId, requestId }),
  };
};
