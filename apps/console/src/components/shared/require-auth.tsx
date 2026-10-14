import { Navigate, Outlet, useLocation } from "react-router-dom";
import { getToken } from "@/api/client";

export const RequireAuth = () => {
  const location = useLocation();
  if (!getToken()) {
    const from = `${location.pathname}${location.search}`;
    const target = from === "/" ? "/login" : `/login?from=${encodeURIComponent(from)}`;
    return <Navigate to={target} replace />;
  }
  return <Outlet />;
};
