import { AppHeader } from "@/components/shared/app-header";
import { Outlet } from "react-router-dom";

export const Layout = () => {
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-neutral-100">
      <AppHeader />

      <div className="min-h-0 flex-1 overflow-y-auto bg-white">
        <main className="flex w-full flex-col gap-6 px-4 pt-8 pb-8 sm:px-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
};
