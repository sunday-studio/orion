import { Button, Dialog, DialogContent, DialogTitle } from "@/design-system";

export const shortcuts = [
  { label: "Search and jump", keys: "Cmd K" },
  { label: "Go to Incidents", keys: "G then I" },
  { label: "Go to Monitors", keys: "G then M" },
  { label: "Go to Servers", keys: "G then S" },
  { label: "Move down or up a row", keys: "J, K" },
  { label: "Open the selected row", keys: "Enter" },
  { label: "Focus the filter bar", keys: "/" },
  { label: "This sheet", keys: "?" },
] as const;

export const ShortcutsSheet = ({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) => (
  <Dialog open={open} onOpenChange={onOpenChange}>
    <DialogContent className="max-w-[440px] gap-3.5 rounded-2xl p-[18px]">
      <DialogTitle className="text-[17px] font-medium">Keyboard shortcuts</DialogTitle>
      <ul className="m-0 list-none overflow-hidden rounded-[10px] border border-hairline p-0">
        {shortcuts.map((shortcut) => (
          <li
            key={shortcut.label}
            className="grid grid-cols-[1fr_130px] items-center border-b border-line-soft px-3.5 py-[9px] text-[13px] last:border-b-0"
          >
            <span>{shortcut.label}</span>
            <kbd className="font-sans text-xs text-faint">{shortcut.keys}</kbd>
          </li>
        ))}
      </ul>
      <div className="flex justify-end">
        <Button variant="secondary" onClick={() => onOpenChange(false)}>
          Close
        </Button>
      </div>
    </DialogContent>
  </Dialog>
);
