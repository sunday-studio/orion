import type { ReactNode } from "react";
import type { NavIconName } from "./nav-model";

/*
 * Sidebar icons from the canvas: two-tone with round ends. The outline uses currentColor and the
 * soft fill uses --nav-fill, so an active item can tint both. Search is a plain line icon.
 */

type IconProps = { size?: number; className?: string };

const Svg = ({ size = 17, className, children }: IconProps & { children: ReactNode }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.6"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
    className={className}
  >
    {children}
  </svg>
);

const FILL = "var(--nav-fill)";

export const NavIcon = ({ name, ...props }: IconProps & { name: NavIconName }) => {
  switch (name) {
    case "overview":
      return (
        <Svg {...props}>
          <rect x="3.5" y="3.5" width="7.5" height="7.5" rx="2.5" fill={FILL} />
          <rect x="13" y="3.5" width="7.5" height="7.5" rx="2.5" />
          <rect x="3.5" y="13" width="7.5" height="7.5" rx="2.5" />
          <rect x="13" y="13" width="7.5" height="7.5" rx="2.5" fill={FILL} />
        </Svg>
      );
    case "incidents":
      return (
        <Svg {...props}>
          <path
            d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3z"
            fill={FILL}
          />
          <path d="M12 9v4" />
          <path d="M12 17h.01" />
        </Svg>
      );
    case "monitoring":
      return (
        <Svg {...props}>
          <rect x="3" y="4" width="18" height="13" rx="3.5" fill={FILL} />
          <path d="M6.5 11h2.2l1.6-3.2 3 6.4 1.6-3.2h2.6" />
          <path d="M9 20.5h6" />
        </Svg>
      );
    case "logs":
      return (
        <Svg {...props}>
          <rect x="4.5" y="3" width="15" height="18" rx="3.5" fill={FILL} />
          <path d="M8.5 8.5h7" />
          <path d="M8.5 12.5h7" />
          <path d="M8.5 16.5h4" />
        </Svg>
      );
    case "notify":
      return (
        <Svg {...props}>
          <path
            d="M3.26 15.33A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.67C19.41 13.96 18 12.5 18 8A6 6 0 0 0 6 8c0 4.5-1.41 5.96-2.74 7.33z"
            fill={FILL}
          />
          <path d="M10.27 21a2 2 0 0 0 3.46 0" />
        </Svg>
      );
    case "settings":
      return (
        <Svg {...props}>
          <path d="M4 7h8" />
          <path d="M18 7h2" />
          <path d="M4 17h2" />
          <path d="M12 17h8" />
          <circle cx="15" cy="7" r="2.6" fill={FILL} />
          <circle cx="9" cy="17" r="2.6" fill={FILL} />
        </Svg>
      );
  }
};

export const SearchIcon = (props: IconProps) => (
  <Svg {...props}>
    <circle cx="11" cy="11" r="6.5" />
    <path d="m20 20-4-4" />
  </Svg>
);

/** Panel icon; `direction` adds the arrow that shows what the button will do. */
export const PanelIcon = ({ direction, ...props }: IconProps & { direction?: "open" }) => (
  <Svg {...props}>
    <rect x="3.5" y="4.5" width="17" height="15" rx="3.5" />
    <path d="M9.5 4.5v15" />
    {direction === "open" && <path d="m13.5 9.5 2.5 2.5-2.5 2.5" />}
  </Svg>
);

export const ChevronIcon = ({ size = 13, className }: IconProps) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
    className={className}
  >
    <path d="m9 6 6 6-6 6" />
  </svg>
);

/** Logo mark: a dashed orbit around a point. */
export const LogoMark = () => (
  <span
    aria-hidden="true"
    className="flex size-[22px] items-center justify-center rounded-[7px] bg-[#1f7a50] text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.25),0_1px_2px_rgba(0,0,0,0.2)]"
  >
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="3" />
      <circle cx="12" cy="12" r="9" strokeDasharray="2 4" />
    </svg>
  </span>
);
