import { useQueryClient } from "@tanstack/react-query";
import {
  getGetMonitorQueryKey,
  getGetMonitorsQueryKey,
  getGetServiceQueryKey,
  getListAppsQueryKey,
  getListServiceSuggestionsQueryKey,
  getListServicesQueryKey,
} from "@/orion-sdk";

/** Refreshes everything a link change can touch. Linking never changes logs or history. */
export const useRefreshServices = () => {
  const queryClient = useQueryClient();
  return (serviceId?: string) => {
    void queryClient.invalidateQueries({ queryKey: getListServicesQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getListServiceSuggestionsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getListAppsQueryKey() });
    void queryClient.invalidateQueries({ queryKey: getGetMonitorsQueryKey() });
    void queryClient.invalidateQueries({
      predicate: (query) => String(query.queryKey[0]).endsWith("/app-resources"),
    });
    if (serviceId)
      void queryClient.invalidateQueries({ queryKey: getGetServiceQueryKey(serviceId) });
  };
};

export const useRefreshMonitor = () => {
  const queryClient = useQueryClient();
  return (monitorId: string) => {
    void queryClient.invalidateQueries({ queryKey: getGetMonitorQueryKey(monitorId) });
  };
};
