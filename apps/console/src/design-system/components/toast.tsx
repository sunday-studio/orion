import { Toast as ToastPrimitive } from "@base-ui/react/toast";
import { XIcon } from "lucide-react";
import "@/design-system/components/toast.css";

export const useToast = ToastPrimitive.useToastManager;

const ToastList = () => {
  const { toasts } = ToastPrimitive.useToastManager();
  return toasts.map((toast) => (
    <ToastPrimitive.Root key={toast.id} toast={toast} className="ds-toast">
      <ToastPrimitive.Content className="ds-toast-content">
        <ToastPrimitive.Title className="font-medium" />
        <ToastPrimitive.Description className="whitespace-pre-line text-quiet" />
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

/** Mount once near the app root. Call useToast().add({ title, description }); toasts stack. */
export const ToastProvider = ({ children }: { children: React.ReactNode }) => (
  <ToastPrimitive.Provider limit={5}>
    {children}
    <ToastPrimitive.Portal>
      <ToastPrimitive.Viewport className="ds-toast-viewport">
        <ToastList />
      </ToastPrimitive.Viewport>
    </ToastPrimitive.Portal>
  </ToastPrimitive.Provider>
);
