import { AppHeader } from "@/components/shared/app-header";
import { GlobalShortcuts } from "@/components/shared/global-shortcuts";
import { OfflineBanner } from "@/components/shared/offline-banner";
import { RouteErrorBoundary } from "@/components/shared/route-error-boundary";
import { Suspense } from "react";
import { Outlet, useLocation } from "react-router-dom";

export const Layout = () => {
  const { pathname } = useLocation();
  return (
    <div className="flex h-screen flex-col overflow-hidden bg-neutral-100">
      <AppHeader />
      <GlobalShortcuts />
      <OfflineBanner />

      <div className="min-h-0 flex-1 overflow-y-auto bg-white">
        <main className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 pt-10 pb-8 sm:px-6">
          <Suspense fallback={<div className="py-3 text-sm text-neutral-600">Loading...</div>}>
            <RouteErrorBoundary key={pathname}>
              <Outlet />
            </RouteErrorBoundary>
          </Suspense>
        </main>
      </div>
    </div>
  );
};
