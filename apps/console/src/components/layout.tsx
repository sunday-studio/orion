import { AppHeader } from "@/components/app-header";
import { Outlet } from "react-router-dom";

export const Layout = () => {
  return (
    <div className="mx-auto flex min-h-screen max-w-7xl flex-col gap-6 px-4 py-8 sm:px-6">
      <AppHeader />
      <Outlet />
    </div>
  );
};
