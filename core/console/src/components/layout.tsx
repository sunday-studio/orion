import {Outlet} from "react-router";

export const Layout = () => {
  return (
    <div className="min-h-screen flex flex-col px-2 sm:px-4 pt-1 pb-3 bg-neutral-800 relative">
      <div className="py-2 text-neutral-200">
        <p className="text-base sm:text-lg font-medium">Sunday Studio Starter</p>
      </div>
      <div className="w-full bg-neutral-50 rounded-xl flex-1 text-neutral-800 flex items-start justify-center py-4 sm:py-8">
        <div className="w-full max-w-full sm:max-w-[95vw] md:max-w-[700px] xl:max-w-[800px] px-2 sm:px-6 box-border">
          <Outlet />
        </div>
      </div>
    </div>
  );
};
