import { Toast as ToastPrimitive } from "@base-ui/react/toast";
import { XIcon } from "lucide-react";

export const useToast = ToastPrimitive.useToastManager;

const ToastList = () => {
  const { toasts } = ToastPrimitive.useToastManager();
  return toasts.map((toast) => (
    <ToastPrimitive.Root
      key={toast.id}
      toast={toast}
      className="relative rounded-[10px] border border-hairline bg-card p-3 pr-9 text-[13px] shadow-overlay transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0"
    >
      <ToastPrimitive.Content>
        <ToastPrimitive.Title className="font-medium" />
        <ToastPrimitive.Description className="text-quiet" />
        {toast.actionProps && (
          <ToastPrimitive.Action className="mt-2 rounded-md border border-input-border bg-chip px-2 py-1 text-[12.5px] font-medium hover:bg-tray" />
        )}
        <ToastPrimitive.Close
          aria-label="Dismiss"
          className="absolute right-2 top-2 rounded p-1 text-quiet hover:text-ink"
        >
          <XIcon className="size-3.5" />
        </ToastPrimitive.Close>
      </ToastPrimitive.Content>
    </ToastPrimitive.Root>
  ));
};

/** Mount once near the app root. Call useToast().add({ title, description }) to show a toast. */
export const ToastProvider = ({ children }: { children: React.ReactNode }) => (
  <ToastPrimitive.Provider>
    {children}
    <ToastPrimitive.Portal>
      <ToastPrimitive.Viewport className="fixed bottom-4 right-4 z-[60] flex w-80 flex-col gap-2">
        <ToastList />
      </ToastPrimitive.Viewport>
    </ToastPrimitive.Portal>
  </ToastPrimitive.Provider>
);
