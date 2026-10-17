import { AppHeader } from "@/components/shared/app-header";
import { GlobalShortcuts } from "@/components/shared/global-shortcuts";
import { ToastProvider } from "@/design-system";
import { OfflineBanner } from "@/components/shared/offline-banner";
import { RouteErrorBoundary } from "@/components/shared/route-error-boundary";
import { Suspense } from "react";
import { Outlet, useLocation } from "react-router-dom";

export const Layout = () => {
  const { pathname } = useLocation();
  return (
    <ToastProvider>
      <div className="flex h-screen flex-col overflow-hidden bg-page">
        <AppHeader />
        <GlobalShortcuts />
        <OfflineBanner />

        <div className="min-h-0 flex-1 overflow-y-auto bg-page">
          <main className="mx-auto flex w-full max-w-[1040px] flex-col gap-6 px-4 pt-10 pb-8 sm:px-6">
            <Suspense fallback={<div className="py-3 text-sm text-neutral-600">Loading...</div>}>
              <RouteErrorBoundary key={pathname}>
                <Outlet />
              </RouteErrorBoundary>
            </Suspense>
          </main>
        </div>
      </div>
    </ToastProvider>
  );
};
