import { useQueryClient } from "@tanstack/react-query";
import {
  getGetNodeDiscoveryQueryKey,
  getListNodeDiscoveryRequestsQueryKey,
  useCancelNodeDiscoveryRequest,
  useCreateNodeDiscoveryRequest,
} from "@/orion-sdk";
import { useToast } from "@/design-system";

const queued: Record<string, string> = {
  rescan: "Rescan requested. The Node checks in about every minute.",
  set_kind: "Change requested. The Node applies it at its next check-in.",
  set_scanning: "Change requested. The Node applies it at its next check-in.",
  dismiss: "Dismiss requested",
  restore: "Restore requested",
  ignore: "Never suggest requested",
  track_storage: "Tracking requested. The Node reports a size in a few minutes.",
};

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
      onSuccess: (_result, variables) => {
        refresh();
        toast.add({
          title:
            queued[variables.data.type] ?? "Requested. The Node applies it at its next check-in.",
        });
      },
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
