import { AppHeader } from "@/components/app-header";
import { Outlet } from "react-router-dom";

export const Layout = () => {
  return (
    <div className="min-h-screen bg-neutral-100">
      <AppHeader />

      <div className="w-full bg-white h-screen ">
        <main className="mx-auto flex max-w-7xl flex-col gap-6 px-4 pt-10 pb-8 sm:px-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
};
