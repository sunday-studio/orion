import { Link } from "react-router-dom";
import { getToken } from "@/api/client";
import { AccountMenu } from "@/components/shared/account-menu";
import { cn } from "@/utils/cn";
import type { HealthChip } from "./use-health-chip";

const DOT: Record<HealthChip["tone"], string> = {
  issues: "bg-dot-warn",
  ok: "bg-dot-up",
  idle: "bg-faint",
  error: "bg-dot-down",
};

const pillClass = (tone: HealthChip["tone"]) =>
  cn(
    "mb-[3px] flex h-[26px] items-center gap-2 rounded-lg px-[9px] text-[12.5px] font-medium",
    tone === "issues"
      ? "bg-[var(--tone-degraded-bg)] text-[var(--tone-degraded-fg)]"
      : tone === "error"
        ? "bg-[var(--tone-down-bg)] text-[var(--tone-down-fg)]"
        : "bg-sub text-mid shadow-[inset_0_0_0_1px_var(--color-hairline)]",
  );

/** Issue count and account menu, pinned to the bottom of the open sidebar. */
export const SidebarFooter = ({ chip }: { chip: HealthChip }) => {
  const body = (
    <>
      <i aria-hidden="true" className={cn("block size-[7px] rounded-full", DOT[chip.tone])} />
      {chip.text}
    </>
  );
  return (
    <div className="mt-auto flex flex-col">
      {chip.tone === "issues" ? (
        <Link to="/health-issues" className={cn(pillClass(chip.tone), "no-underline")}>
          {body}
        </Link>
      ) : (
        <div role="status" className={pillClass(chip.tone)}>
          {body}
        </div>
      )}
      {getToken() && (
        <div className="mt-[5px] border-t border-hairline pt-2">
          <AccountMenu variant="row" />
        </div>
      )}
    </div>
  );
};

/** The same two things as icons, for the collapsed rail. */
export const CollapsedFooter = ({ chip }: { chip: HealthChip }) => (
  <div className="mt-auto flex flex-col items-center gap-1.5">
    <Link
      to="/health-issues"
      aria-label={chip.text}
      title={chip.text}
      className="flex h-[30px] w-[34px] items-center justify-center rounded-lg outline-none focus-visible:ring-[3px] focus-visible:ring-focus"
    >
      <i
        aria-hidden="true"
        className={cn(
          "block size-[9px] rounded-full",
          DOT[chip.tone],
          chip.tone === "issues" && "shadow-[0_0_0_4px_var(--tone-degraded-bg)]",
        )}
      />
    </Link>
    {getToken() && <AccountMenu />}
  </div>
);
