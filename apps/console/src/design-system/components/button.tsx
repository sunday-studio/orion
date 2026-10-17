import { type VariantProps, cva } from "class-variance-authority";
import type * as React from "react";
import { controlBase } from "@/design-system/styles";
import { cn } from "@/utils/cn";

export const buttonStyles = cva(
  `${controlBase} inline-flex shrink-0 cursor-pointer items-center justify-center gap-2 whitespace-nowrap rounded-lg font-medium transition-[filter,background-color] [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0`,
  {
    variants: {
      variant: {
        primary:
          "border border-(--button-border) bg-gradient-to-b from-(--button-from) to-(--button-to) text-white shadow-[inset_0_1px_0_rgba(255,255,255,0.18),0_1px_2px_rgba(0,0,0,0.12)] hover:brightness-95",
        secondary: "border border-input-border bg-card text-ink shadow-card hover:bg-sub",
        destructive:
          "border border-destructive bg-destructive text-white shadow-card hover:brightness-95",
        ghost: "text-ink hover:bg-sub",
        link: "text-brand underline-offset-4 hover:underline",
      },
      size: {
        sm: "h-7 px-2.5 text-xs",
        md: "h-9 px-3.5 text-[13px]",
        icon: "size-9",
        "icon-sm": "size-7",
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
