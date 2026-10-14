import { AppHeader } from "@/components/shared/app-header";
import { Suspense } from "react";
import { Outlet } from "react-router-dom";

export const Layout = () => {
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-neutral-100">
      <AppHeader />

      <div className="min-h-0 flex-1 overflow-y-auto bg-white">
        <main className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 pt-10 pb-8 sm:px-6">
          <Suspense fallback={<div className="py-3 text-sm text-neutral-600">Loading...</div>}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  );
};
