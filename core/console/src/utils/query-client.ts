import {createSyncStoragePersister} from "@tanstack/query-sync-storage-persister";
import {QueryClient} from "@tanstack/react-query";
import {persistQueryClient} from "@tanstack/react-query-persist-client";

export const initQueryClient = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        gcTime: 1000 * 60 * 60 * 24, // 1 day
        retry: false,
      },
    },
  });

  const localStoragePersister = createSyncStoragePersister({
    storage: window.localStorage,
  });

  persistQueryClient({
    queryClient,
    dehydrateOptions: {
      shouldDehydrateQuery(query) {
        if (query.meta?.persist === false) return false;
        return query.state.status === "success";
      },
    },
    persister: localStoragePersister,
  });

  return queryClient;
};
