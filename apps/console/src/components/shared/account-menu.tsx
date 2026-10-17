import { Menu, MenuContent, MenuItem, MenuSeparator, MenuTrigger } from "@/design-system";
import { LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { clearToken } from "@/api/client";
import { RadialAvatar } from "@/components/shared/radial-avatar";
import { OPEN_SHORTCUTS_EVENT } from "@/components/shared/global-shortcut-events";

export const AccountMenu = () => {
  const navigate = useNavigate();

  const signOut = () => {
    clearToken();
    navigate("/login", { replace: true });
  };

  return (
    <Menu>
      <MenuTrigger
        aria-label="Account menu"
        className="flex size-8 items-center justify-center rounded-full"
      >
        <RadialAvatar seed="casprin-eSs" />
      </MenuTrigger>
      <MenuContent className="min-w-48 rounded-lg border-hairline">
        <MenuItem onClick={() => window.dispatchEvent(new Event(OPEN_SHORTCUTS_EVENT))}>
          Keyboard shortcuts
        </MenuItem>
        <MenuSeparator />
        <MenuItem onClick={signOut}>
          <LogOut className="size-4" />
          Sign out
        </MenuItem>
      </MenuContent>
    </Menu>
  );
};
