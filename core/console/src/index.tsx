import {initQueryClient} from "./utils/query-client";
import {QueryClientProvider} from "@tanstack/react-query";
import {RouterProvider} from "react-router";
import {router} from "./router";

const queryClient = initQueryClient();

export const App = () => {
  return (
    <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
    </QueryClientProvider>
  );
};
