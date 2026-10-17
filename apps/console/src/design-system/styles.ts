/** Shared class fragments so every control focuses and disables the same way. */
export const focusRing =
  "outline-none focus-visible:ring-[3px] focus-visible:ring-focus focus-visible:border-brand";

export const disabledStyle = "disabled:pointer-events-none disabled:opacity-50";

export const controlBase = `${focusRing} ${disabledStyle}`;
