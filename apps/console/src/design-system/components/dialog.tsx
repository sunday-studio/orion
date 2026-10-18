import { Dialog as DialogPrimitive } from "@base-ui/react/dialog";
import { XIcon } from "lucide-react";
import type * as React from "react";
import { buttonStyles } from "@/design-system/components/button";
import { focusRing } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export const DialogContent = ({
  className,
  children,
  showCloseButton = true,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Popup> & { showCloseButton?: boolean }) => (
  <DialogPrimitive.Portal>
    <DialogPrimitive.Backdrop className="fixed inset-0 z-50 bg-black/50 transition-opacity data-[ending-style]:opacity-0 data-[starting-style]:opacity-0" />
    <DialogPrimitive.Popup
      data-slot="ds-dialog"
      className={cn(
        "fixed left-1/2 top-1/2 z-50 grid w-[calc(100%-2rem)] max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 rounded-[10px] border border-hairline bg-card p-6 shadow-overlay outline-none transition-[opacity,transform] duration-[260ms] ease-[cubic-bezier(0.22,1,0.36,1)] data-[ending-style]:scale-[0.98] data-[ending-style]:opacity-0 data-[starting-style]:scale-[0.96] data-[starting-style]:opacity-0",
        className,
      )}
      {...props}
    >
      {children}
      {showCloseButton && (
        <DialogPrimitive.Close
          aria-label="Close"
          className={cn(
            focusRing,
            "absolute right-3.5 top-3.5 rounded-md p-1.5 text-quiet transition-colors hover:bg-tray hover:text-ink",
          )}
        >
          <XIcon className="size-4" />
        </DialogPrimitive.Close>
      )}
    </DialogPrimitive.Popup>
  </DialogPrimitive.Portal>
);

export const DialogHeader = ({ className, ...props }: React.ComponentProps<"div">) => (
  <div className={cn("space-y-1.5", className)} {...props} />
);

export const DialogFooter = ({
  className,
  children,
  showCloseButton = false,
  ...props
}: React.ComponentProps<"div"> & { showCloseButton?: boolean }) => (
  <div
    className={cn("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)}
    {...props}
  >
    {children}
    {showCloseButton && (
      <DialogPrimitive.Close className={buttonStyles({ variant: "secondary" })}>
        Close
      </DialogPrimitive.Close>
    )}
  </div>
);

export const DialogTitle = ({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) => (
  <DialogPrimitive.Title className={cn("text-[15px] font-medium", className)} {...props} />
);

export const DialogDescription = ({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) => (
  <DialogPrimitive.Description className={cn("text-[13px] text-quiet", className)} {...props} />
);
