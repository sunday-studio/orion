import { useState } from "react";
import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  DrawerTrigger,
  Menu,
  MenuContent,
  MenuItem,
  MenuSeparator,
  MenuTrigger,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Tooltip,
  buttonStyles,
  useToast,
} from "@/design-system";
import { Section } from "@/design-system/preview/preview-controls";

export const OverlaysPreview = () => {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmed, setConfirmed] = useState(0);
  const toast = useToast();

  return (
    <Section title="Overlays">
      <div className="flex flex-wrap items-center gap-2">
        <Dialog>
          <DialogTrigger className={buttonStyles({ variant: "secondary" })}>
            Open dialog
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Example dialog</DialogTitle>
              <DialogDescription>
                Escape closes it and focus returns to the trigger.
              </DialogDescription>
            </DialogHeader>
          </DialogContent>
        </Dialog>

        <Button variant="destructive" onClick={() => setConfirmOpen(true)}>
          Delete server
        </Button>
        <ConfirmDialog
          open={confirmOpen}
          onOpenChange={setConfirmOpen}
          title="Delete edge-01?"
          description="4 monitors on this server will be deleted."
          confirmLabel="Delete server"
          onConfirm={() => setConfirmed((count) => count + 1)}
        />
        <span aria-live="polite" className="text-xs text-quiet">
          Confirmed {confirmed}
        </span>

        <Drawer swipeDirection="right">
          <DrawerTrigger className={buttonStyles({ variant: "secondary" })}>
            Open drawer
          </DrawerTrigger>
          <DrawerContent>
            <DrawerTitle>Example drawer</DrawerTitle>
            <DrawerDescription>Detail panels slide in from the right.</DrawerDescription>
          </DrawerContent>
        </Drawer>

        <Popover>
          <PopoverTrigger className={buttonStyles({ variant: "secondary" })}>
            Open popover
          </PopoverTrigger>
          <PopoverContent>Popover content</PopoverContent>
        </Popover>

        <Tooltip content="Helpful hint">
          <button type="button" className={buttonStyles({ variant: "secondary" })}>
            Hover or focus me
          </button>
        </Tooltip>

        <Menu>
          <MenuTrigger className={buttonStyles({ variant: "secondary" })}>Open menu</MenuTrigger>
          <MenuContent>
            <MenuItem>Rename</MenuItem>
            <MenuItem>Duplicate</MenuItem>
            <MenuSeparator />
            <MenuItem>Delete</MenuItem>
          </MenuContent>
        </Menu>

        <Button
          variant="secondary"
          onClick={() => toast.add({ title: "Saved", description: "Your changes were saved." })}
        >
          Show toast
        </Button>
      </div>
    </Section>
  );
};
