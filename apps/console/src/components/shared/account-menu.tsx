import { LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { clearToken } from "@/api/client";
import { RadialAvatar } from "@/components/shared/radial-avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { OPEN_SHORTCUTS_EVENT } from "@/components/shared/global-shortcut-events";

export const AccountMenu = () => {
  const navigate = useNavigate();

  const signOut = () => {
    clearToken();
    navigate("/login", { replace: true });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Account menu"
        className="flex size-8 items-center justify-center rounded-full"
      >
        <RadialAvatar seed="casprin-eSs" />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="min-w-48 rounded-lg border-hairline">
        <DropdownMenuItem onClick={() => window.dispatchEvent(new Event(OPEN_SHORTCUTS_EVENT))}>
          Keyboard shortcuts
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={signOut}>
          <LogOut className="size-4" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
