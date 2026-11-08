import { AppSidebar } from "@/components/sidebar/app-sidebar";
import { useSidebarMode } from "@/components/sidebar/use-sidebar-mode";
import { GlobalShortcuts } from "@/components/shared/global-shortcuts";
import { ToastProvider } from "@/design-system";
import { OfflineBanner } from "@/components/shared/offline-banner";
import { RouteErrorBoundary } from "@/components/shared/route-error-boundary";
import { Suspense } from "react";
import { Outlet, useLocation } from "react-router-dom";

export const Layout = () => {
  const { pathname } = useLocation();
  const { mode, setMode } = useSidebarMode();
  return (
    <ToastProvider>
      <div className="relative flex h-screen overflow-hidden bg-page">
        <AppSidebar mode={mode} onModeChange={setMode} />
        <GlobalShortcuts />

        <div className="flex min-w-0 flex-1 flex-col">
          <OfflineBanner />
          <div className="min-h-0 flex-1 overflow-y-auto bg-page">
            <main className="mx-auto flex w-full max-w-[1040px] flex-col gap-3 px-6 pb-8">
              <Suspense fallback={<div className="py-3 text-sm text-neutral-600">Loading...</div>}>
                <RouteErrorBoundary key={pathname}>
                  <div className="flex flex-col gap-3 animate-in fade-in duration-150 motion-reduce:animate-none">
                    <Outlet />
                  </div>
                </RouteErrorBoundary>
              </Suspense>
            </main>
          </div>
        </div>
      </div>
    </ToastProvider>
  );
};
