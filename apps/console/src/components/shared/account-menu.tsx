import { Menu, MenuContent, MenuItem, MenuSeparator, MenuTrigger } from "@/design-system";
import { LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { clearToken } from "@/api/client";
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
        <span
          aria-hidden
          className="block size-6 rounded-full"
          style={{ background: "conic-gradient(from 40deg, #6366f1, #14b8a6, #f59e0b, #6366f1)" }}
        />
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
