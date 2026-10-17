import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";

const shortcuts = [
  { keys: "Ctrl/Cmd + K", action: "Open the command palette" },
  { keys: "?", action: "Show keyboard shortcuts" },
  { keys: "Esc", action: "Close dialogs" },
];

export const ShortcutsSheet = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>Keyboard shortcuts</DialogTitle>
      </DialogHeader>
      <dl className="grid grid-cols-[10rem_1fr] gap-y-2 text-sm">
        {shortcuts.map((shortcut) => (
          <div className="contents" key={shortcut.keys}>
            <dt className="font-mono text-xs">{shortcut.keys}</dt>
            <dd>{shortcut.action}</dd>
          </div>
        ))}
      </dl>
    </DialogContent>
  </Dialog>
);
