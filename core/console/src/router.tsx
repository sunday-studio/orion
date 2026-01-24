import {Route, createBrowserRouter, createRoutesFromElements} from "react-router";
import {Home} from "./features/home";
import {GifDetailView} from "./features/detail-view";
import {Analytics} from "./features/analytics";
import {Layout} from "@/components/layout";

export const router = createBrowserRouter(
  createRoutesFromElements(
    <Route path="" element={<Layout />}>
      <Route index element={<Home />} />
    </Route>
  )
);
