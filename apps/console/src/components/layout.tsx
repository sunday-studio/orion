import { AppHeader } from "@/components/app-header";
import { Outlet } from "react-router-dom";

export const Layout = () => {
  return (
    <div className="min-h-screen">
      <AppHeader />
      <main className="mx-auto flex max-w-7xl flex-col gap-6 px-4 pt-24 pb-8 sm:px-6">
        <Outlet />
      </main>
    </div>
  );
};
