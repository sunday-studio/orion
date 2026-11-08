import { useQueryClient } from "@tanstack/react-query";
import {
  type ApiUpdateSettings,
  type ApiUpdateSettingsRequest,
  getGetUpdateSettingsQueryKey,
  useCheckUpdatesNow,
  useGetUpdateSettings,
  useUpdateUpdateSettings,
} from "@/orion-sdk";

/** What Core knows about releases. Core checks once a day, so a few minutes of caching is plenty. */
export const useUpdates = () =>
  useGetUpdateSettings({ query: { staleTime: 5 * 60_000, retry: false } });

const useStoreUpdates = () => {
  const queryClient = useQueryClient();
  return (data: ApiUpdateSettings) =>
    queryClient.setQueryData(getGetUpdateSettingsQueryKey(), data);
};

export const useCheckForUpdates = () => {
  const store = useStoreUpdates();
  return useCheckUpdatesNow({ mutation: { onSuccess: store } });
};

export const useChangeUpdateSettings = () => {
  const store = useStoreUpdates();
  return useUpdateUpdateSettings({ mutation: { onSuccess: store } });
};

export type { ApiUpdateSettings, ApiUpdateSettingsRequest };
