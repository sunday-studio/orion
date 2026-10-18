import { type VariantProps, cva } from "class-variance-authority";
import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const buttonStyles = cva(
  `${controlBase} inline-flex shrink-0 cursor-pointer items-center justify-center whitespace-nowrap font-medium transition-[filter,background-color,scale] duration-200 ease-out active:scale-[0.98] [&_svg]:pointer-events-none [&_svg]:size-3.5 [&_svg]:shrink-0`,
  {
    variants: {
      variant: {
        primary:
          "border border-(--button-border) bg-gradient-to-b from-(--button-from) to-(--button-to) text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.25),inset_0_-1px_0_rgba(0,0,0,0.18),0_1px_2px_rgba(20,87,58,0.4),0_4px_10px_-3px_rgba(31,122,80,0.35)] hover:brightness-105",
        secondary:
          "border border-input-border bg-card text-ink shadow-[0_1px_2px_rgba(24,24,27,0.08)] hover:bg-sub",
        destructive:
          "border border-destructive bg-destructive text-white shadow-card hover:brightness-95",
        ghost: "text-ink hover:bg-sub",
        link: "text-brand underline-offset-4 hover:underline",
      },
      size: {
        // Canvas dense buttons: 30px default (12.5px text), 24px small (12px text), 14px icons.
        sm: "h-6 gap-1.5 rounded-[5px] px-2 text-xs",
        md: "h-[30px] gap-1.5 rounded-md px-[11px] text-[12.5px]",
        icon: "size-[30px] rounded-md",
        "icon-sm": "size-6 rounded-[5px]",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  },
);

export type ButtonProps = React.ComponentProps<"button"> & VariantProps<typeof buttonStyles>;

export const Button = ({ className, variant, size, type = "button", ...props }: ButtonProps) => (
  <button
    data-slot="ds-button"
    type={type}
    className={cn(buttonStyles({ variant, size }), className)}
    {...props}
  />
);
