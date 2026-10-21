import { useQueryClient } from "@tanstack/react-query";
import {
  getGetNodeDiscoveryQueryKey,
  getListNodeDiscoveryRequestsQueryKey,
  useCancelNodeDiscoveryRequest,
  useCreateNodeDiscoveryRequest,
} from "@/orion-sdk";
import { useToast } from "@/design-system";

/** Queues an action for the node (rescan, dismiss, ...) and refreshes the request list. */
export const useDiscoveryRequest = (nodeId: string) => {
  const toast = useToast();
  const queryClient = useQueryClient();
  const refresh = () => {
    void queryClient.invalidateQueries({
      queryKey: getListNodeDiscoveryRequestsQueryKey(nodeId),
    });
    void queryClient.invalidateQueries({ queryKey: getGetNodeDiscoveryQueryKey(nodeId) });
  };
  const create = useCreateNodeDiscoveryRequest({
    mutation: {
      onSuccess: refresh,
      onError: (failure) =>
        toast.add({ title: (failure as Error).message || "Unable to queue the request" }),
    },
  });
  const cancel = useCancelNodeDiscoveryRequest({ mutation: { onSuccess: refresh } });
  return {
    pending: create.isPending,
    send: (type: string, payload?: Record<string, unknown>) =>
      create.mutate({
        nodeId,
        data: { type: type as "rescan", payload },
      }),
    cancel: (requestId: string) => cancel.mutate({ id: nodeId, requestId }),
  };
};
